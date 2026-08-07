package main

// 本文件集中存放 tingo init --multi-app 的多应用脚手架模板，
// 与 scaffold_templates.go 中的单应用/通用模板分离，便于独立维护。

// ── 多应用模式：config/app.toml（[app] 段增加多应用调度键） ──
const tplAppConfigMulti = `# 应用配置。
name = "{{.Name}}"
debug = "${APP_DEBUG:-true}"
default_app = "index"
auto_multi_app = true
# app_map  : url 段 -> 应用名（别名）。访问 /m/... 落到 admin 应用。
# app_map  = { "m" = "admin" }
# domain_bind : 域名 -> 应用名。访问 admin.example.com 落到 admin 应用。
# domain_bind = { "admin.example.com" = "admin" }
# deny_app : 禁止通过 url 直接访问的应用（仅作内部复用）。
# deny_app = ["common"]

default_timezone = "Asia/Shanghai"

# HTTP 服务。
[server]
addr = "${SERVER_ADDR:-:8080}"
mode = "${SERVER_MODE:-debug}"     # debug / release / test
print_routes = true
read_timeout = "60s"
read_header_timeout = "20s"
write_timeout = "60s"
idle_timeout = "120s"
shutdown_timeout = "10s"
max_header_bytes = 1048576
max_multipart_memory = 33554432
trusted_proxies = []
`

// ── 多应用模式：共享应用装配内核（core/kernel.go，被各子应用复用） ──
//
// 共享装配放在顶级的 core 包（与 app 平级），不放在 app 包，避免与
// app/applications.go（聚合器，package app，匿名导入各子应用）形成 import cycle：
// 子应用只 import 顶级 core/controller/middleware/provider，不 import app 包。
const tplMultiAppKernel = `// Package core 是各子应用共享的装配内核。
//
// 多应用模式下，每个子应用（app/index、app/admin …）都通过本内核
// 完成中间件登记、容器绑定、事件订阅与系统服务注册，保证行为一致。
package core

import (
	"{{.Module}}/provider"
	t "github.com/xmszy/tingo/frame"
)

// Kernel 是本项目的应用装配内核。
var Kernel = t.NewKernel().
	// ---- 全局中间件 ----
	// 异常捕获中间件由框架自动置于最外层，此处只需登记业务中间件。
	Use(
	// t.Log(),
	// t.CORS(),
	).

	// ---- 容器绑定 ----
	Provide(func(c *t.Container) error {
		// t.Bind(func(*t.Container) (*service.User, error) {
		//     return service.NewUser(), nil
		// })
		return nil
	}).

	// ---- 事件订阅 ----
	Subscribe(func() error {
		// t.On("user.registered", func(u User) { ... })
		return nil
	}).

	// ---- 系统服务 ----
	Register(
		&provider.AppService{},
	)
`

// ── 多应用模式：共享全局异常处理器（app/core/exception.go） ──
const tplMultiAppException = `// Package core 的全局异常处理器。
package core

import (
	t "github.com/xmszy/tingo/frame"
)

// ExceptionHandle 构造全局异常处理器。
//
// 它决定了两件事：
//   - 哪些异常需要写日志（IgnoreReport 清单之外的都会记录）；
//   - 异常以什么形式呈现（Ajax/JSON 请求输出 JSON，否则输出 HTML 错误页）。
func ExceptionHandle() *t.ExceptionHandle {
	h := t.NewExceptionHandle()

	// debug 开启时输出错误详情与堆栈，生产环境请通过配置关闭。
	h.Debug = t.Config().Bool("debug", false)

	// 异常写入框架日志。
	h.Reporter = t.NewLogReporter(nil)

	// 纯 API 项目可打开此开关，让 HTML 请求也返回 JSON。
	// h.RenderJSON = true

	return h
}
`

// ── 多应用模式：共享应用级系统服务（app/provider/provider.go） ──
const tplMultiAppProvider = `package provider

import (
	t "github.com/xmszy/tingo/frame"
)

// AppService 是应用级系统服务。
//
// Register 在装配期执行，用于向容器登记服务；
// Boot 在所有服务注册完毕后执行，用于依赖其他服务的初始化。
type AppService struct{}

// Register 注册服务到容器。
func (*AppService) Register(c *t.Container) error {
	return nil
}

// Boot 在全部服务注册完成后执行初始化。
func (*AppService) Boot(c *t.Container) error {
	return nil
}

// Priority 决定装配顺序，值小者先执行。
func (*AppService) Priority() int { return 0 }
`

