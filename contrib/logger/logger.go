// Package logger 提供 HTTP 访问日志中间件。
//
// 设计：零外部依赖，默认输出到标准错误（可由 SetOutput 重定向）。
// 记录方法、路径、状态码、耗时、客户端 IP、user-agent。
package logger

import (
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/xmszy/tingo/core"
)

var (
	mu     sync.Mutex
	output io.Writer = os.Stderr
	logger_          = log.New(os.Stderr, "", log.LstdFlags)
)

// SetOutput 设置日志输出目标（如文件）。
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	output = w
	logger_ = log.New(w, "", log.LstdFlags)
}

// Middleware 返回访问日志中间件。
func Middleware() core.Handler {
	return func(c *core.Ctx) {
		start := time.Now()
		path := c.Path()
		raw := c.RawQuery()
		c.Next()
		latency := time.Since(start)
		status := c.G().Writer.Status()
		mu.Lock()
		l := logger_
		mu.Unlock()
		l.Printf("http-access status=%d latency=%s ip=%s method=%s path=%s%s agent=%s",
			status, latency, c.IP(), c.Method(), path, querySuffix(raw), c.UserAgent())
	}
}

func querySuffix(raw string) string {
	if raw == "" {
		return ""
	}
	return "?" + raw
}

// 保留 net/http 引用，避免未使用告警（部分场景需要 StatusText）。
var _ = http.StatusOK
