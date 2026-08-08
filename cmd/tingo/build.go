package main

import (
	"flag"
	"fmt"
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
  --output <dir>     输出目录（默认 bin）
  --name <name>      可执行文件名（默认取模块名或目录名）
  --ldflags <args>   传给 go build -ldflags 的参数
  --platform <os/arch>  交叉编译目标，如 linux/amd64
  --docker           生成 Dockerfile（配合 --platform 使用）
  -h, --help         打印本帮助
`)
	}
	outDir := fs.String("output", "bin", "输出目录")
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
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		os.Exit(1)
	}
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
	build := exec.Command("go", "build", "-o", out, ".")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	fmt.Println("building ...")
	if err := build.Run(); err != nil {
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
	fmt.Printf("serving on http://localhost:%s (Ctrl+C 退出)\n", listenAddrURL(listen))
	run := exec.Command(out)
	run.Env = append(os.Environ(), "TINGO_ADDR="+listen)
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	run.Stdin = os.Stdin
	if err := run.Run(); err != nil {
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

	start := func() bool {
		if proc != nil {
			_ = proc.Process.Kill()
			_, _ = proc.Process.Wait()
		}
		fmt.Println("building ...")
		build := exec.Command("go", "build", "-o", out, ".")
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
			return false
		}
		proc = exec.Command(out)
		proc.Env = append(os.Environ(), "TINGO_ADDR="+listen)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		if err := proc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
			return false
		}
		fmt.Printf("serving on http://localhost:%s (Ctrl+C 退出，文件变更自动重载)\n", listenAddrURL(listen))
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

// listenAddrURL 把监听地址（如 :8080、0.0.0.0:8080、127.0.0.1:8080）转成
// 可点击的 localhost URL 片段（如 8080），便于在启动日志里拼接成
// http://localhost:8080。LAN 地址由框架自身另行打印。
func listenAddrURL(listen string) string {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return "8080"
	}
	if strings.HasPrefix(listen, ":") {
		return strings.TrimPrefix(listen, ":")
	}
	// 含 host:port 时只取端口部分，统一用 localhost 作为可点击入口。
	if idx := strings.LastIndex(listen, ":"); idx >= 0 {
		return listen[idx+1:]
	}
	return listen
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

清理构建产物（bin/、runtime/logs、根目录下的 *.exe 与 *.test 文件）。
选项:
  -h, --help   打印本帮助
`)
	}
	fs.Parse(args)

	targets := []string{"bin", "runtime/logs"}
	for _, t := range targets {
		if _, err := os.Stat(t); err == nil {
			if err := os.RemoveAll(t); err != nil {
				fmt.Fprintf(os.Stderr, "清理 %s 失败: %v\n", t, err)
				os.Exit(1)
			}
			fmt.Printf("removed %s\n", t)
		}
	}
	// 删除根目录下的可执行文件与测试二进制。
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".test") || strings.HasSuffix(n, ".exe") {
			_ = os.Remove(n)
			fmt.Printf("removed %s\n", n)
		}
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
