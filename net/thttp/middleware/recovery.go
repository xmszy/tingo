// Package middleware 提供 tingo 内置中间件。
package middleware

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/xmszy/tingo/core"
	terrors "github.com/xmszy/tingo/errors"
)

// RecoveryConfig 是 Recovery 中间件的配置。
type RecoveryConfig struct {
	// StackSize 是捕获调用栈的缓冲区大小。
	StackSize int
	// PrintStack 决定是否将调用栈打印到 stderr。
	PrintStack bool
	// Handler 自定义 panic 处理，为空时使用默认的统一错误响应。
	Handler func(c *core.Ctx, err any, stack []byte)
}

// Recovery 返回 panic 恢复中间件。
//
// 它会：
//   - 捕获 panic 并转为统一的 500 错误响应；
//   - 识别客户端断连（broken pipe），此时不打日志也不写响应；
//   - 保留 *errors.Error 的原始状态码与业务码。
func Recovery(opts ...func(*RecoveryConfig)) core.Handler {
	cfg := RecoveryConfig{StackSize: 4 << 10, PrintStack: true}
	for _, o := range opts {
		o(&cfg)
	}

	return func(c *core.Ctx) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			// 客户端主动断开连接时，写响应已无意义且会再次报错。
			if isBrokenPipe(r) {
				c.Abort()
				return
			}

			stack := make([]byte, cfg.StackSize)
			stack = stack[:runtime.Stack(stack, false)]

			if cfg.PrintStack {
				fmt.Fprintf(os.Stderr, "\n[tingo] panic recovered: %v\n%s\n", r, stack)
			}

			if cfg.Handler != nil {
				cfg.Handler(c, r, stack)
				return
			}

			c.Abort()
			core.CurrentResponder().Fail(c, panicToError(r))
		}()

		c.Next()
	}
}

// panicToError 将 panic 值转为结构化错误，尽量保留原始语义。
func panicToError(r any) error {
	switch v := r.(type) {
	case *terrors.Error:
		return v
	case error:
		return terrors.ErrInternal.Wrap(v)
	case string:
		return terrors.ErrInternal.Wrap(errors.New(v))
	default:
		return terrors.ErrInternal.Wrap(fmt.Errorf("%v", v))
	}
}

// isBrokenPipe 判断 panic 是否源于客户端断连。
func isBrokenPipe(r any) bool {
	ne, ok := r.(*net.OpError)
	if !ok {
		return false
	}
	var se *os.SyscallError
	if !errors.As(ne.Err, &se) {
		return false
	}
	msg := strings.ToLower(se.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "an established connection was aborted") ||
		strings.Contains(msg, "an existing connection was forcibly closed")
}
