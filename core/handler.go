package core

import (
	"net/http"
	"sync"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/errors"
)

/* ------------------------------------------------------------------ */
/* Handler 类型                                                        */
/* ------------------------------------------------------------------ */

// Handler 是 tingo 的原生 handler，与 gin.HandlerFunc 内存布局一致。
// 由于 Ctx 与 gin.Context 布局相同，Handler 与 gin.HandlerFunc 之间
// 可以通过 unsafe-free 的函数值转换互转，无运行时开销。
type Handler func(*Ctx)

// HandlerE 是可返回错误的 handler，错误由统一的错误中间件处理。
type HandlerE func(*Ctx) error

// Middleware 是中间件类型，等同于 Handler。
type Middleware = Handler

// WeightedMiddleware 是带优先级的中间件。
//
// 数值越小越先执行（全局 -> 模块 -> 控制器 -> 动作 由小到大注册）。
// 同级按注册顺序。通过 Router.UseOrdered 注册。
type WeightedMiddleware struct {
	H        Handler
	Priority int
}

/* ------------------------------------------------------------------ */
/* 零成本转换                                                           */
/* ------------------------------------------------------------------ */

// ginOf 将 Handler 转为 gin.HandlerFunc，不引入任何包装层。
//
// 正确性依据：
//   - Ctx 由 `type Ctx gin.Context` 定义，二者底层类型相同，
//     内存布局、大小、对齐完全一致（见 TestCtxLayoutCompatible）；
//   - 因此 func(*Ctx) 与 func(*gin.Context) 的调用约定与函数值表示相同，
//     重新解释函数值是安全的，且语义等价于逐个转换参数。
//
// 相比 `func(c *gin.Context){ h((*Ctx)(c)) }` 这种闭包写法，
// 本实现省去一层函数调用与一次闭包分配，使转换真正零成本。
//
// 这是框架内唯一使用 unsafe 之处，其前提由 TestCtxLayoutCompatible
// 与 TestGinOfIdentity 在每次构建时校验。
func ginOf(h Handler) gin.HandlerFunc {
	return *(*gin.HandlerFunc)(unsafe.Pointer(&h))
}

// GinOf 导出的 Handler → gin.HandlerFunc 转换。零成本。
func GinOf(h Handler) gin.HandlerFunc { return ginOf(h) }

// GinChain 批量转换，注册期一次性完成。
func GinChain(hs []Handler) gin.HandlersChain {
	if len(hs) == 0 {
		return nil
	}
	out := make(gin.HandlersChain, len(hs))
	for i, h := range hs {
		out[i] = ginOf(h)
	}
	return out
}

// HandlerOf 将 gin.HandlerFunc 转为 Handler，用于复用 gin 生态中间件。零成本。
func HandlerOf(h gin.HandlerFunc) Handler {
	return *(*Handler)(unsafe.Pointer(&h))
}

/* ------------------------------------------------------------------ */
/* 统一响应协议                                                         */
/* ------------------------------------------------------------------ */

// Responder 决定 handler 返回值如何写入响应。
// 框架提供默认实现，业务可全局替换以定制响应格式。
type Responder interface {
	// Reply 在 handler 成功返回时调用。data 可能为 nil。
	Reply(c *Ctx, data any)
	// Fail 在 handler 返回错误时调用。
	Fail(c *Ctx, err error)
}

// defaultResponder 是框架默认的响应协议：
//
//	成功 { "code": "", "message": "ok", "data": ... }
//	失败 { "code": "XXX", "message": "...", "meta": {...} }
type defaultResponder struct{}

// Reply 输出成功响应。
func (defaultResponder) Reply(c *Ctx, data any) {
	if data == nil {
		c.G().JSON(http.StatusOK, okEmpty)
		return
	}
	// 复用 map 以避免每次成功响应都分配一个 gin.H（W/WN 装饰器的高频路径）。
	m := replyPool.Get().(gin.H)
	m["code"] = ""
	m["message"] = "ok"
	m["data"] = data
	c.G().JSON(http.StatusOK, m)
	clear(m) // 编码为同步写回 w，清空后即可归还复用
	replyPool.Put(m)
}

// Fail 输出失败响应。
func (defaultResponder) Fail(c *Ctx, err error) {
	e := errors.From(err)
	c.G().JSON(e.Status, e)
}

var okEmpty = gin.H{"code": "", "message": "ok"}

// replyPool 复用成功响应的 gin.H map，降低 W/WN 装饰器路径的分配。
var replyPool = sync.Pool{New: func() any { return gin.H{} }}

// responder 是当前生效的响应协议。
var responder Responder = defaultResponder{}

// SetResponder 替换全局响应协议。应在服务启动前调用。
func SetResponder(r Responder) {
	if r != nil {
		responder = r
	}
}

// CurrentResponder 返回当前响应协议。
func CurrentResponder() Responder { return responder }
