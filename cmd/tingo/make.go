package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// makeCmd 生成代码骨架。
//
//	tingo make controller  <Name> [--app index] [--api|--plain] [--force]
//	tingo make model       <Name> [--app index] [--force]
//	tingo make middleware  <Name> [--app index] [--force]
//	tingo make validate    <Name> [--app index] [--force]
//	tingo make service     <Name> [--app index] [--force]
//	tingo make command     <Name> [--app index] [--force]
//	tingo make event       <Name> [--app index] [--force]
//	tingo make listener    <Name> [--app index] [--force]
//	tingo make subscribe   <Name> [--app index] [--force]
func makeCmd(args []string) {
	// 帮助优先：-h / --help / help 任意位置都打印用法。
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			// 有具体类型则打印该类型专属帮助，否则打印总览。
			kind := ""
			if len(args) >= 1 && args[0] != "-h" && args[0] != "--help" && args[0] != "help" {
				kind = args[0]
			}
			if kind == "" {
				printMakeUsage()
			} else {
				printMakeKindUsage(kind)
			}
			os.Exit(0)
		}
	}

	if len(args) < 2 {
		printMakeUsage()
		os.Exit(2)
	}
	kind := args[0]

	// 解析 @app 内联语法（对标 TP：controller index@User 表示 index 应用下的 User 控制器）。
	// @ 优先于 --app 标志。
	rawName := args[1]
	appInline := ""
	if at := strings.Index(rawName, "@"); at >= 0 {
		appInline = rawName[:at]
		rawName = rawName[at+1:]
	}
	name := sanitizeModule(rawName)
	if name == "" || strings.HasPrefix(rawName, "-") {
		fmt.Fprintf(os.Stderr, "无效的名称 %q。用法: tingo make %s <Name> [--app ...] [--force]\n", rawName, kind)
		os.Exit(2)
	}

	fs := flag.NewFlagSet("make", flag.ExitOnError)
	fs.Usage = func() { printMakeUsage() }
	app := fs.String("app", "", "目标应用；留空使用单应用 app/<layer>")
	api := fs.Bool("api", false, "生成 API 控制器（无 create/edit 方法）")
	plain := fs.Bool("plain", false, "生成空控制器（仅结构体声明）")
	force := fs.Bool("force", false, "覆盖已存在的文件")
	stub := fs.String("stub", "", "自定义模板文件路径（不指定则自动查找 stubs/ 或使用内置模板）")
	fs.Parse(args[2:])
	if appInline != "" {
		*app = appInline
	}

	// 多应用：生成子应用骨架并维护聚合导入。
	if kind == "app" {
		makeApp(name, *force)
		return
	}

	module, err := modulePathOf(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "未能读取模块路径: %v\n", err)
		os.Exit(1)
	}

	// 控制器变体
	variant := ""
	if kind == "controller" {
		if *plain {
			variant = "plain"
		} else if *api {
			variant = "api"
		}
	}

	// 1. 先验证模板文件存在（--stub 指定时）
	if *stub != "" {
		if _, err := os.Stat(*stub); err != nil {
			fmt.Fprintf(os.Stderr, "模板文件不存在: %s\n", *stub)
			os.Exit(2)
		}
	}

	// 2. 再检查目标文件
	targetRoot := "app"
	if *app != "" {
		targetRoot = filepath.Join(targetRoot, *app)
	}
	target := filepath.Join(targetRoot, kindDir(kind), name+".go")
	if _, err := os.Stat(target); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "已存在: %s（使用 --force 覆盖）\n", target)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		os.Exit(1)
	}

	// 3. 渲染模板
	content := renderMake(kind, name, *app, module, variant, *stub)
	if content == "" {
		if *stub != "" {
			// 不应到此，stub 文件已在前面验证过
			fmt.Fprintf(os.Stderr, "模板解析失败: %s\n", *stub)
		} else {
			fmt.Fprintf(os.Stderr, "未知类型: %s，支持: %s\n", kind, strings.Join(allMakeKinds(), ", "))
		}
		os.Exit(2)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("created  %s\n", target)
}

// printMakeUsage 打印 make 总览用法（无具体类型时）。
func printMakeUsage() {
	fmt.Print(`用法: tingo make <类型> <名称> [选项]

类型（查看某类型详情请用：tingo make <类型> -h）：
  app           多应用：生成子应用骨架（app/<名称>/）并维护聚合导入
  controller    资源控制器（7 个 REST 方法）
  model         数据模型（泛型 Model[T]）
  middleware    中间件
  validate      校验规则
  service       业务服务
  command       控制台指令
  event         事件载荷
  listener      事件监听器
  subscribe     事件订阅者

多应用目标（二选一）：
  <名称> 内联 @ 应用名    tingo make controller index@User
  --app <应用名>          tingo make controller User --app index

通用选项（各类型支持范围见其专属帮助）：
  --app   目标应用名（多应用时必填）
  --api   控制器仅生成 API 方法（无 create/edit）
  --plain 控制器仅生成空结构体
  --force 覆盖已存在的文件
  --stub  <file>  自定义模板路径（不指定则自动查找 stubs/<类型>.tpl 或用内置模板）

示例:
  tingo make controller User
  tingo make model Order --app api
  tingo make event UserRegistered
`)
}

