package sessions

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore 是基于 Redis 的会话存储（go-redis/v9）。
type redisStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisStore 创建一个 Redis 会话存储。
// ttl 为会话空闲过期时间，建议与会话 MaxAge 一致。
func NewRedisStore(client *redis.Client, prefix string, ttl time.Duration) Store {
	if prefix == "" {
		prefix = "tingo:session:"
	}
	return &redisStore{client: client, prefix: prefix, ttl: ttl}
}

func (s *redisStore) key(id string) string { return s.prefix + id }

func (s *redisStore) New(id string) Session {
	return &session{id: id, data: make(map[string]any)}
}

func (s *redisStore) Read(id string) (Session, bool) {
	ctx := context.Background()
	val, err := s.client.Get(ctx, s.key(id)).Result()
	if err != nil {
		return nil, false
	}
	data := make(map[string]any)
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, false
	}
	return &session{id: id, data: data}, true
}

func (s *redisStore) Save(ss Session) {
	s2, ok := ss.(*session)
	if !ok {
		return
	}
	b, err := json.Marshal(s2.data)
	if err != nil {
		return
	}
	ctx := context.Background()
	_ = s.client.Set(ctx, s.key(ss.ID()), b, s.ttl).Err()
}
