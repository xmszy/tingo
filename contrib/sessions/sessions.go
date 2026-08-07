// Package sessions 提供会话中间件。
//
// 设计：定义 Store 接口，内置基于安全 Cookie 的内存/文件存储，并可选接入
// Redis（go-redis/v9）。中间件为每个请求注入会话，结束时自动保存。
package sessions

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/xmszy/tingo/core"
)

// Session 是单个会话。
type Session interface {
	// ID 返回会话标识。
	ID() string
	// Get 读取值（interface{}）。
	Get(key string) any
	// Set 写入值。
	Set(key string, value any)
	// Delete 删除键。
	Delete(key string)
	// Clear 清空。
	Clear()
	// Flush 标记销毁。
	Flush()
}

// Store 是会话存储接口。
type Store interface {
	// New 创建（或复用）会话。
	New(id string) Session
	// Save 持久化会话。
	Save(s Session)
	// Read 读取已存在的会话，不存在返回 (nil, false)。
	Read(id string) (Session, bool)
}

type session struct {
	mu   sync.RWMutex
	id   string
	data map[string]any
}

func (s *session) ID() string { return s.id }

func (s *session) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

func (s *session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = map[string]any{}
}

func (s *session) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = map[string]any{}
}

// Config 是会话中间件配置。
type Config struct {
	// Store 底层存储；缺省使用 CookieStore（内存）。
	Store Store
	// CookieName 浏览器 Cookie 名，默认 "tingo_session"。
	CookieName string
	// Path Cookie 路径。
	Path string
	// MaxAge Cookie 有效期（秒），0 表示会话级。
	MaxAge int
	// Secure 仅 HTTPS 传输。
	Secure bool
	// HttpOnly 禁止 JS 访问。
	HttpOnly bool
}

// Middleware 返回会话中间件。
func Middleware(cfg Config) core.Handler {
	if cfg.Store == nil {
		cfg.Store = NewCookieStore()
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "tingo_session"
	}
	if cfg.Path == "" {
		cfg.Path = "/"
	}
	return func(c *core.Ctx) {
		id := readCookie(c, cfg.CookieName)
		var s Session
		if id != "" {
			if existing, ok := cfg.Store.Read(id); ok {
				s = existing
			}
		}
		if s == nil {
			if id == "" {
				id = genID()
			}
			s = cfg.Store.New(id)
		}
		c.G().Set("session", s)
		c.Next()
		// 保存并写回 Cookie（即便是新会话也要回写，否则后续请求丢失身份）。
		cfg.Store.Save(s)
		writeCookie(c, cfg, id)
	}
}

// FromContext 获取当前会话。
func FromContext(c *core.Ctx) Session {
	if v, ok := c.G().Get("session"); ok {
		if s, ok := v.(Session); ok {
			return s
		}
	}
	return nil
}

func readCookie(c *core.Ctx, name string) string {
	ck, err := c.G().Request.Cookie(name)
	if err != nil {
		return ""
	}
	return ck.Value
}

func writeCookie(c *core.Ctx, cfg Config, id string) {
	http.SetCookie(c.Res(), &http.Cookie{
		Name:     cfg.CookieName,
		Value:    id,
		Path:     cfg.Path,
		MaxAge:   cfg.MaxAge,
		Secure:   cfg.Secure,
		HttpOnly: cfg.HttpOnly,
	})
}

func genID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// 保留 time 引用（MaxAge 相关的 TTL 策略可扩展）。
var _ = time.Second
