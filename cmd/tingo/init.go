package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/xmszy/tingo"
)

// faviconBytes 是最小透明 16×16 PNG（67 字节），用作占位 favicon。
var faviconBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF,
	0x61, 0x00, 0x00, 0x00, 0x01, 0x73, 0x52, 0x47, 0x42, 0x00, 0xAE, 0xCE, 0x1C, 0xE9, 0x00, 0x00,
	0x00, 0x1D, 0x49, 0x44, 0x41, 0x54, 0x18, 0x57, 0x63, 0x60, 0xF8, 0xCF, 0xC0, 0x00, 0x02, 0x8C,
	0x50, 0x14, 0x0B, 0x84, 0x06, 0x14, 0x73, 0x18, 0x31, 0x74, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// initCmd 根据项目名称创建项目脚手架。
//
//	tingo init <name>
//
// name 同时作为目录名和 Go 模块名。
// 生成的目录结构（对齐 all/tp 骨架）：
//
//	<name>/
//	  go.mod
//	  main.go
//	  .env.example
//	  .gitignore
//	  app/
//	    app.go
//	    kernel.go
//	    exception.go
//	    provider.go
//	    common.go
//	    config/
//	    route/
//	    controller/
//	    model/
//	    service/
//	    middleware/
//	    validate/
//	    view/
//	  config/
//	    app.toml
//	    cache.toml
//	    console.toml
//	    cookie.toml
//	    database.toml
//	    filesystem.toml
//	    lang.toml
//	    log.toml
//	    middleware.toml
//	    route.toml
//	    session.toml
//	    trace.toml
//	    view.toml
//	  public/
//	    favicon.ico
//	    index.html
//	    robots.txt
//	    .htaccess
//	    static/
//	  route/                    # 顶层路由（TP route/ 目录对等）
//	  view/                     # 顶层视图（TP view/ 目录对等）
//	  extend/                   # 扩展类库（TP extend/ 目录对等）
//	  runtime/
func initCmd(args []string) {
	// -h / --help 显示帮助。
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			usageInit()
			return
		}
	}

	// 项目名称必填。
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "用法: tingo init <name>\n请指定项目名称。\n")
		os.Exit(2)
	}
	name := args[0]
	if strings.HasPrefix(name, "-") {
		fmt.Fprintf(os.Stderr, "无效的项目名称 %q。用法: tingo init <name>\n", name)
		os.Exit(2)
	}

	// 多应用模式开关。
	multiApp := false
	for _, a := range args[1:] {
		switch a {
		case "--multi-app", "-a":
			multiApp = true
		case "-h", "--help", "help":
			usageInit()
			return
		}
	}

	// 以项目名创建子目录。
	root := name
	mod := sanitizeModule(name)

	// 检查目标目录是否非空。
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		fmt.Fprintf(os.Stderr, "目录 %q 非空，放弃初始化以避免覆盖。\n", root)
		os.Exit(1)
	}

	modulePath := mod

	data := map[string]string{
		"Module":     modulePath,
		"Name":       mod,
		"Version":    "v" + tingo.Version,

		"LeftDelim":  "{{",
		"RightDelim": "}}",
	}

	files := map[string]string{
		"go.mod":                  tplGomod,
		"main.go":                 tplMain,
		".env.example":            tplEnvExample,
		".gitignore":              tplGitignore,
		"README.md":               tplReadme,
		"LICENSE":                 tplLicense,
		"public/favicon.ico":      "", // 下方 raw bytes 单独写入
		"public/index.html":       tplPublicIndexHTML,
		"public/robots.txt":       tplRobotsTxt,
		"public/.htaccess":        tplHTAccess,
		"config/cache.toml":       tplCacheConfig,
		"config/console.toml":     tplConsoleConfig,
		"config/cookie.toml":      tplCookieConfig,
		"config/database.toml":    tplDatabaseConfig,
		"config/filesystem.toml":  tplFilesystemConfig,
		"config/lang.toml":        tplLangConfig,
		"config/log.toml":         tplLogConfig,
		"config/middleware.toml":   tplMiddlewareConfig,
		"config/route.toml":       tplRouteConfig,
		"config/session.toml":     tplSessionConfig,
		"config/trace.toml":       tplTraceConfig,
		"config/view.toml":        tplViewConfig,
	}

	// 应用层：单应用（默认）与多应用两种形态二选一。
	if multiApp {
		files["config/app.toml"] = tplAppConfigMulti

		// 各子应用共享的装配文件（与单应用形态能力对齐）。
		// 放在与 app 平级的顶级包（controller/core/middleware/provider），
		// 子应用只 import 这些顶级包、不 import app 包，避免聚合器形成 import cycle。
		sharedFiles := map[string]string{
			"core/kernel.go":         tplMultiAppKernel,
			"core/exception.go":      tplMultiAppException,
			"provider/provider.go":   tplMultiAppProvider,
			"core/common.go":         tplMultiAppCommon,
			"controller/base.go":     tplMultiAppBaseController,
			"middleware/auth.go":     tplMultiAppMiddlewareAuth,
		}
		for rel, tpl := range sharedFiles {
			b, err := render(tpl, data, strings.HasSuffix(rel, ".go"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "模板渲染失败(%s): %v\n", rel, err)
				os.Exit(1)
			}
			files[rel] = string(b)
		}

		for _, appName := range []string{"index", "admin"} {
			dataApp := map[string]string{
				"Module":     modulePath,
				"Name":       mod,
				"AppName":    appName,
				"LeftDelim":  "{{",
				"RightDelim": "}}",
			}
			appBytes, err := render(tplMultiAppApp, dataApp, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "模板渲染失败(%s): %v\n", "app/"+appName+"/app.go", err)
				os.Exit(1)
			}
			files["app/"+appName+"/app.go"] = string(appBytes)
			ctrlBytes, err := render(tplMultiAppController, dataApp, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "模板渲染失败(%s): %v\n", "app/"+appName+"/controller/index.go", err)
				os.Exit(1)
			}
			files["app/"+appName+"/controller/index.go"] = string(ctrlBytes)
			cfgBytes, err := render(tplMultiAppAppConfig, dataApp, false)
			if err != nil {
				fmt.Fprintf(os.Stderr, "模板渲染失败(%s): %v\n", "app/"+appName+"/config/app.toml", err)
				os.Exit(1)
			}
			files["app/"+appName+"/config/app.toml"] = string(cfgBytes)
		}
		aggBytes, err := render(tplMultiAppAggregator, data, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "模板渲染失败(app/applications.go): %v\n", err)
			os.Exit(1)
		}
		files["app/applications.go"] = string(aggBytes)
	} else {
		files["config/app.toml"] = tplAppConfig
		files["app/app.go"] = tplApplication
		files["app/config/app.toml"] = tplApplicationConfig
		files["app/route/app.go"] = tplRoute
		files["app/controller/index.go"] = tplController
		files["app/.htaccess"] = tplAppHtaccess
		files["app/kernel.go"] = tplKernel
		files["app/exception.go"] = tplExceptionHandle
		files["app/provider.go"] = tplAppService
		files["app/common.go"] = tplCommon
		files["app/controller/base.go"] = tplBaseController
		files["app/middleware/auth.go"] = tplMiddlewareAuth
	}
	keepDirs := []string{
		"public/static", "route", "view", "extend", "runtime",
	}
	if !multiApp {
		keepDirs = append(keepDirs,
			"app/model", "app/service", "app/validate", "app/view",
		)
	}

	for rel, tpl := range files {
		var content []byte
		var err error
		if rel == "public/favicon.ico" {
			// favicon 是二进制文件，直接写入原始字节。
			content = faviconBytes
		} else {
			content, err = render(tpl, data, strings.HasSuffix(rel, ".go"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "模板渲染失败(%s): %v\n", rel, err)
				os.Exit(1)
			}
		}
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", rel, err)
			os.Exit(1)
		}
		fmt.Printf("  created  %s\n", rel)
	}
	for _, d := range keepDirs {
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
			os.Exit(1)
		}
		keep := filepath.Join(full, ".gitkeep")
		if err := os.WriteFile(keep, []byte{}, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", keep, err)
			os.Exit(1)
		}
		fmt.Printf("  created  %s/\n", d)
	}

	fmt.Printf("\n项目 %q 已初始化（module %s）。\n", mod, modulePath)
	fmt.Print(`
下一步：
  cd 到项目目录
  go mod tidy
  tingo run               # 启动服务（默认 :8080）

提示：
  tingo gen model                从默认数据库生成 app/model
  tingo make controller <name>   生成控制器
  tingo make app <name>          新增子应用（多应用模式）
  tingo init <name> --multi-app  生成 index/admin 多应用骨架
`)
}

