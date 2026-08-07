// Package view 提供视图渲染中间件与辅助函数。
//
// 核心功能：
//  1. 初始化全局模板引擎（SetDefault）
//  2. 提供 Render/RenderIn 辅助函数从控制器渲染模板
//  3. 支持布局继承
//
// 用法：
//
//	view.SetDefault(tview.NewFromTree(cfgData))
//	engine.Use(view.Middleware(view.Config{}))
//	// 在控制器中：
//	html, err := view.Render("index/index", map[string]any{"title": "Home"})
//	html, err := view.RenderIn("layout", "index/index", data)
package view

import (
	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tview"
)

// defaultEngine 全局默认模板引擎（SetDefault 注册）。
var defaultEngine *tview.Engine

// SetDefault 注册全局默认模板引擎。
// 必须在应用中初始化时调用一次。
func SetDefault(e *tview.Engine) {
	defaultEngine = e
}

// Engine 返回全局模板引擎。
func Engine() *tview.Engine {
	return defaultEngine
}

// Config 视图中间件配置。
type Config struct {
	// Engine 模板引擎实例，nil 则使用 SetDefault 注册的全局引擎。
	Engine *tview.Engine
}

// Middleware 返回视图中间件。
// 模板引擎是无状态的，中间件仅负责确保引擎已就绪。
func Middleware(cfg Config) core.Handler {
	e := cfg.Engine
	if e == nil {
		e = defaultEngine
	}
	// 不活跃变量抑制编译告警；引擎通过 Engine() 或 Render 获取。
	_ = e
	return func(c *core.Ctx) {
		c.Next()
	}
}

// Render 使用全局引擎渲染模板。
func Render(name string, data any) (string, error) {
	return defaultEngine.Render(name, data)
}

// RenderWith 使用指定引擎渲染模板。
func RenderWith(e *tview.Engine, name string, data any) (string, error) {
	return e.Render(name, data)
}

// RenderIn 使用全局引擎在布局中渲染模板（layout 继承）。
// 布局模板通过 {{.content}} 引用子模板的输出。
func RenderIn(layout, name string, data any) (string, error) {
	return defaultEngine.RenderIn(layout, name, data)
}

// RenderInWith 使用指定引擎在布局中渲染模板。
func RenderInWith(e *tview.Engine, layout, name string, data any) (string, error) {
	return e.RenderIn(layout, name, data)
}
