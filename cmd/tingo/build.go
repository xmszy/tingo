package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xmszy/tingo/net/thttp"
	"github.com/xmszy/tingo/os/tenv"
)

// toolLogDir 是 tingo 工具链日志目录（与框架运行时日志 runtime/log 一致）。
const toolLogDir = "runtime/log"

// openToolLog 以追加模式打开工具链日志文件 name（位于 runtime/log 下），
// 并确保目录存在。打开失败返回 nil（日志为可选能力，不影响主流程）。
func openToolLog(name string) *os.File {
	if err := os.MkdirAll(toolLogDir, 0o755); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(toolLogDir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}

// 热重载快照（首次调用建立基准）。
var (
	mu      sync.Mutex
	lastMod time.Time
)

// buildCmd 编译当前项目为可执行文件。
//
//	tingo build [--output dir] [--name bin] [--ldflags ...]
//	         [--platform linux/amd64] [--docker]
func buildCmd(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`用法: tingo build [选项]

编译当前项目为可执行文件。

选项:
  --output <dir>     输出目录（默认当前目录，不单独创建 bin/）
  --name <name>      可执行文件名（默认取模块名或目录名）
  --ldflags <args>   传给 go build -ldflags 的参数
  --platform <os/arch>  交叉编译目标，如 linux/amd64
  --docker           生成 Dockerfile（配合 --platform 使用）
  -h, --help         打印本帮助
`)
	}
	outDir := fs.String("output", ".", "输出目录（默认当前目录）")
	name := fs.String("name", "", "可执行文件名（默认取模块名或目录名）")
	ldflags := fs.String("ldflags", "", "传给 go build -ldflags 的参数")
	platform := fs.String("platform", "", "交叉编译目标，格式 GOOS/GOARCH，如 linux/amd64")
	docker := fs.Bool("docker", false, "生成 Dockerfile（配合 --platform 使用）")
	fs.Parse(args)

	mod, _ := modulePathOf(".")
	binName := *name
	if binName == "" {
		binName = modBase(mod)
	}
	binName = binaryName(binName)

	out := filepath.Join(*outDir, binName)
	cmdArgs := []string{"build", "-o", out}
	if *ldflags != "" {
		cmdArgs = append(cmdArgs, "-ldflags", *ldflags)
	}
	cmdArgs = append(cmdArgs, ".")

	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if *platform != "" {
		parts := strings.SplitN(*platform, "/", 2)
		if len(parts) == 2 {
			cmd.Env = append(cmd.Env, "GOOS="+parts[0], "GOARCH="+parts[1])
			out = out + "-" + parts[0] + "-" + parts[1]
			cmdArgs[2] = out // 更新 -o 目标
		} else {
			fmt.Fprintf(os.Stderr, "非法 --platform（应为 GOOS/GOARCH）: %s\n", *platform)
			os.Exit(2)
		}
	}
	fmt.Printf("building %s ...\n", out)

	// 构建日志同时落盘到 runtime/log/tingo-build.log，便于事后追溯
	// 构建报错、链接失败、溢出（stack overflow / OOM）等问题。
	logFile := openToolLog("tingo-build.log")
	if logFile != nil {
		defer logFile.Close()
		cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
		cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
		fmt.Fprintf(logFile, "\n[%s] tingo build %s\n", time.Now().Format("2006-01-02 15:04:05"), out)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "[%s] build failed: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(logFile, "[%s] built: %s\n", time.Now().Format("2006-01-02 15:04:05"), out)
	fmt.Printf("built: %s\n", out)

	if *docker {
		genDockerfile(*platform)
	}
}

// genDockerfile 生成最小化多阶段 Dockerfile（零外部依赖，基于 alpine/distroless）。
func genDockerfile(platform string) {
	base := "alpine:3.20"
	goos, _ := splitPlatform(platform)
	if goos == "windows" {
		base = "mcr.microsoft.com/windows/nanoserver:ltsc2022"
	}
	df := fmt.Sprintf(`# syntax=docker/dockerfile:1
# 由 tingo build --docker 生成。可自由修改。
FROM %s AS base
WORKDIR /app
COPY bin/ /app/bin/
EXPOSE 8080
ENTRYPOINT ["/app/bin/%s"]
`, base, binaryName(modBase(firstModule())))
	if err := os.WriteFile("Dockerfile", []byte(df), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "生成 Dockerfile 失败: %v\n", err)
		return
	}
	fmt.Println("generated Dockerfile")
}