// usageInit 显示 init 子命令的用法。
func usageInit() {
	fmt.Fprintln(os.Stderr, `用法: tingo init <name> [--multi-app]

在 <name> 子目录下创建项目脚手架。

参数:
  name  项目名称，同时作为目录名和 Go 模块名。

选项:
  --multi-app, -a  生成多应用骨架（app/index 与 app/admin 两个应用，
                   并由 app/applications.go 聚合导入触发注册）。
                   不指定则生成单应用（app 包）形态。

生成的配置:
  config/
    app.toml          应用与 HTTP 服务（多应用模式含 [app] 调度键）
    cache.toml        缓存（内存 / Redis）
    console.toml      控制台自定义指令
    cookie.toml       Cookie 设置
    database.toml     数据库（MySQL/PgSQL/SQLite/SQLServer）
    filesystem.toml   文件系统多磁盘
    lang.toml         多语言
    log.toml          日志与访问日志
    middleware.toml   中间件别名与优先级
    route.toml        路由行为
    session.toml      会话与存储驱动
    trace.toml        Debug 调试工具栏
    view.toml         模板引擎

示例:
  tingo init blog              → 创建 blog/ 目录并生成单应用项目
  tingo init blog --multi-app  → 生成 blog/ 多应用骨架（index/admin）`)
	os.Exit(0)
}

// sanitizeModule 把任意名称规整为合法模块片段：小写、非字母数字转连字符。
func sanitizeModule(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '/':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	return out
}

// modulePathOf 读取给定目录 go.mod 的 module 指令。
func modulePathOf(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("未找到 module 指令")
}

// render 用 text/template 渲染模板并 gofmt 化（goFile 为 true 时）。
func render(tpl string, data map[string]string, goFile bool) ([]byte, error) {
	t, err := template.New("f").Parse(tpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	// 仅对 Go 源文件做 gofmt 校验（失败不致命，保留原始）。
	if goFile {
		if formatted, ferr := format.Source(out); ferr == nil {
			out = formatted
		}
	}
	return out, nil
}
