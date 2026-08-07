// Package ratelimit 提供请求限流中间件。
//
// 提供两种后端：
//   - 内存令牌桶：零外部依赖，单机限流（注意多实例需各自独立计数）；
//   - Redis 分布式滑动窗口：跨实例统一限流，依赖 go-redis/v9。
//
// 限流键默认取客户端 IP，可通过 KeyFunc 自定义（如按用户/API Key）。
package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xmszy/tingo/core"
)

// Config 通用配置。
type Config struct {
	// Rate 每秒允许的请求数（令牌桶补充速率）。
	Rate float64
	// Burst 桶容量（瞬时突发上限）。
	Burst int
	// KeyFunc 自定义限流键；默认按客户端 IP。
	KeyFunc func(c *core.Ctx) string
	// Error 自定义超限响应；默认 429。
	Error func(c *core.Ctx)
	// Redis 可选；非空则走分布式滑动窗口限流。
	Redis *redis.Client
	// Prefix Redis 键前缀，默认 tingo:rl:.
	Prefix string
}

// Middleware 根据配置返回限流中间件。
func Middleware(cfg Config) core.Handler {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *core.Ctx) string { return c.IP() }
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "tingo:rl:"
	}
	if cfg.Redis != nil {
		return redisLimiter(cfg)
	}
	return newMemoryLimiter(cfg)
}

/* ----------------------------- 内存令牌桶 ----------------------------- */

type memBucket struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	rate     float64
	burst    float64
}

func (b *memBucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type memoryLimiter struct {
	cfg    Config
	buckets sync.Map // key -> *memBucket
}

func newMemoryLimiter(cfg Config) core.Handler {
	return (&memoryLimiter{cfg: cfg}).handle
}

func (m *memoryLimiter) handle(c *core.Ctx) {
	key := m.cfg.KeyFunc(c)
	var b *memBucket
	if v, ok := m.buckets.Load(key); ok {
		b = v.(*memBucket)
	} else {
		b = &memBucket{tokens: float64(m.cfg.Burst), last: time.Now(), rate: m.cfg.Rate, burst: float64(m.cfg.Burst)}
		m.buckets.Store(key, b)
	}
	if !b.allow(time.Now()) {
		deny(c, m.cfg)
		return
	}
	c.Next()
}

/* ----------------------------- Redis 滑动窗口 ----------------------------- */

func redisLimiter(cfg Config) core.Handler {
	rdb := cfg.Redis
	return func(c *core.Ctx) {
		key := cfg.Prefix + cfg.KeyFunc(c)
		ctx := context.Background()
		now := time.Now()
		window := time.Duration(float64(time.Second) * float64(cfg.Burst) / cfg.Rate)
		// 用 ZSET 记录请求时间戳，窗口外清理，统计窗口内数量。
		pipe := rdb.TxPipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", fmtFloat(float64(now.Add(-window).UnixNano())))
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
		pipe.Expire(ctx, key, window+time.Second)
		pipe.ZCard(ctx, key)
		cmders, err := pipe.Exec(ctx)
		if err != nil {
			// Redis 不可用时放行（fail open），避免限流组件拖垮服务。
			c.Next()
			return
		}
		count := cmders[len(cmders)-1].(*redis.IntCmd).Val()
		if int(count) > cfg.Burst {
			deny(c, cfg)
			return
		}
		c.Next()
	}
}

/* ----------------------------- 公共 ----------------------------- */

func deny(c *core.Ctx, cfg Config) {
	if cfg.Error != nil {
		cfg.Error(c)
		return
	}
	c.G().AbortWithStatusJSON(http.StatusTooManyRequests, map[string]any{
		"code":    http.StatusTooManyRequests,
		"message": "too many requests",
	})
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