func splitPlatform(p string) (goos, goarch string) {
	parts := strings.SplitN(p, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return p, ""
}

// firstModule 返回当前模块路径（供 Dockerfile 命名）。
func firstModule() string {
	mod, _ := modulePathOf(".")
	return mod
}

// runCmd 编译并运行当前项目。带 --watch 时进入热重载模式（文件变更自动重建）。
//
//	tingo run [--addr :8080] [--output dir] [--watch]
//	tingo serve ...（别名）
func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`用法: tingo run [选项]
       tingo serve [选项]   （serve 为 run 别名）

编译并运行当前项目。

选项:
  --addr <addr>      监听地址（默认 :8080）
  --output <dir>     临时构建目录（默认系统临时目录）
  --watch            文件变更自动重建并重载（开发模式）
  -h, --help         打印本帮助
`)
	}
	addr := fs.String("addr", ":8080", "监听地址")
	outDir := fs.String("output", filepath.Join(os.TempDir(), "tingo-run"), "临时构建目录（默认系统临时目录）")
	watch := fs.Bool("watch", false, "文件变更自动重建并重载（开发模式，零外部依赖）")
	fs.Parse(args)

	// 与框架一致：先自动加载 .env（及 .env.local），使 SERVER_ADDR 等
	// 环境变量在构建/运行流程中可见（缺失则忽略）。
	_ = tenv.Load(".env", ".env.local")

	if *watch {
		watchRun(*addr, *outDir)
		return
	}

	mod, _ := modulePathOf(".")
	binName := binaryName(modBase(mod))
	out := filepath.Join(*outDir, binName)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		os.Exit(1)
	}

	// 运行日志落盘到 runtime/log/tingo-run.log，便于事后追溯运行时问题
	// （panic、stack overflow、OOM、端口占用等构建期无法暴露的异常）。
	logFile := openToolLog("tingo-run.log")
	if logFile != nil {
		defer logFile.Close()
		fmt.Fprintf(logFile, "\n[%s] tingo run -> %s\n", time.Now().Format("2006-01-02 15:04:05"), out)
	}

	build := exec.Command("go", "build", "-o", out, ".")
	if logFile != nil {
		build.Stdout = io.MultiWriter(os.Stdout, logFile)
		build.Stderr = io.MultiWriter(os.Stderr, logFile)
	} else {
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
	}
	fmt.Println("building ...")
	if err := build.Run(); err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "[%s] build failed: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		os.Exit(1)
	}

	// 监听地址优先级与框架一致：SERVER_ADDR > TINGO_ADDR > --addr 默认值（:8080）。
	// 纯数字端口（如 8081）自动补 ":"。保证这里打印的端口与引擎实际监听端口一致。
	listen := *addr
	if v := tenv.Get("SERVER_ADDR", ""); v != "" {
		listen = v
	} else if v := tenv.Get("TINGO_ADDR", ""); v != "" {
		listen = v
	}
	listen = thttp.ResolveAddr(listen)
	run := exec.Command(out)
	// 启动横幅由框架 printStartup 统一打印（含 localhost/LAN 地址与耗时），
	// 与 go run 输出的 [TINGO] ... 格式一致，无需在此重复打印。
	run.Env = append(os.Environ(), "TINGO_ADDR="+listen)
	if logFile != nil {
		run.Stdout = io.MultiWriter(os.Stdout, logFile)
		run.Stderr = io.MultiWriter(os.Stderr, logFile)
	} else {
		run.Stdout = os.Stdout
		run.Stderr = os.Stderr
	}
	run.Stdin = os.Stdin
	if err := run.Run(); err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "[%s] exit: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}

