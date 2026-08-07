// Package recovery 提供 panic 恢复中间件。
//
// 设计：零外部依赖，捕获 handler 链路中的 panic，返回 500 并记录日志。
package recovery

import (
	"net/http"
	"runtime"

	"github.com/xmszy/tingo/core"
)

// Handler 是自定义的 panic 处理器，可返回自定义响应。
type Handler func(c *core.Ctx, err any, stack []byte)

// Middleware 返回恢复中间件。当发生 panic 时记录栈并返回 500。
func Middleware() core.Handler {
	return MiddlewareWith(nil)
}

// MiddlewareWith 允许传入自定义 panic 处理器。
func MiddlewareWith(h Handler) core.Handler {
	return func(c *core.Ctx) {
		defer func() {
			if err := recover(); err != nil {
				stack := make([]byte, 4096)
				n := runtime.Stack(stack, false)
				stack = stack[:n]
				if h != nil {
					h(c, err, stack)
					return
				}
				c.G().AbortWithStatusJSON(http.StatusInternalServerError, map[string]any{
					"code":    500,
					"message": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
