package thttp

import (
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"unicode"

	"github.com/xmszy/tingo/core"
)

// ctxType 与 errorType 用于注册期快速判定方法是否为路由动作。
//
// 注意 core.Ctx 本身是 *gin.Context 的类型别名，故 ctxType 取 core.Ctx 而非 *core.Ctx。
var (
	ctxType   = reflect.TypeOf((*core.Ctx)(nil))
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

// reservedActionNames 是框架约定的非路由方法（钩子/声明），即使签名像 action 也不注册。
var reservedActionNames = map[string]bool{
	"Initialize": true, // tapp.Initializer 钩子
	"Middleware": true, // tapp.MiddlewareDeclarer 声明
}

/* ------------------------------------------------------------------ */
/* RESTful 资源路由                                                     */
/* ------------------------------------------------------------------ */

// restRoute 描述一条 RESTful 约定路由。
type restRoute struct {
	method string
	path   string
	action string
}

// restRoutes 是 resource 约定的七个动作。
// 顺序重要：静态路径（create）必须先于参数路径（:id）注册，
// 否则 radix tree 会产生冲突。
var restRoutes = []restRoute{
	{http.MethodGet, "", "Index"},
	{http.MethodGet, "/create", "Create"},
	{http.MethodPost, "", "Save"},
	{http.MethodGet, "/:id", "Read"},
	{http.MethodGet, "/:id/edit", "Edit"},
	{http.MethodPut, "/:id", "Update"},
	{http.MethodDelete, "/:id", "Delete"},
}

// Resource 注册 RESTful 资源路由。
//
//	r.Resource("/users", &controller.User{})
//
// 会按约定注册（仅注册控制器上实际存在的方法）：
//
//	GET    /users            → Index
//	GET    /users/create     → Create
//	POST   /users            → Save
//	GET    /users/:id        → Read
//	GET    /users/:id/edit   → Edit
//	PUT    /users/:id        → Update
//	DELETE /users/:id        → Delete
func (r *router) Resource(prefix string, ctrl any) core.Router {
	v := reflect.ValueOf(ctrl)
	name := controllerName(v.Type())

	for _, rt := range restRoutes {
		m := v.MethodByName(rt.action)
		if !m.IsValid() {
			continue // 未实现的动作直接跳过，不注册
		}
		r.registerMethod(rt.method, joinPath(prefix, rt.path), m.Interface(), name, rt.action)
	}
	return r
}

/* ------------------------------------------------------------------ */
/* 约定式控制器路由                                                      */
/* ------------------------------------------------------------------ */

// Controller 按约定注册控制器的全部导出方法。
//
//	r.Controller("/user", &controller.User{})
//
// 方法名到路径的映射遵循 url_convert 规则（驼峰转下划线）：
//
//	Index()       → ANY  /user
//	List()        → ANY  /user/list
//	GetProfile()  → GET  /user/profile      （Get 前缀识别为方法）
//	PostSave()    → POST /user/save
//	UserInfo()    → ANY  /user/user_info
//
// 只有「签名形如 action」的方法才会被注册为路由：以 *core.Ctx 为首个参数，
// 返回值为空或 error。这一约束自动排除了 Initialize() 钩子、Middleware()
// 声明，以及内嵌基类的绑定/校验方法（如 Validate(data any, ...)），
// 避免把框架内部方法误暴露成路由。
//
// 全部解析在注册期完成，运行时无任何动态分发。
func (r *router) Controller(prefix string, ctrl any) core.Router {
	v := reflect.ValueOf(ctrl)
	t := v.Type()
	name := controllerName(t)

	for i := 0; i < t.NumMethod(); i++ {
		mt := t.Method(i)
		if !mt.IsExported() {
			continue
		}
		if reservedActionNames[mt.Name] {
			continue
		}
		if !isActionMethod(mt.Type) {
			continue
		}
		httpMethod, action := parseMethodName(mt.Name)
		p := joinPath(prefix, actionPath(action))
		r.registerMethod(httpMethod, p, v.Method(i).Interface(), name, mt.Name)
		// Index 作为索引动作默认映射到控制器根路径（如 /admin），
		// 这里额外注册一条 /{prefix}/index，让用户也能用完整的
		// “控制器/方法”写法直接访问（如 /admin/index）。
		if action == "Index" {
			r.registerMethod(httpMethod, joinPath(prefix, "/index"), v.Method(i).Interface(), name, mt.Name)
		}
	}
	return r
}

// isActionMethod 判断一个方法是否应作为路由动作注册。
//
// 路由动作的方法签名必须恰好是：接收者 + 唯一的 *core.Ctx 参数，
// 返回值只能为空或 error。这一约束自动排除了：
//   - Initialize() 钩子与 Middleware() 声明（由保留名名单跳过）；
//   - 内嵌基类的绑定/校验方法（如 Bind(c, obj any)、Validate(data any, ...)），
//     它们带有额外参数，不应暴露成路由。
func isActionMethod(mt reflect.Type) bool {
	if mt.NumIn() != 2 || mt.In(1) != ctxType {
		return false
	}
	switch mt.NumOut() {
	case 0:
		return true
	case 1:
		return mt.Out(0) == errorType
	default:
		return false
	}
}

// registerMethod 注册单个控制器方法。
func (r *router) registerMethod(httpMethod, p string, fn any, ctrlName, action string) {
	h := core.Adapt(fn)
	full := joinPath(r.basePath, p)
	meta := &core.RouteMeta{App: r.app, Controller: ctrlName, Action: action}

	if httpMethod == "ANY" {
		for _, m := range anyMethods {
			r.bindMeta(m, full, meta)
		}
		r.group.Any(p, core.GinOf(h))
	} else {
		r.bindMeta(httpMethod, full, meta)
		r.group.Handle(httpMethod, p, core.GinOf(h))
	}
	r.engine.record(httpMethod, full, r.app, ctrlName+"."+action)
}

/* ------------------------------------------------------------------ */
/* 命名约定                                                             */
/* ------------------------------------------------------------------ */

// methodPrefixes 是可从方法名前缀推断出的 HTTP 方法。
var methodPrefixes = []struct {
	prefix string
	method string
}{
	{"Get", http.MethodGet},
	{"Post", http.MethodPost},
	{"Put", http.MethodPut},
	{"Delete", http.MethodDelete},
	{"Patch", http.MethodPatch},
	{"Head", http.MethodHead},
	{"Options", http.MethodOptions},
}

// parseMethodName 从方法名解析出 HTTP 方法与动作名。
//
// 仅当前缀后紧跟大写字母时才识别为 HTTP 方法前缀，
// 避免把 Getter、Postpone 之类的名字误判。
func parseMethodName(name string) (httpMethod, action string) {
	for _, mp := range methodPrefixes {
		if len(name) > len(mp.prefix) && strings.HasPrefix(name, mp.prefix) {
			next := rune(name[len(mp.prefix)])
			if unicode.IsUpper(next) {
				return mp.method, name[len(mp.prefix):]
			}
		}
	}
	return "ANY", name
}

// actionPath 将动作名转为 URL 路径片段。
// Index 映射到空串（即控制器根路径）；Controller 方法还会额外为 Index
// 注册一条 /index 路径，详见 Controller 中的处理。其余动作按下划线规则转换。
func actionPath(action string) string {
	if action == "Index" {
		return ""
	}
	return "/" + snake(action)
}

// snake 将驼峰命名转为下划线小写（url_convert 规则）。
//
//	UserInfo   → user_info
//	HTTPServer → http_server
//	ID         → id
func snake(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)

	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			// 在以下位置插入下划线：
			//   1. 小写/数字 后接大写      userInfo → user_info
			//   2. 连续大写后接小写        HTTPServer → http_server
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if !unicode.IsUpper(prev) || nextIsLower {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// controllerName 从控制器类型推断其名称。
func controllerName(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return snake(t.Name())
}

// nameOf 尽力推断 handler 的可读名称，用于路由表展示。
func nameOf(h any) string {
	v := reflect.ValueOf(h)
	if v.Kind() != reflect.Func {
		return ""
	}
	full := runtime.FuncForPC(v.Pointer()).Name()
	if i := strings.LastIndexByte(full, '/'); i >= 0 {
		full = full[i+1:]
	}
	return strings.TrimSuffix(full, "-fm")
}