// watchRun 热重载主循环：基于 mtime 轮询（不引入 fsnotify），零外部依赖。
func watchRun(addr, outDir string) {
	mod, _ := modulePathOf(".")
	binName := binaryName(modBase(mod))
	out := filepath.Join(outDir, binName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		os.Exit(1)
	}

	// 与 runCmd 一致的地址解析：SERVER_ADDR > TINGO_ADDR > --addr，纯数字补 ":"。
	listen := addr
	if v := tenv.Get("SERVER_ADDR", ""); v != "" {
		listen = v
	} else if v := tenv.Get("TINGO_ADDR", ""); v != "" {
		listen = v
	}
	listen = thttp.ResolveAddr(listen)
	var proc *exec.Cmd

	// 运行日志落盘到 runtime/log/tingo-run.log，整个 watch 会话共用，
	// 便于事后追溯运行时问题（panic、stack overflow、OOM 等）。
	logFile := openToolLog("tingo-run.log")
	if logFile != nil {
		defer logFile.Close()
		fmt.Fprintf(logFile, "\n[%s] tingo run --watch -> %s\n", time.Now().Format("2006-01-02 15:04:05"), out)
	}

	start := func() bool {
		if proc != nil {
			_ = proc.Process.Kill()
			_, _ = proc.Process.Wait()
		}
		fmt.Println("building ...")
		build := exec.Command("go", "build", "-o", out, ".")
		if logFile != nil {
			build.Stdout = io.MultiWriter(os.Stdout, logFile)
			build.Stderr = io.MultiWriter(os.Stderr, logFile)
		} else {
			build.Stdout = os.Stdout
			build.Stderr = os.Stderr
		}
		if err := build.Run(); err != nil {
			if logFile != nil {
				fmt.Fprintf(logFile, "[%s] build failed: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
			}
			fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
			return false
		}
		proc = exec.Command(out)
		proc.Env = append(os.Environ(), "TINGO_ADDR="+listen)
		if logFile != nil {
			proc.Stdout = io.MultiWriter(os.Stdout, logFile)
			proc.Stderr = io.MultiWriter(os.Stderr, logFile)
		} else {
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr
		}
		if err := proc.Start(); err != nil {
			if logFile != nil {
				fmt.Fprintf(logFile, "[%s] start failed: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
			}
			fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
			return false
		}
		return true
	}

	if !start() {
		fmt.Fprintln(os.Stderr, "初次构建失败，监听文件变更以重试")
	}

	// 信号退出。
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(700 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-sig:
			if proc != nil {
				_ = proc.Process.Kill()
			}
			fmt.Println("\nbye")
			return
		case <-ticker.C:
			if changed(".") {
				fmt.Println("检测到文件变更，重建中…")
				start()
			}
		}
	}
}

// changed 返回自上次快照以来项目内 .go 源文件是否发生变更（排除 _test.go 与 vendor）。
// 首次调用建立快照。
func changed(root string) bool {
	now := latestModTime(root)
	mu.Lock()
	defer mu.Unlock()
	if lastMod.Equal((time.Time{})) {
		lastMod = now
		return false
	}
	if now.After(lastMod) {
		lastMod = now
		return true
	}
	return false
}

// latestModTime 递归查找 root 下最新 .go 文件（非测试/非 vendor）的修改时间。
func latestModTime(root string) time.Time {
	var newest time.Time
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			n := d.Name()
			if n == "vendor" || n == "bin" || strings.HasPrefix(n, ".") && n != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return nil
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}

// testCmd 运行当前项目测试，透传其余参数给 go test。
//
//	tingo test [./...] [-race] [-bench .]
func testCmd(args []string) {
	ta := args
	if len(ta) == 0 {
		ta = []string{"./..."}
	}
	cmd := exec.Command("go", append([]string{"test"}, ta...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "测试失败: %v\n", err)
		os.Exit(1)
	}
}

// cleanCmd 清理构建产物。
//
//	tingo clean
func cleanCmd(args []string) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`用法: tingo clean

清理构建产物（根目录下的可执行文件与 *.test、runtime/log 日志）。
选项:
  -h, --help   打印本帮助
`)
	}
	fs.Parse(args)

	// 删除根目录下的可执行文件与测试二进制（tingo build 默认输出到当前目录）。
	mod, _ := modulePathOf(".")
	binName := binaryName(modBase(mod))
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		switch {
		case n == binName,
			strings.HasSuffix(n, ".test"),
			strings.HasSuffix(n, ".exe"):
			_ = os.Remove(n)
			fmt.Printf("removed %s\n", n)
		}
	}
	// 清理 tingo 与框架的运行时日志（runtime/log）。
	if _, err := os.Stat(toolLogDir); err == nil {
		if err := os.RemoveAll(toolLogDir); err != nil {
			fmt.Fprintf(os.Stderr, "清理 %s 失败: %v\n", toolLogDir, err)
			os.Exit(1)
		}
		fmt.Printf("removed %s\n", toolLogDir)
	}
	fmt.Println("clean done")
}

// modBase 取模块路径最后一段作为默认二进制名。
func modBase(mod string) string {
	if mod == "" {
		if wd, err := os.Getwd(); err == nil {
			return filepath.Base(wd)
		}
		return "app"
	}
	return filepath.Base(mod)
}

// binaryName 按操作系统补齐可执行文件后缀。
func binaryName(base string) string {
	if strings.HasSuffix(base, ".exe") {
		return base
	}
	if goos() == "windows" {
		return base + ".exe"
	}
	return base
}

// goos 返回当前目标操作系统（尊重 GOOS 环境变量）。
func goos() string {
	if v := os.Getenv("GOOS"); v != "" {
		return v
	}
	return runtime.GOOS
}

// ── route:list ──

// routeListCmd 编译当前项目并以 TINGO_LIST_ROUTES=1 环境变量运行，
// 此时引擎在启动后打印路由表并退出，不会真正监听端口。
//
//	tingo route:list [--method GET]
func routeListCmd(args []string) {
	fs := flag.NewFlagSet("route:list", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Print(`用法: tingo route:list [选项]

编译当前项目并列出所有已注册路由（打印后退出，不监听端口）。

选项:
  --method <METHOD>  按请求方法筛选（如 GET、POST）
  -h, --help         打印本帮助
`)
	}
	method := fs.String("method", "", "按请求方法筛选（如 GET、POST）")
	fs.Parse(args)

	tmpDir, err := os.MkdirTemp("", "tingo-routelist-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建临时目录失败: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	mod, _ := modulePathOf(".")
	binName := binaryName(modBase(mod))
	out := filepath.Join(tmpDir, binName)

	build := exec.Command("go", "build", "-o", out, ".")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		os.Exit(1)
	}

	env := append(os.Environ(), "TINGO_LIST_ROUTES=1")
	if *method != "" {
		env = append(env, "TINGO_LIST_ROUTES_METHOD="+*method)
	}

	run := exec.Command(out)
	run.Env = env
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	run.Run()
}
