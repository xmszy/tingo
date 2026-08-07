package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/xmszy/tingo/core"
)

// HeaderRequestID 是传递请求 ID 的默认头名。
const HeaderRequestID = "X-Request-ID"

// RequestIDConfig 是请求 ID 中间件的配置。
type RequestIDConfig struct {
	// Header 是读写请求 ID 的头名。
	Header string
	// Generator 自定义 ID 生成函数。
	Generator func() string
	// TrustUpstream 为 true 时复用上游传入的 ID，便于全链路追踪。
	TrustUpstream bool
}

// RequestID 返回请求 ID 中间件。
//
// 它为每个请求分配唯一 ID，写入上下文与响应头，
// 使日志、错误、追踪可以串联。
func RequestID(opts ...func(*RequestIDConfig)) core.Handler {
	cfg := RequestIDConfig{Header: HeaderRequestID, TrustUpstream: true}
	for _, o := range opts {
		o(&cfg)
	}
	gen := cfg.Generator
	if gen == nil {
		gen = newRequestID
	}
	header := cfg.Header
	trust := cfg.TrustUpstream

	return func(c *core.Ctx) {
		var id string
		if trust {
			id = c.Header(header)
		}
		if id == "" {
			id = gen()
		}
		c.SetRequestID(id)
		c.SetHeader(header, id)
		c.Next()
	}
}

// idPool 复用 ID 生成的字节缓冲，避免每请求分配。
var idPool = sync.Pool{
	New: func() any {
		b := make([]byte, 16)
		return &b
	},
}

// newRequestID 生成 32 位十六进制随机 ID。
func newRequestID() string {
	bp := idPool.Get().(*[]byte)
	defer idPool.Put(bp)
	b := *bp
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败属于系统级异常，此处降级不应影响请求。
		return ""
	}
	return hex.EncodeToString(b)
}
