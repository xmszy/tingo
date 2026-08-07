// Package cache 提供响应缓存中间件（内存版）。
//
// 设计：零外部依赖，使用带 TTL 的并发安全内存缓存。仅缓存 GET 请求的
// 成功响应（2xx），按方法+路径+查询做键。响应体在写入时完整捕获。
package cache

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

type entry struct {
	status int
	header http.Header
	body   []byte
	expire time.Time
}

// Store 是缓存存储接口，便于替换为 Redis 等实现。
type Store interface {
	Get(key string) (*entry, bool)
	Set(key string, e *entry, ttl time.Duration)
}

type memStore struct {
	mu      sync.RWMutex
	m       map[string]*entry
	ttl     time.Duration
	now     func() time.Time
}

func (s *memStore) Get(key string) (*entry, bool) {
	s.mu.RLock()
	e, ok := s.m[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if s.now().After(e.expire) {
		s.mu.Lock()
		delete(s.m, key)
		s.mu.Unlock()
		return nil, false
	}
	return e, true
}

func (s *memStore) Set(key string, e *entry, ttl time.Duration) {
	e.expire = s.now().Add(ttl)
	s.mu.Lock()
	s.m[key] = e
	s.mu.Unlock()
}

// Config 是缓存配置。
type Config struct {
	// TTL 缓存存活时间。
	TTL time.Duration
	// Store 自定义存储；缺省使用内存存储。
	Store Store
}

// Middleware 返回响应缓存中间件。仅缓存 GET 方法。
func Middleware(ttl time.Duration) core.Handler {
	return MiddlewareWith(Config{TTL: ttl})
}

// MiddlewareWith 允许自定义配置。
func MiddlewareWith(cfg Config) core.Handler {
	store := cfg.Store
	if store == nil {
		store = &memStore{m: make(map[string]*entry), ttl: cfg.TTL, now: time.Now}
	}
	return func(c *core.Ctx) {
		if c.Method() != http.MethodGet {
			c.Next()
			return
		}
		key := c.Method() + " " + c.Path() + "?" + c.RawQuery()
		if e, ok := store.Get(key); ok {
			for k, vs := range e.header {
				for _, v := range vs {
					c.Res().Header().Add(k, v)
				}
			}
			c.Res().WriteHeader(e.status)
			c.Res().Write(e.body)
			c.Abort()
			return
		}
		// 包装 writer 捕获响应。
		bw := &bodyWriter{ResponseWriter: c.Res(), buf: &bytes.Buffer{}}
		c.G().Writer = bw
		c.Next()
		if c.G().Writer.Status() >= 200 && c.G().Writer.Status() < 300 {
			store.Set(key, &entry{
				status: bw.status,
				header: bw.Header().Clone(),
				body:   append([]byte(nil), bw.buf.Bytes()...),
			}, cfg.TTL)
		}
	}
}

type bodyWriter struct {
	gin.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (w *bodyWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}
