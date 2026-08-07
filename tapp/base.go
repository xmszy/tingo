package tapp

import (
	"github.com/xmszy/tingo/core"
)

/* ------------------------------------------------------------------ */
/* 应用基类：多应用模式下的 app/<name>/                                  */
/* ------------------------------------------------------------------ */

// Base 是业务应用的可选基类，内嵌它即可获得默认实现，
// 只需覆写关心的部分。
//
//	type Admin struct {
//	    tapp.Base
//	}
//
//	func (a *Admin) Routes(r core.Router) {
//	    r.Resource("/users", &controller.User{})
//	}
//
//	func init() { core.RegisterApp("admin", &Admin{}) }
type Base struct {
	// Cfg 是应用配置，可在构造时指定前缀、域名等。
	Cfg core.AppConfig

	// Mws 是应用级中间件，作用于该应用的全部路由。
	Mws []core.Handler
}

// Config 实现 core.AppConfigurer。
func (b *Base) Config() core.AppConfig { return b.Cfg }

// Middlewares 实现 core.AppMiddlewarer。
func (b *Base) Middlewares() []core.Handler { return b.Mws }

// Boot 实现 core.AppBooter 的默认空实现。
func (b *Base) Boot() error { return nil }

// 确保 Base 满足全部可选接口，业务应用内嵌后无需重复实现。
var (
	_ core.AppConfigurer  = (*Base)(nil)
	_ core.AppMiddlewarer = (*Base)(nil)
	_ core.AppBooter      = (*Base)(nil)
)

/* ------------------------------------------------------------------ */
/* 引擎装配                                                              */
/* ------------------------------------------------------------------ */

// Attach 把内核装配到一个具备 Use 方法的 HTTP 引擎上。
//
// 传入引擎的 Use 方法即可，例如：
//
//	kernel.Attach(func(mws ...core.Handler) { engine.Use(mws...) })
//
// 以函数而非接口作为参数，是为了避免 core 反向依赖 net/thttp。
//
// 它完成两件事：
//  1. 在最外层安装异常捕获中间件（Recover）；
//  2. 依次安装全局中间件。
//
// 中间件顺序：异常捕获永远在最外层，
// 保证其后所有中间件与控制器抛出的异常都能被兜住。
func (k *Kernel) Attach(use func(...core.Handler)) *Kernel {
	if use == nil {
		return k
	}
	k.mu.Lock()
	exception := k.exception
	mws := append([]core.Handler(nil), k.middlewares...)
	k.mu.Unlock()

	chain := make([]core.Handler, 0, len(mws)+1)
	if exception != nil {
		chain = append(chain, Recover(exception))
	}
	chain = append(chain, mws...)
	use(chain...)
	return k
}