// makeKindMeta 描述某类型的专属帮助信息。
type makeKindMeta struct {
	title   string // 类型中文名/简述
	target  string // 生成到哪
	opts    string // 该类型支持的选项说明（空表示仅通用选项）
	example string // 典型示例
}

// makeKindMetas 各类型专属帮助数据。
func makeKindMetas() map[string]makeKindMeta {
	return map[string]makeKindMeta{
		"app": {
			title:   "多应用：生成子应用骨架并维护聚合导入",
			target:  "app/<名称>/{controller,model,...} 以及聚合入口",
			opts:    "  --force  覆盖已存在的文件\n  --stub  <file>  自定义模板路径",
			example: "  tingo make app api",
		},
		"controller": {
			title:   "资源控制器（REST 风格 7 个方法：Index/Create/Store/Show/Edit/Update/Destroy）",
			target:  "app/controller/<Name>.go（多应用：app/<app>/controller/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --api    仅生成 API 方法（无 Create/Edit 视图）\n  --plain  仅生成空结构体（无方法）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make controller User\n  tingo make controller User --api\n  tingo make controller Post --app blog",
		},
		"model": {
			title:   "数据模型（泛型 Model[T]，含字段、表名、时间戳）",
			target:  "app/model/<Name>.go（多应用：app/<app>/model/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make model Order\n  tingo make model User --app api",
		},
		"middleware": {
			title:   "HTTP 中间件（func(http.Handler) http.Handler）",
			target:  "app/middleware/<Name>.go（多应用：app/<app>/middleware/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make middleware Auth",
		},
		"validate": {
			title:   "校验规则（结构体验证器，配合 validate 包使用）",
			target:  "app/validate/<Name>.go（多应用：app/<app>/validate/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make validate UserStore",
		},
		"service": {
			title:   "业务服务（无状态服务结构体，承载业务逻辑）",
			target:  "app/service/<Name>.go（多应用：app/<app>/service/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make service Order",
		},
		"command": {
			title:   "控制台指令（实现 cmd.Command 接口，可被 tingo run 调用）",
			target:  "app/command/<Name>.go（多应用：app/<app>/command/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make command SendEmails",
		},
		"event": {
			title:   "事件载荷（纯数据结构，承载事件传递的数据）",
			target:  "app/event/<Name>.go（多应用：app/<app>/event/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make event UserRegistered",
		},
		"listener": {
			title:   "事件监听器（实现 Handle(ctx, event) 的监听者）",
			target:  "app/listener/<Name>.go（多应用：app/<app>/listener/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make listener SendWelcomeEmail",
		},
		"subscribe": {
			title:   "事件订阅者（将监听器绑定到事件的订阅入口）",
			target:  "app/subscribe/<Name>.go（多应用：app/<app>/subscribe/<Name>.go）",
			opts:    "  --app   目标应用名（多应用时必填）\n  --force  覆盖已存在的文件\n  --stub   <file>  自定义模板路径",
			example: "  tingo make subscribe UserEvents",
		},
	}
}

// printMakeKindUsage 打印某类型的专属帮助。
func printMakeKindUsage(kind string) {
	metas := makeKindMetas()
	m, ok := metas[kind]
	if !ok {
		fmt.Fprintf(os.Stderr, "未知类型: %s，支持: %s\n", kind, strings.Join(allMakeKinds(), ", "))
		os.Exit(2)
	}
	fmt.Printf("用法: tingo make %s <名称> [选项]\n\n", kind)
	fmt.Printf("说明: %s\n", m.title)
	fmt.Printf("生成: %s\n\n", m.target)
	fmt.Println("该类型支持的选项:")
	fmt.Println(m.opts)
	fmt.Println("\n示例:")
	fmt.Println(m.example)
	fmt.Println("\n更多类型请见：tingo make -h")
}

// allMakeKinds 返回所有支持的生成类型。
func allMakeKinds() []string {
	return []string{"app", "controller", "model", "middleware", "validate", "service", "command", "event", "listener", "subscribe"}
}

// kindDir 返回脚手架子目录名。
func kindDir(kind string) string {
	switch kind {
	case "controller":
		return "controller"
	case "model":
		return "model"
	case "middleware":
		return "middleware"
	case "validate":
		return "validate"
	case "service":
		return "service"
	case "command":
		return "command"
	case "event":
		return "event"
	case "listener":
		return "listener"
	case "subscribe":
		return "subscribe"
	}
	return kind + "s"
}

