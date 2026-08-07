package tapp

import (
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/xmszy/tingo/core"
)

// RouteMeta 描述一条注解路由：HTTP 方法、路径、以及控制器中处理它的方法名。
//
// 示例（控制器在 init 中登记，运行时由 AnnotationRoute 读取并注册）：
//
//	type User struct{}
//
//	// tingo:route GET /user/list
//	func (c *User) List(ctx *core.Ctx) {}
//	// tingo:route POST /user/save
//	func (c *User) Save(ctx *core.Ctx) {}
//
//	func (c *User) Annotations() []tapp.RouteMeta {
//	    return []tapp.RouteMeta{
//	        {Method: "GET", Path: "/user/list", Handler: "List"},
//	        {Method: "POST", Path: "/user/save", Handler: "Save"},
//	    }
//	}
type RouteMeta struct {
	Method  string // GET/POST/PUT/DELETE/PATCH/ANY（不区分大小写）
	Path    string // 路由路径；若以 "/" 开头则视为绝对路径，否则拼接 prefix
	Handler string // 控制器中处理方法的方法名
}

// RouteAnnotated 是支持注解路由的控制器接口。
// 控制器实现 Annotations() 返回其路由声明，框架即可在启动时自动挂载。
type RouteAnnotated interface {
	Annotations() []RouteMeta
}

// AnnotationRoute 把实现了 RouteAnnotated 的控制器按注解注册到路由器。
//
// prefix 为挂载前缀（可空）；meta.Path 若以 "/" 开头，则忽略 prefix 直接作为绝对路径。
// 在应用的 Routes 里写一行即可：
//
//	func (*App) Routes(r core.Router) {
//	    tapp.AnnotationRoute(r, "/user", &controller.User{})
//	}
//
// 也可配合 RegisterController + AutoRouteAnnotated 做全局自注册（见 autoroute.go）。
func AnnotationRoute(r core.Router, prefix string, ctrl RouteAnnotated) {
	v := reflect.ValueOf(ctrl)
	for _, meta := range ctrl.Annotations() {
		method := strings.ToUpper(strings.TrimSpace(meta.Method))
		if method == "" {
			method = "ANY"
		}
		full := joinRoutePath(prefix, meta.Path)
		m := v.MethodByName(meta.Handler)
		if !m.IsValid() {
			continue
		}
		registerByMethod(r, method, full, m.Interface())
	}
}

// registerByMethod 按 HTTP 方法把 handler 注册到路由。
func registerByMethod(r core.Router, method, path string, handler any) {
	switch method {
	case "GET":
		r.GET(path, handler)
	case "POST":
		r.POST(path, handler)
	case "PUT":
		r.PUT(path, handler)
	case "DELETE":
		r.DELETE(path, handler)
	case "PATCH":
		r.PATCH(path, handler)
	case "HEAD":
		r.HEAD(path, handler)
	case "OPTIONS":
		r.OPTIONS(path, handler)
	default: // ANY / 未知方法一律 Any
		r.Any(path, handler)
	}
}

// joinRoutePath 拼接前缀与路径（保证单一 "/" 分隔）。
func joinRoutePath(prefix, p string) string {
	if strings.HasPrefix(p, "/") {
		return p // 绝对路径
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		if p == "" {
			return "/"
		}
		return "/" + p
	}
	if p == "" {
		return prefix
	}
	return prefix + "/" + p
}

/* ------------------------------------------------------------------ */
/* 注解路由自动注册表：与 RegisterController 对称                          */
/* ------------------------------------------------------------------ */

// annotatedEntry 是注解路由登记表中的一项。
type annotatedEntry struct {
	prefix string
	ctrl   RouteAnnotated
}

var (
	annotatedMu       sync.RWMutex
	annotatedRegistry = make(map[string]annotatedEntry)
)

// RegisterAnnotated 把一个注解路由控制器登记到全局表。
//
//	func init() { tapp.RegisterAnnotated("/user", &controller.User{}) }
func RegisterAnnotated(prefix string, ctrl RouteAnnotated) {
	annotatedMu.Lock()
	annotatedRegistry[prefix] = annotatedEntry{prefix: prefix, ctrl: ctrl}
	annotatedMu.Unlock()
}

// AutoRouteAnnotated 把登记的全部注解路由控制器按声明挂载到路由器。
//
//	func (*App) Routes(r core.Router) { tapp.AutoRouteAnnotated(r) }
func AutoRouteAnnotated(r core.Router) {
	annotatedMu.RLock()
	entries := make([]annotatedEntry, 0, len(annotatedRegistry))
	for _, e := range annotatedRegistry {
		entries = append(entries, e)
	}
	annotatedMu.RUnlock()
	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].prefix) > len(entries[j].prefix)
	})
	for _, e := range entries {
		AnnotationRoute(r, e.prefix, e.ctrl)
	}
}
