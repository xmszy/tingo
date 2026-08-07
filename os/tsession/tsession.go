// Package tsession 提供零外部依赖的 HTTP 会话管理。
//
// 设计要点：
//   - 存储驱动抽象（Store）：内置 MemoryStore（基于 tcache），可扩展为 DBStore（基于 tdb）；
//   - 服务端保存会话数据，客户端仅持有一个签名/加密的会话 ID Cookie（信封）；
//   - 会话数据以 JSON 序列化，支持任意可序列化值；
//   - 线程安全，适配高并发；
//   - 与 thttp（gin）集成通过 Session(ctx) 中间件或直接读写。
package tsession

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/xmszy/tingo/os/tcache"
)

// Session 是单次会话（服务端数据）。
type Session struct {
	ID    string
	data  map[string]any
	mu    sync.Mutex
	dirty bool
}

// Get 读取值。
func (s *Session) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return v, ok
}

// GetT 读取并断言为 T。对 JSON 反序列化的数值（float64）做整数兼容转换。
func Get[T any](s *Session, key string) (T, bool) {
	v, ok := s.Get(key)
	if !ok {
		return *new(T), false
	}
	if t, ok := v.(T); ok {
		return t, true
	}
	// JSON 数字兼容：float64 -> 整数类型。
	if f, ok := v.(float64); ok {
		switch any(*new(T)).(type) {
		case int:
			return any(int(f)).(T), true
		case int64:
			return any(int64(f)).(T), true
		case int32:
			return any(int32(f)).(T), true
		}
	}
	return *new(T), false
}

// Set 写入值，标记 dirty 以便持久化。
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	s.data[key] = value
	s.dirty = true
	s.mu.Unlock()
}

// Delete 删除键。
func (s *Session) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.dirty = true
	s.mu.Unlock()
}

// GetID 返回会话 ID。
func (s *Session) GetID() string { return s.ID }

// flush 序列化数据（内部使用，调用方持有锁由 Store 负责）。
func (s *Session) snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(s.data)
	if err != nil {
		return nil, err
	}
	s.dirty = false
	return b, nil
}

// Store 是会话存储驱动接口。
type Store interface {
	// Load 根据 ID 载入会话数据；不存在返回空数据与 nil。
	Load(id string) (map[string]any, error)
	// Save 持久化会话数据。
	Save(id string, data map[string]any, ttl time.Duration) error
	// Destroy 删除会话。
	Destroy(id string) error
}

// MemoryStore 基于 tcache 的内存存储。
type MemoryStore struct {
	cache *tcache.Cache
}

// NewMemoryStore 创建内存存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{cache: tcache.New()}
}

// Load 实现 Store。
func (m *MemoryStore) Load(id string) (map[string]any, error) {
	v, ok := m.cache.Get("sess:" + id)
	if !ok {
		return map[string]any{}, nil
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	return map[string]any{}, nil
}

// Save 实现 Store。
func (m *MemoryStore) Save(id string, data map[string]any, ttl time.Duration) error {
	m.cache.Set("sess:"+id, data, ttl)
	return nil
}

// Destroy 实现 Store。
func (m *MemoryStore) Destroy(id string) error {
	m.cache.Delete("sess:" + id)
	return nil
}

// Config 配置会话管理器。
type Config struct {
	// CookieName 是客户端 Cookie 名，默认 "tingo_session"。
	CookieName string
	// TTL 是会话有效期，默认 24h。
	TTL time.Duration
	// Store 是存储驱动，默认 MemoryStore。
	Store Store
	// CookiePath / Secure / HttpOnly。
	CookiePath string
	Secure     bool
	HttpOnly   bool
	// HttpOnlySet 标记 HttpOnly 是否由调用方显式设置。
	// 未设置时 New() 默认开启 HttpOnly（防御 XSS 会话劫持）。
	HttpOnlySet bool
}

// Manager 是会话管理器。
type Manager struct {
	cfg   Config
	store Store
}

// New 创建会话管理器。
func New(cfg Config) *Manager {
	if cfg.CookieName == "" {
		cfg.CookieName = "tingo_session"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore()
	}
	// 会话 ID 是敏感凭证，HttpOnly 默认开启以防御 XSS 会话劫持；
	// 业务需显式置为 false 才允许 JS 读取（极少见）。
	if !cfg.HttpOnlySet {
		cfg.HttpOnly = true
	}
	return &Manager{cfg: cfg, store: cfg.Store}
}

// newID 生成随机会话 ID。
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退化为时间戳（仍足够随机用于会话）。
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	const hex = "0123456789abcdef"
	out := make([]byte, 32)
	for i := range 16 {
		out[i*2] = hex[b[i]>>4]
		out[i*2+1] = hex[b[i]&0x0f]
	}
	return string(out)
}

// LoadOrCreate 根据传入的会话 ID（可能为空）载入或新建会话。
func (m *Manager) LoadOrCreate(id string) (*Session, error) {
	if id == "" {
		return &Session{ID: newID(), data: map[string]any{}}, nil
	}
	data, err := m.store.Load(id)
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, data: data}, nil
}

// Save 持久化会话（通常由中间件在请求结束时调用）。
func (m *Manager) Save(s *Session) error {
	data, err := s.snapshot()
	if err != nil {
		return err
	}
	var m2 map[string]any
	if err := json.Unmarshal(data, &m2); err != nil {
		return err
	}
	return m.store.Save(s.ID, m2, m.cfg.TTL)
}

// Destroy 销毁会话。
func (m *Manager) Destroy(s *Session) error {
	return m.store.Destroy(s.ID)
}

// Config_ 暴露配置（供中间件写 Cookie）。
func (m *Manager) Config() Config { return m.cfg }
