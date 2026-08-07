package sessions

import "sync"

// cookieStore 是基于内存的安全 Cookie 会话存储。
// 会话数据保存在服务端内存，浏览器仅持有一个随机 ID Cookie。
type cookieStore struct {
	mu sync.RWMutex
	m  map[string]*session
}

// NewCookieStore 创建一个内存会话存储。
func NewCookieStore() Store {
	return &cookieStore{m: make(map[string]*session)}
}

func (s *cookieStore) New(id string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := &session{id: id, data: make(map[string]any)}
	s.m[id] = ss
	return ss
}

func (s *cookieStore) Read(id string) (Session, bool) {
	s.mu.RLock()
	ss, ok := s.m[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return ss, true
}

func (s *cookieStore) Save(ss Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s2, ok := ss.(*session); ok {
		s.m[ss.ID()] = s2
	}
}
