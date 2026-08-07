package core

/* ------------------------------------------------------------------ */
/* 请求级键值存储                                                        */
/* ------------------------------------------------------------------ */

// Set 写入请求级键值。
func (c *Ctx) Set(key string, val any) { c.G().Set(key, val) }

// Get 读取请求级键值。
func (c *Ctx) Get(key string) (any, bool) { return c.G().Get(key) }

// MustGet 读取请求级键值，不存在时 panic。
func (c *Ctx) MustGet(key string) any { return c.G().MustGet(key) }

// GetString 读取字符串型请求级键值。
func (c *Ctx) GetString(key string) string { return c.G().GetString(key) }

// GetInt 读取 int 型请求级键值。
func (c *Ctx) GetInt(key string) int { return c.G().GetInt(key) }

// GetInt64 读取 int64 型请求级键值。
func (c *Ctx) GetInt64(key string) int64 { return c.G().GetInt64(key) }

// GetBool 读取 bool 型请求级键值。
func (c *Ctx) GetBool(key string) bool { return c.G().GetBool(key) }

/* ------------------------------------------------------------------ */
/* 泛型键值：类型安全、无断言                                             */
/* ------------------------------------------------------------------ */

// CtxKey 是类型安全的请求级键。
//
// 用法：
//
//	var UserKey = core.NewCtxKey[*model.User]("auth.user")
//	UserKey.Set(c, u)
//	u, ok := UserKey.Get(c)
type CtxKey[T any] struct{ name string }

// NewCtxKey 创建一个类型安全的请求级键。
func NewCtxKey[T any](name string) CtxKey[T] { return CtxKey[T]{name: name} }

// Name 返回键名。
func (k CtxKey[T]) Name() string { return k.name }

// Set 写入值。
func (k CtxKey[T]) Set(c *Ctx, v T) { c.G().Set(k.name, v) }

// Get 读取值。第二个返回值表示是否存在且类型匹配。
func (k CtxKey[T]) Get(c *Ctx) (T, bool) {
	v, ok := c.G().Get(k.name)
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}

// Must 读取值，不存在时返回零值。
func (k CtxKey[T]) Must(c *Ctx) T {
	t, _ := k.Get(c)
	return t
}

/* ------------------------------------------------------------------ */
/* 框架内置的请求级状态                                                   */
/* ------------------------------------------------------------------ */

// 内置键名。使用固定前缀避免与业务键冲突。
const (
	keyRequestID    = "tingo.request_id"
	keyFrameworkApp = "tingo.framework_app"
)

// BindFrameworkApp 将请求绑定到处理它的框架 App。由 HTTP 适配器在管线首部调用。
func BindFrameworkApp(c *Ctx, app *App) { c.G().Set(keyFrameworkApp, app) }

// Framework 返回处理当前请求的框架 App；未显式绑定时兼容返回 DefaultApp。
func (c *Ctx) Framework() *App {
	if value, exists := c.G().Get(keyFrameworkApp); exists {
		if app, ok := value.(*App); ok && app != nil {
			return app
		}
	}
	return DefaultApp()
}

// RouteMeta 描述一条路由的静态归属信息。
//
// 该结构在「注册期」构造一次并被同一路由的所有请求共享，
// 它是只读的，可安全并发访问。
type RouteMeta struct {
	// App 是所属应用名。
	App string
	// Controller 是控制器名。
	Controller string
	// Action 是动作名。
	Action string
}

// emptyMeta 用于未绑定路由时返回，避免调用方判空。
var emptyMeta = &RouteMeta{}

func metaKey(method, fullPath string) string { return method + " " + fullPath }

// RegisterRouteMeta 在当前 App 的注册期登记路由元信息。
func (a *App) RegisterRouteMeta(method, fullPath string, meta *RouteMeta) {
	a.mu.Lock()
	a.routeMeta[metaKey(method, fullPath)] = meta
	a.mu.Unlock()
}

func (a *App) routeMetaOf(method, fullPath string) (*RouteMeta, bool) {
	a.mu.RLock()
	meta, ok := a.routeMeta[metaKey(method, fullPath)]
	a.mu.RUnlock()
	return meta, ok
}

// RegisterMeta 是向默认 App 登记路由元信息的兼容入口。
func RegisterMeta(method, fullPath string, meta *RouteMeta) {
	DefaultApp().RegisterRouteMeta(method, fullPath, meta)
}

// Route 返回当前请求的路由归属信息。
// 未启用元信息绑定或路由未登记时返回空的 RouteMeta，不会返回 nil。
//
// 本方法零分配：FullPath 由 gin 在路由匹配时已填好，
// 拼接键使用栈上小字符串，map 查找不产生逃逸。
func (c *Ctx) Route() *RouteMeta {
	g := c.G()
	fp := g.FullPath()
	if fp == "" {
		return emptyMeta
	}
	if meta, ok := c.Framework().routeMetaOf(g.Request.Method, fp); ok {
		return meta
	}
	return emptyMeta
}

// App 返回当前请求归属的应用名，如 admin、api。
func (c *Ctx) App() string { return c.Route().App }

// Controller 返回当前请求命中的控制器名。
func (c *Ctx) Controller() string { return c.Route().Controller }

// Action 返回当前请求命中的方法名。
func (c *Ctx) Action() string { return c.Route().Action }

// RequestID 返回请求追踪 ID。
func (c *Ctx) RequestID() string { return c.G().GetString(keyRequestID) }

// SetRequestID 设置请求追踪 ID。
func (c *Ctx) SetRequestID(id string) { c.G().Set(keyRequestID, id) }

// ResetMeta 清空默认 App 的路由元信息。仅供测试使用。
func ResetMeta() {
	app := DefaultApp()
	app.mu.Lock()
	app.routeMeta = make(map[string]*RouteMeta, 64)
	app.mu.Unlock()
}
