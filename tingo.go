package tingo

import (
	"context"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/database/tdb"
	"github.com/xmszy/tingo/net/thttp"
	"github.com/xmszy/tingo/net/thttp/middleware"
	"github.com/xmszy/tingo/os/tcfg"
	"github.com/xmszy/tingo/os/tenv"
)

// Version 是当前框架版本。
const Version = "0.0.1"

// New 创建绑定默认 App 的 HTTP 引擎，并装配默认中间件链。
func New(opts ...thttp.Option) *thttp.Engine {
	return newEngine(core.DefaultApp(), false, opts...)
}

// NewBare 创建绑定默认 App、但不带中间件的引擎。
func NewBare(opts ...thttp.Option) *thttp.Engine {
	return newEngine(core.DefaultApp(), true, opts...)
}

func newEngine(app *core.App, bare bool, opts ...thttp.Option) *thttp.Engine {
	// 在加载配置前自动加载 .env（及 .env.local），使 config/*.toml 中的
	// ${APP_DEBUG} 等占位符可被展开；文件不存在时忽略，已存在的系统变量不会被覆盖。
	_ = tenv.Load(".env", ".env.local")

	if !app.HasService(tcfg.ServiceName) {
		if err := app.Register(tcfg.NewService(".")); err != nil {
			panic(err)
		}
	}
	if !app.HasService(tdb.ServiceName) {
		if err := app.Register(tdb.NewService("config/database.toml")); err != nil {
			panic(err)
		}
	}
	e := thttp.NewWithApp(app, opts...)
	if bare {
		return e
	}
	accessLogger := middleware.NewAccessLogger()
	e.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		accessLogger.Handler(),
	)
	e.ConfigureAtBoot(accessLogger.ConfigureFromTree)
	e.ConfigureAtBoot(e.ConfigureAdminFromTree)
	return e
}

// Framework 是显式持有生命周期、容器和 HTTP 引擎的框架实例。
type Framework struct {
	app    *core.App
	engine *thttp.Engine
}

// NewApp 创建完全隔离的框架实例。推荐新项目使用该入口。
func NewApp(opts ...thttp.Option) *Framework {
	app := core.NewApp()
	return &Framework{app: app, engine: newEngine(app, false, opts...)}
}

// Core 返回实例级生命周期内核。
func (f *Framework) Core() *core.App { return f.app }

// Engine 返回 HTTP 引擎逃生舱。
func (f *Framework) Engine() *thttp.Engine { return f.engine }

// Register 注册框架服务。
func (f *Framework) Register(services ...core.Service) error { return f.app.Register(services...) }

// App 注册业务应用并返回自身，便于链式装配。
func (f *Framework) App(name string, application core.Application) *Framework {
	f.app.RegisterApplication(name, application)
	return f
}

// Use 注册全局 HTTP 中间件。
func (f *Framework) Use(middlewares ...core.Handler) *Framework {
	f.engine.Use(middlewares...)
	return f
}

// Run 启动当前框架实例，并由系统信号驱动关闭。
func (f *Framework) Run() error { return f.engine.Run() }

// RunContext 启动当前框架实例，ctx 取消时优雅关闭。
func (f *Framework) RunContext(ctx context.Context) error { return f.engine.RunContext(ctx) }

// Shutdown 关闭当前框架实例及其全部服务。
func (f *Framework) Shutdown(ctx context.Context) error { return f.engine.ShutdownContext(ctx) }

// Run 以默认配置创建引擎、装配全部已注册应用并启动服务。
//
// 这是最常用的入口，等价于 New(opts...).Run()。
func Run(opts ...thttp.Option) error {
	return New(opts...).Run()
}

/* ------------------------------------------------------------------ */
/* 常用符号再导出，便于 main 包直接使用                                   */
/* ------------------------------------------------------------------ */

type (
	// Kernel 是实例级框架生命周期内核。
	Kernel = core.App
	// Service 是框架服务生命周期契约。
	Service = core.Service
	// Application 是业务应用契约。
	Application = core.Application
	// Ctx 是请求上下文。
	Ctx = core.Ctx
	// Router 是路由注册器。
	Router = core.Router
	// Handler 是请求处理器。
	Handler = core.Handler
	// Engine 是 HTTP 引擎。
	Engine = thttp.Engine
)

// 引擎配置项。
var (
	Addr        = thttp.Addr
	PrintRoutes = thttp.PrintRoutes
	TLS         = thttp.TLS
)

// RegisterApp 注册一个应用。
var RegisterApp = core.RegisterApp

// SetResponder 替换全局响应协议。
var SetResponder = core.SetResponder
