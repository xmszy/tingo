package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/xmszy/tingo/core"
)

/* ------------------------------------------------------------------ */
/* 限流中间件                                                            */
/* ------------------------------------------------------------------ */

// RateLimiter 是限流判定接口，便于替换为 redis/分布式实现。
type RateLimiter interface {
	// Allow 返回该 key 本次是否放行，并在放行时消耗一个配额。
	Allow(key string) bool
}

// RateLimitConfig 是限流中间件的配置。
type RateLimitConfig struct {
	// Limit 是时间窗内允许的最大请求数。
	Limit int
	// Window 是限流时间窗。
	Window time.Duration
	// KeyFunc 生成限流键（默认按客户端 IP）。
	// 例如按用户 token 限流时自定义。
	KeyFunc func(c *core.Ctx) string
	// Limiter 自定义限流器（默认进程内令牌桶）。
	// 分布式部署应注入 redis 实现。
	Limiter RateLimiter
	// ErrCode 是限流拒绝时的业务码（写入 WriteError）。
	ErrCode int
	// ErrMessage 是限流拒绝时的提示文案。
	ErrMessage string
}

// DefaultRateLimitConfig 返回默认配置：单 IP 每 60 秒 60 次。
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Limit:   60,
		Window:  time.Minute,
		KeyFunc: func(c *core.Ctx) string { return c.IP() },
		ErrCode: 429,
		ErrMessage: "too many requests",
	}
}

// tokenBucket 是简单的进程内固定窗口令牌桶。
// 每个 key 独立计数，窗口结束后重置。并发安全。
type tokenBucket struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	buckets  map[string]*bucketState
	now      func() time.Time
	lastSweep time.Time
}

type bucketState struct {
	count    int
	resetAt  time.Time
}

func newTokenBucket(limit int, window time.Duration) *tokenBucket {
	return &tokenBucket{
		limit:     limit,
		window:    window,
		buckets:   make(map[string]*bucketState),
		now:       time.Now,
		lastSweep: time.Now(),
	}
}

func (t *tokenBucket) Allow(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	// 周期性清理过期桶，避免内存无限增长。
	if now.Sub(t.lastSweep) > t.window {
		for k, b := range t.buckets {
			if now.After(b.resetAt) {
				delete(t.buckets, k)
			}
		}
		t.lastSweep = now
	}

	b, ok := t.buckets[key]
	if !ok || now.After(b.resetAt) {
		b = &bucketState{count: 0, resetAt: now.Add(t.window)}
		t.buckets[key] = b
	}
	if b.count >= t.limit {
		return false
	}
	b.count++
	return true
}

// RateLimit 返回限流中间件。
//
// 默认按客户端 IP 做固定窗口限流；可通过 opts 自定义 key（如按用户）、
// 限流阈值或替换为分布式 limiter。
//
//	e.Router().Use(middleware.RateLimit())
//	e.Router().Use(middleware.RateLimit(func(c *middleware.RateLimitConfig) {
//	    c.Limit = 100
//	    c.Window = time.Minute
//	}))
func RateLimit(opts ...func(*RateLimitConfig)) core.Handler {
	cfg := DefaultRateLimitConfig()
	for _, o := range opts {
		o(&cfg)
	}
	limiter := cfg.Limiter
	if limiter == nil {
		limiter = newTokenBucket(cfg.Limit, cfg.Window)
	}

	return func(c *core.Ctx) {
		key := ""
		if cfg.KeyFunc != nil {
			key = cfg.KeyFunc(c)
		}
		if key == "" {
			c.Next()
			return
		}
		if !limiter.Allow(key) {
			c.SetHeader("Retry-After", strconv.Itoa(int(cfg.Window.Seconds())))
			c.JSONStatus(http.StatusTooManyRequests, map[string]any{
				"code": cfg.ErrCode,
				"msg":  cfg.ErrMessage,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
