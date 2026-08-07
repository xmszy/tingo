// Package trace 提供轻量级请求链路追踪中间件（零外部依赖）。
//
// 提供请求可观测能力，用标准库实现，不引入 OpenTelemetry。
// 每个请求分配一个 TraceID（优先复用 X-Request-ID / traceparent，
// 便于与上游网关串联），写入响应头并在上下文中透传，供日志关联。
package trace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/xmszy/tingo/core"
)

const (
	headerTraceID  = "X-Trace-Id"
	headerRequest  = "X-Request-Id"
	ctxKeyTraceID  = "_tingo_trace_id"
)

// Config 配置。
type Config struct {
	// HeaderName 写回响应头的字段名，默认 X-Trace-Id。
	HeaderName string
	// Sample 采样概率 [0,1]；默认 1（全采）。设为 0 仅注入 ID 不记录慢日志。
	Sample float64
	// SlowLog 慢请求阈值；大于该值的请求打印一条告警日志。默认 500ms。
	SlowLog time.Duration
	// OnSlow 慢请求回调（可选）；默认打印到 stderr。
	OnSlow func(traceID, method, path string, dur time.Duration)
}

func (c *Config) norm() {
	if c.HeaderName == "" {
		c.HeaderName = headerTraceID
	}
	if c.Sample == 0 {
		c.Sample = 1
	}
	if c.SlowLog == 0 {
		c.SlowLog = 500 * time.Millisecond
	}
}

// Middleware 返回 trace 中间件。
func Middleware(cfg Config) core.Handler {
	cfg.norm()
	return func(c *core.Ctx) {
		id := c.Header(headerTraceID)
		if id == "" {
			id = c.Header(headerRequest)
		}
		if id == "" {
			id = newTraceID()
		}
		c.G().Set(ctxKeyTraceID, id)
		// 回写响应头，便于前端/网关串联。
		c.G().Header(cfg.HeaderName, id)

		start := time.Now()
		c.Next()
		dur := time.Since(start)
		if dur >= cfg.SlowLog {
			if cfg.OnSlow != nil {
				cfg.OnSlow(id, c.Method(), c.Path(), dur)
			} else {
				defaultSlowLog(id, c.Method(), c.Path(), dur)
			}
		}
	}
}

// ID 从上下文取出当前 TraceID（供业务/日志使用）。
func ID(c *core.Ctx) string {
	if v, ok := c.G().Get(ctxKeyTraceID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func newTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

func defaultSlowLog(id, method, path string, dur time.Duration) {
	fmt.Fprintf(os.Stderr, "[trace] SLOW %s %s %s %s\n", id, method, path, dur)
}