// ── 多应用模式：共享项目助手（app/core/common.go） ──
const tplMultiAppCommon = `package core

// 本文件存放项目级助手函数。
//
// 框架已内置常用助手，可直接使用：
//
//	t.Abort(404, "不存在")   中断请求并抛出异常
//	t.Snake / t.Camel        命名风格转换
//	t.Validate(data, rules)  数据校验
//
// 在此追加项目自己的公共函数即可。
`

// ── 多应用模式：共享控制器基类（app/controller/base.go） ──
const tplMultiAppBaseController = `// Package controller 提供各子应用共享的控制器基类。
//
// 项目中所有控制器都内嵌 Base，便于统一追加鉴权、公共数据等能力。
package controller

import t "github.com/xmszy/tingo/frame"

// Base 是本项目的控制器基类。
//
// 它内嵌框架的 t.Controller，因而自带：
//   - Success / Error / Result / Redirect  统一响应
//   - Validate / Bind / BindValidate       参数绑定与校验
//
// 在此追加的方法，所有业务控制器都能直接使用。
type Base struct {
	t.Controller
}

// User 演示如何在基类中提供公共能力：
// 从上下文取出鉴权中间件写入的当前用户标识。
func (Base) User(c *t.Ctx) string {
	return t.Req(c).Header("X-User-Id")
}
`

// ── 多应用模式：共享鉴权中间件示例（app/middleware/auth.go） ──
const tplMultiAppMiddlewareAuth = `// Package middleware 存放项目中间件。
package middleware

import t "github.com/xmszy/tingo/frame"

// Auth 是一个鉴权中间件示例。
//
// 中间件在注册期即展开进路由树，运行时没有任何动态分发开销。
func Auth() t.Handler {
	return func(c *t.Ctx) {
		if t.Req(c).Header("X-User-Id") == "" {
			t.Abort(401, "请先登录")
		}
		c.Next()
	}
}
`

// ── 多应用模式：子应用 app.go（package = 应用名） ──
const tplMultiAppApp = `// Package {{.AppName}} 是 {{.AppName}} 应用。
//
// 本文件在 init() 中通过 t.App() 把应用注册到引擎。
// 路由前缀 / 域名 / 默认应用等行为由 config/app.toml 的 [app] 段统一调度。
package {{.AppName}}

import (
	"{{.Module}}/core"
	"{{.Module}}/app/{{.AppName}}/controller"
    t "github.com/xmszy/tingo/frame"
)

// App 是 {{.AppName}} 应用的核心结构。
type App struct {
	t.BaseApp
}

// Boot 在路由注册前执行应用装配：异常处理器、全局中间件、容器绑定、事件订阅与系统服务。
func (App) Boot() error {
	core.Kernel.SetException(core.ExceptionHandle())
	return core.Kernel.Boot(nil)
}

// Middlewares 返回应用级中间件，作用于本应用的全部路由。
func (App) Middlewares() []t.Handler { return core.Kernel.Middlewares() }

// Routes 注册本应用的路由。
func (App) Routes(r t.Router) {
	r.Controller("/", &controller.IndexController{})
}

func init() {
	t.App("{{.AppName}}", App{})
}
`

// ── 多应用模式：子应用默认配置（app/<name>/config/app.toml） ──
const tplMultiAppAppConfig = `# {{.AppName}} 应用配置。这里的值覆盖 config/app.toml 中的同名项。
prefix = "/"
default = true
`

// ── 多应用模式：子应用控制器 ──
const tplMultiAppController = `// Package controller 是 {{.AppName}} 应用的控制器集合。
package controller

import t "github.com/xmszy/tingo/frame"

// IndexController 是 {{.AppName}} 应用的首页控制器。
type IndexController struct {
	t.Controller
}

// Index 处理 GET /（index 应用）或 /admin/（admin 应用）等。
func (c *IndexController) Index(ctx *t.Ctx) {
	c.Success(ctx, "欢迎使用 tingo {{.AppName}} 应用")
}
`

// ── 多应用模式：聚合导入，触发各子应用 init() 注册 ──
//
// Go 是编译型语言，无法像 PHP 那样在运行时扫描目录自动注册应用；
// 这里通过匿名导入各子应用包，触发其 init() 中的 t.App() 注册。
// 新增应用后，请在此追加对应的匿名导入。
const tplMultiAppAggregator = `// Package app 聚合并注册所有子应用。
package app

import (
	_ "{{.Module}}/app/index"
	_ "{{.Module}}/app/admin"
)
`