// renderMake 渲染指定类型的骨架代码（已 gofmt）。
// variant: "" 默认，controller 可为 "api" 或 "plain"。
// stubPath: 用户指定的模板文件路径（空表示自动查找）。
func renderMake(kind, name, app, module, variant, stubPath string) string {
	var tmplStr string
	if stubPath != "" {
		data, err := os.ReadFile(stubPath)
		if err != nil {
			return ""
		}
		tmplStr = string(data)
	} else {
		// 自动查找：项目 stubs/ → 嵌入式内置模板
		tmplStr = loadStub(kind, variant)
		if tmplStr == "" {
			tmplStr = loadStub(kind, "")
		}
	}
	if tmplStr == "" {
		return ""
	}
	t, err := template.New(kind).Funcs(stubFuncs).Parse(tmplStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "模板解析失败 (%s): %v\n", kind, err)
		return ""
	}

	var buf bytes.Buffer
	_ = t.Execute(&buf, stubData(name, app, module))
	return gofmtSafe(buf.Bytes())
}

// stubData 构建模板变量。
func stubData(name, app, module string) map[string]interface{} {
	return map[string]interface{}{
		"Name":      pascalIdentifier(name),
		"App":       app,
		"Module":    module,
		"Table":     snakeIdentifier(name),
		"ShortName": lowerCamelIdentifier(name),
		"Command":   kebabIdentifier(name),
	}
}

// makeApp 生成一个新的子应用骨架（app/<name>/），并维护聚合导入文件。
//
// 对标 TP 多应用目录，但 Go 是编译型语言，无法运行时扫描目录自动注册，
// 因此每个子应用包需在 app/applications.go 中匿名导入以触发其 init() 注册。
func makeApp(name string, force bool) {
	module, err := modulePathOf(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "未能读取模块路径: %v\n", err)
		os.Exit(1)
	}
	if name == "" || name == "app" {
		fmt.Fprintf(os.Stderr, "无效的应用名 %q（不能与聚合包 app 重名）\n", name)
		os.Exit(2)
	}

	data := map[string]string{
		"Module":     module,
		"Name":       name,
		"AppName":    name,
		"LeftDelim":  "{{",
		"RightDelim": "}}",
	}

	files := map[string]string{
		"app/" + name + "/app.go":               renderMust(tplMultiAppApp, data),
		"app/" + name + "/controller/index.go":  renderMust(tplMultiAppController, data),
	}
	for rel, content := range files {
		target := filepath.Join(".", rel)
		if _, err := os.Stat(target); err == nil && !force {
			fmt.Fprintf(os.Stderr, "已存在: %s（使用 --force 覆盖）\n", target)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("created  %s\n", target)
	}

	ensureAppImport(module, name)
	fmt.Printf("\n提示：新应用 %q 已生成。默认路由前缀 /%s/（可在 config/app.toml [app] 段通过 app_map/domain_bind 调整）。\n", name, name)
}

// renderMust 渲染模板（Go 文件会 gofmt 化），失败即退出。
func renderMust(tpl string, data map[string]string) string {
	b, err := render(tpl, data, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "模板渲染失败: %v\n", err)
		os.Exit(1)
	}
	return string(b)
}

// ensureAppImport 在 app/applications.go 中维护子应用的匿名导入。
// 文件不存在则创建；存在则追加尚未导入的应用包（gofmt 格式化）。
func ensureAppImport(module, name string) {
	path := filepath.Join(".", "app", "applications.go")
	impLine := fmt.Sprintf("\t_ %q\n", module+"/app/"+name)
	content, err := os.ReadFile(path)

	if os.IsNotExist(err) {
		agg := fmt.Sprintf("// Package app 聚合并注册所有子应用。\npackage app\n\nimport (\n%s)\n", impLine)
		if formatted, ferr := format.Source([]byte(agg)); ferr == nil {
			agg = string(formatted)
		}
		if werr := os.WriteFile(path, []byte(agg), 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "写入聚合文件失败: %v\n", werr)
			os.Exit(1)
		}
		fmt.Printf("created  %s\n", path)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取聚合文件失败: %v\n", err)
		os.Exit(1)
	}

	cur := string(content)
	if strings.Contains(cur, module+"/app/"+name) {
		return // 已导入
	}
	if i := strings.Index(cur, "import ("); i >= 0 {
		insertAt := i + len("import (")
		newCur := cur[:insertAt] + "\n" + impLine + cur[insertAt:]
		if formatted, ferr := format.Source([]byte(newCur)); ferr == nil {
			newCur = string(formatted)
		}
		if werr := os.WriteFile(path, []byte(newCur), 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "更新聚合文件失败: %v\n", werr)
			os.Exit(1)
		}
		fmt.Printf("updated  %s\n", path)
	} else {
		fmt.Fprintf(os.Stderr, "聚合文件格式异常，未能自动追加导入；请手动 import %s\n", module+"/app/"+name)
	}
}

// gofmtSafe 格式化 Go 源码，失败则原样返回。
func gofmtSafe(b []byte) string {
	if out, err := format.Source(b); err == nil {
		return string(out)
	}
	return string(b)
}


