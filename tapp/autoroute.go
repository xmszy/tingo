package tapp

import (
	"sort"
	"sync"

	"github.com/xmszy/tingo/core"
)

/* ------------------------------------------------------------------ */
/* 自动路由注册表：免声明扫描                                             */
/* ------------------------------------------------------------------ */

// controllerEntry 是自动路由登记表中的一项，对应一个控制器的路由前缀与实例。
type controllerEntry struct {
	pattern string
	ctrl    any
}

// controllerRegistry 保存所有通过 RegisterController 登记的控制器。
//
// Go 没有「运行时列出某目录下所有类」的能力，
// 因此采用等价的约定式自注册：每个控制器在自己的 init() 里调用
// RegisterController 把自己登记到全局表，框架启动时由 AutoRoute 一次性
// 全部挂载。登记与挂载都在注册期完成，运行时无任何反射或动态分发。
var (
	controllerMu       sync.RWMutex
	controllerRegistry = make(map[string]controllerEntry)
)

// RegisterController 把一个控制器登记到自动路由表。
//
// 在控制器所在包的 init() 中调用一次即可，例如：
//
//	func init() {
//	    tapp.RegisterController("/", &controller.Index{})
//	    tapp.RegisterController("/user", &controller.User{})
//	}
//
// pattern 是该控制器挂载的路由前缀（与 r.Controller 的第一个参数相同）。
// 重复登记同一前缀会被覆盖（后者生效）。
func RegisterController(pattern string, ctrl any) {
	controllerMu.Lock()
	controllerRegistry[pattern] = controllerEntry{pattern: pattern, ctrl: ctrl}
	controllerMu.Unlock()
}

// AutoRoute 把自动路由表中登记的全部控制器按约定注册到路由器。
//
// 在应用的 Routes 里写一行即可：
//
//	func (*App) Routes(r core.Router) { tapp.AutoRoute(r) }
//
// 控制器方法到路径的映射遵循 url_convert 规则
// （驼峰转下划线）：GetUserInfo() -> GET /user/user_info，Index() -> /user。
func AutoRoute(r core.Router) {
	controllerMu.RLock()
	entries := make([]controllerEntry, 0, len(controllerRegistry))
	for _, e := range controllerRegistry {
		entries = append(entries, e)
	}
	controllerMu.RUnlock()

	// 按前缀排序，保证路由注册顺序稳定（长前缀优先，避免短前缀抢占）。
	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].pattern) > len(entries[j].pattern)
	})
	for _, e := range entries {
		r.Controller(e.pattern, e.ctrl)
	}
}
