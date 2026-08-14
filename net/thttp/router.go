// Package thttp 提供 tingo 的 HTTP 服务层。
//
// 核心策略：所有多应用/多控制器的路由在注册期全部展开成
// 静态路由，运行时由 radix tree 直接命中，
// 不做任何字符串解析、反射查找或 map 查询，因此性能与原生等同。
package thttp

import (
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

// router 是 core.Router 的实现，包装 gin 的 RouterGroup。
type router struct {
	// group 是底层 gin 路由组。
	group gin.IRouter
	// app 是当前所属应用名，注册期常量。
	app string
	// engine 反向引用，用于记录路由表。
	engine *Engine
	// basePath 是当前组的路径前缀，用于生成路由表。
	basePath string
}

// 确保实现契约。
var _ core.Router = (*router)(nil)

/* ------------------------------------------------------------------ */
/* 基础方法                                                             */
/* ------------------------------------------------------------------ */

// Handle 注册任意 HTTP 方法的路由。
func (r *router) Handle(method, p string, h any) core.Router {
	handler := core.Adapt(h)
	full := joinPath(r.basePath, p)
	name := nameOf(h)

	// 元信息登记进查找表，不包装 handler，运行时零额外开销。
	r.bindMeta(method, full, &core.RouteMeta{App: r.app, Action: name})

	r.group.Handle(method, p, core.GinOf(handler))
	r.engine.record(method, full, r.app, name)
	return r
}

// GET 注册 GET 路由。
func (r *router) GET(p string, h any) core.Router { return r.Handle(http.MethodGet, p, h) }

// POST 注册 POST 路由。
func (r *router) POST(p string, h any) core.Router { return r.Handle(http.MethodPost, p, h) }

// PUT 注册 PUT 路由。
func (r *router) PUT(p string, h any) core.Router { return r.Handle(http.MethodPut, p, h) }

// DELETE 注册 DELETE 路由。
func (r *router) DELETE(p string, h any) core.Router { return r.Handle(http.MethodDelete, p, h) }

// PATCH 注册 PATCH 路由。
func (r *router) PATCH(p string, h any) core.Router { return r.Handle(http.MethodPatch, p, h) }

// HEAD 注册 HEAD 路由。
func (r *router) HEAD(p string, h any) core.Router { return r.Handle(http.MethodHead, p, h) }

// OPTIONS 注册 OPTIONS 路由。
func (r *router) OPTIONS(p string, h any) core.Router { return r.Handle(http.MethodOptions, p, h) }

// Any 注册全部常用方法的路由。
func (r *router) Any(p string, h any) core.Router {
	handler := core.Adapt(h)
	full := joinPath(r.basePath, p)
	name := nameOf(h)

	meta := &core.RouteMeta{App: r.app, Action: name}
	for _, m := range anyMethods {
		r.bindMeta(m, full, meta)
	}

	r.group.Any(p, core.GinOf(handler))
	r.engine.record("ANY", full, r.app, name)
	return r
}

// anyMethods 是 Any 所覆盖的 HTTP 方法，与 gin 的实现保持一致。
var anyMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodHead, http.MethodOptions, http.MethodDelete,
	http.MethodConnect, http.MethodTrace,
}

// Use 追加中间件到当前组。中间件在注册期即 flatten 进 gin 的 HandlersChain。
func (r *router) Use(mws ...core.Handler) core.Router {
	if len(mws) == 0 {
		return r
	}
	r.group.Use(core.GinChain(mws)...)
	return r
}

// PrioritizedMiddleware 是带优先级中间件（别名 core.WeightedMiddleware）。
//
// 数值越小越先执行（全局 -> 模块 -> 控制器 -> 动作 由小到大注册）。
// 同级按注册顺序。
type PrioritizedMiddleware = core.WeightedMiddleware

// UseOrdered 按优先级追加中间件到当前组。
//
// 在注册期对传入的中间件按 Priority 升序排序后，再 flatten 进 gin 的 HandlersChain，
// 使调用顺序与优先级一致，无需依赖调用次序。
//
// 用法：
//
//	r.UseOrdered(
//	    thttp.PrioritizedMiddleware{Recover(), 0},
//	    thttp.PrioritizedMiddleware{Auth(), 100},
//	    thttp.PrioritizedMiddleware{Log(), 50},
//	)
//	// 执行顺序：Recover(0) -> Log(50) -> Auth(100)
func (r *router) UseOrdered(mws ...core.WeightedMiddleware) core.Router {
	if len(mws) == 0 {
		return r
	}
	sorted := make([]core.WeightedMiddleware, len(mws))
	copy(sorted, mws)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	hs := make([]core.Handler, len(sorted))
	for i, mw := range sorted {
		hs[i] = mw.H
	}
	r.group.Use(core.GinChain(hs)...)
	return r
}

// Group 创建子路由组。
func (r *router) Group(prefix string, fn func(core.Router), mws ...core.Handler) core.Router {
	g := r.group.Group(prefix, core.GinChain(mws)...)
	sub := &router{
		group:    g,
		app:      r.app,
		engine:   r.engine,
		basePath: joinPath(r.basePath, prefix),
	}
	if fn != nil {
		fn(sub)
	}
	return r
}

// Module 创建按模块名隔离的子路由组。
//
// 自动套 /{module} 前缀并隔离中间件组：模块级中间件仅作用于该模块内路由，
// 不影响模块外；模块内再 Group 基于 /{module} 前缀展开。
//
// 例：Module("admin", func(r) { r.GET("/user", listUser) })
// 最终注册路由：GET /admin/user
func (r *router) Module(name string, fn func(core.Router), mws ...core.Handler) core.Router {
	prefix := "/" + strings.Trim(name, "/")
	return r.Group(prefix, fn, mws...)
}

// Static 注册静态文件目录。
func (r *router) Static(prefix, root string) core.Router {
	r.group.Static(prefix, root)
	r.engine.record("GET", joinPath(r.basePath, prefix)+"/*filepath", r.app, "static:"+root)
	return r
}

/* ------------------------------------------------------------------ */
/* 路由元信息绑定                                                        */
/* ------------------------------------------------------------------ */

// bindMeta 在注册期登记路由元信息。
//
// 关键优化：元信息进入只读查找表而非 handler 包装层，
// 因此开启元信息不增加任何运行时调用层级与内存分配。
func (r *router) bindMeta(method, fullPath string, m *core.RouteMeta) {
	if !r.engine.bindMeta || (m.App == "" && m.Controller == "" && m.Action == "") {
		return
	}
	r.engine.app.RegisterRouteMeta(method, fullPath, m)
}

/* ------------------------------------------------------------------ */
/* 路径工具                                                             */
/* ------------------------------------------------------------------ */

// joinPath 拼接两段路由路径，规范化斜杠。
func joinPath(base, p string) string {
	if p == "" {
		return ensureSlash(base)
	}
	if base == "" || base == "/" {
		return ensureSlash(p)
	}
	joined := path.Join(base, p)
	// path.Join 会吃掉结尾斜杠，若原路径有则补回（gin 对此敏感）。
	if strings.HasSuffix(p, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return ensureSlash(joined)
}

// ensureSlash 确保路径以 / 开头。
func ensureSlash(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		return "/" + p
	}
	return p
}
