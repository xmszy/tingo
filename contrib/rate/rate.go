// Package rate 提供基于令牌桶的 HTTP 限流中间件（contrib 示例子模块）。
//
// 该包属于独立 module github.com/xmszy/tingo/contrib，依赖主模块。
// 设计目标：零外部依赖，使用标准库 golang.org/x/time/rate 风格的算法自实现，
// 避免引入额外依赖；与 thttp/gin 通过 Handler 适配集成。
package rate

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

// Limiter 是令牌桶限流器。
type Limiter struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	rate       float64 // 每秒补充令牌数
	last       time.Time
	cleanup    map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// New 创建限流器，rps 为每秒允许请求数，burst 为突发容量。
func New(rps, burst float64) *Limiter {
	return &Limiter{
		capacity: burst,
		tokens:   burst,
		rate:     rps,
		last:     time.Now(),
		cleanup:  map[string]*bucket{},
	}
}

// Allow 判断 key（如客户端 IP）是否通过限流。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.cleanup[key]
	if !ok {
		b = &bucket{tokens: l.capacity, last: now}
		l.cleanup[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Middleware 返回一个可直接注册到引擎的中间件：
//
//	t.Use(rate.Middleware(100, 20)) // 每秒 100 请求，突发 20
//
// 以客户端 IP 为限流键，超限返回 429（application/json）。
// 未超限时调用 c.Next() 放行后续处理链。
func (l *Limiter) Middleware() core.Handler {
	return func(c *core.Ctx) {
		if !l.Allow(c.G().ClientIP()) {
			c.G().AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many requests",
			})
			return
		}
		c.Next()
	}
}

// Middleware 创建一个 rps/burst 的限流器并返回其中间件。
func Middleware(rps, burst float64) core.Handler {
	return New(rps, burst).Middleware()
}
