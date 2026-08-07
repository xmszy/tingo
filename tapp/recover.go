package tapp

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
	"github.com/xmszy/tingo/os/ttrace"
)

/* ------------------------------------------------------------------ */
/* 异常捕获中间件                                                        */
/* ------------------------------------------------------------------ */

// PanicError 携带 panic 现场信息，便于异常处理器输出堆栈。
type PanicError struct {
	// Value 是 recover() 得到的原始值。
	Value any
	// Stack 是 panic 时的调用栈。
	Stack []byte
}

// Error 实现 error 接口。
func (p *PanicError) Error() string {
	return fmt.Sprintf("panic: %v\n%s", p.Value, p.Stack)
}

// Recover 返回把 panic 转交给异常处理器的中间件。
//
// 该中间件只在 panic 发生时才有额外成本，正常路径仅一次 defer。
func Recover(h *ExceptionHandle) core.Handler {
	if h == nil {
		h = NewExceptionHandle()
	}
	return func(c *core.Ctx) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			// 客户端主动断开连接不属于业务异常，不记录也不渲染。
			if isBrokenPipe(r) {
				c.Abort()
				return
			}
			// tapp.Abort() 抛出的是结构化错误，需保留其状态码与业务码；
			// 只有真正的意外 panic 才归类为 500 并附带堆栈。
			err := toError(r)
			// 记录到调试工具栏（Error 面板 / X-Tingo-Trace 头）。
			if pe, ok := err.(*PanicError); ok {
				ttrace.AddPanic(r, pe.Stack)
			} else {
				ttrace.LogError(err.Error())
			}
			h.Report(c, err)
			if !c.G().Writer.Written() {
				h.Render(c, err)
			}
			c.Abort()
		}()
		c.Next()
	}
}

// toError 把 recover 到的值转换为框架错误。
//
// 分三种情况：
//   - *errors.Error：由 Abort() 主动抛出，原样保留状态码与业务码；
//   - error：包装为 500 并保留错误链；
//   - 其他值：真正的意外 panic，包装为 500 并附带堆栈。
func toError(r any) error {
	switch v := r.(type) {
	case *errors.Error:
		return v
	case error:
		return errors.ErrInternal.Wrap(v)
	default:
		return errors.ErrInternal.Wrap(&PanicError{Value: r, Stack: debug.Stack()})
	}
}

// isBrokenPipe 判断 panic 是否由客户端断连引起。
func isBrokenPipe(r any) bool {
	ne, ok := r.(*net.OpError)
	if !ok {
		return false
	}
	var se *os.SyscallError
	if !errors.As(ne, &se) {
		return false
	}
	msg := strings.ToLower(se.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer")
}

/* ------------------------------------------------------------------ */
/* 404 / 405 兜底                                                       */
/* ------------------------------------------------------------------ */

// NoRoute 返回交给异常处理器渲染的 404 处理器。
func NoRoute(h *ExceptionHandle) core.Handler {
	if h == nil {
		h = NewExceptionHandle()
	}
	return func(c *core.Ctx) {
		h.Render(c, errors.ErrNotFound.WithMessagef("路由 %s %s 不存在", c.Method(), c.Path()))
	}
}

// NoMethod 返回交给异常处理器渲染的 405 处理器。
func NoMethod(h *ExceptionHandle) core.Handler {
	if h == nil {
		h = NewExceptionHandle()
	}
	return func(c *core.Ctx) {
		c.Status(http.StatusMethodNotAllowed)
		h.Render(c, errors.ErrMethodNotAllowed)
	}
}
