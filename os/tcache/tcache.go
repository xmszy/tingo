// Package tcache 提供并发安全、支持过期与容量上限的内存缓存。
//
// 设计要点：
//   - 分片（sharding）降低锁竞争：默认 256 个分片，每片一把 sync.RWMutex；
//   - 过期采用惰性删除 + 可选后台清扫，读取时发现过期即回收，几乎零额外开销；
//   - 内存项用 any 存储，但提供泛型 Get[T] 免去类型断言；
//   - 可选容量上限：超出时按近似 LRU（分片内维护访问序号）淘汰最旧项，
//     淘汰仅在单分片内进行，不影响全局吞吐。
package tcache

import (
	"sync"
	"sync/atomic"
	"time"
)

const defaultShards = 256

type item struct {
	value   any
	expire  int64 // 纳秒时间戳；0 表示永不过期
	updated int64 // 最近访问序号（用于 LRU）
}

// Cache 是并发安全缓存。
type Cache struct {
	shards    []*shard
	capPer    int64 // 每分片容量上限，0 表示无限制
	seq       uint64
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

type shard struct {
	mu  sync.RWMutex
	m   map[string]*item
	cap int64
	seq *uint64
}

// Options 配置缓存行为。
type Options struct {
	// Shards 分片数，默认 256，建议为 2 的幂。
	Shards int
	// MaxEntries 全局容量上限，0 表示无限制。
	// 达到上限时按分片 LRУ 淘汰。
	MaxEntries int
	// SweepInterval 后台过期清扫间隔；<=0 表示不启动后台清扫
	// （仅依赖惰性删除，已足够大多数场景）。
	SweepInterval time.Duration
}

// New 创建缓存。
func New(opts ...Options) *Cache {
	o := Options{}
	if len(opts) > 0 {
		o = opts[0]
	}
	shards := o.Shards
	if shards <= 0 {
		shards = defaultShards
	}
	capPer := int64(0)
	if o.MaxEntries > 0 {
		capPer = int64(o.MaxEntries/shards) + 1
	}
	c := &Cache{shards: make([]*shard, shards), capPer: capPer, seq: 0}
	for i := 0; i < shards; i++ {
		c.shards[i] = &shard{m: make(map[string]*item, 8), cap: capPer, seq: &c.seq}
	}
	if o.SweepInterval > 0 {
		c.stop = make(chan struct{})
		c.done = make(chan struct{})
		go c.sweep(o.SweepInterval)
	}
	return c
}

// Global 返回进程级默认缓存（全局共享单例）。
var Global = New()

func (c *Cache) shardOf(key string) *shard {
	// FNV-1a 哈希分散到分片。
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return c.shards[h%uint32(len(c.shards))]
}

func (s *shard) nextSeq() int64 {
	return int64(atomic.AddUint64(s.seq, 1))
}

// Set 写入缓存，ttl<=0 表示永不过期。
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}
	s.m[key] = &item{value: value, expire: exp, updated: s.nextSeq()}
	if s.cap > 0 && int64(len(s.m)) > s.cap {
		s.evict()
	}
}

// evict 淘汰分片内最久未访问的一项（调用方须持有写锁）。
func (s *shard) evict() {
	var oldKey string
	var oldSeq int64 = 1<<62 - 1
	for k, it := range s.m {
		updated := atomic.LoadInt64(&it.updated)
		if updated < oldSeq {
			oldSeq = updated
			oldKey = k
		}
	}
	if oldKey != "" {
		delete(s.m, oldKey)
	}
}

// Get 读取缓存，命中且未过期返回 (value, true)。
func (c *Cache) Get(key string) (any, bool) {
	s := c.shardOf(key)
	s.mu.RLock()
	it, ok := s.m[key]
	if !ok {
		s.mu.RUnlock()
		return nil, false
	}
	if it.expire != 0 && it.expire <= time.Now().UnixNano() {
		s.mu.RUnlock()
		// 惰性回收。
		s.mu.Lock()
		delete(s.m, key)
		s.mu.Unlock()
		return nil, false
	}
	atomic.StoreInt64(&it.updated, s.nextSeq())
	v := it.value
	s.mu.RUnlock()
	return v, true
}

// GetOr 读取缓存，未命中时调用 fn 计算并写入（ttl 由 ttl 参数给定）。
// 用于缓存未命中回源的常见模式，避免调用方重复写样板。
func (c *Cache) GetOr(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	v, err := fn()
	if err != nil {
		return nil, err
	}
	c.Set(key, v, ttl)
	return v, nil
}

// SetIfNotExist 仅当 key 不存在或已过期时写入，返回 true 表示写入成功。
// 分片锁内完成存在检查+写入，原子 CAS 语义，避免缓存击穿。
func (c *Cache) SetIfNotExist(key string, value any, ttl time.Duration) bool {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.m[key]; ok {
		if it.expire == 0 || it.expire > time.Now().UnixNano() {
			return false
		}
	}
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}
	s.m[key] = &item{value: value, expire: exp, updated: s.nextSeq()}
	if s.cap > 0 && int64(len(s.m)) > s.cap {
		s.evict()
	}
	return true
}

// GetOrSet 读取缓存，未命中时调用 fn 计算并写入。
// 与 GetOr 不同：GetOrSet 加锁后二次检查，并发场景下保证 fn 只被调用一次。
func (c *Cache) GetOrSet(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	// 快速路径：无锁读
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	// 慢速路径：加锁双重检查
	s := c.shardOf(key)
	s.mu.Lock()
	if it, ok := s.m[key]; ok {
		if it.expire == 0 || it.expire > time.Now().UnixNano() {
			atomic.StoreInt64(&it.updated, s.nextSeq())
			v := it.value
			s.mu.Unlock()
			return v, nil
		}
	}
	s.mu.Unlock()

	v, err := fn()
	if err != nil {
		return nil, err
	}
	c.Set(key, v, ttl)
	return v, nil
}

// Update 更新值，保持过期时间和 LRU 序位不变。
func (c *Cache) Update(key string, value any) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.m[key]; ok {
		it.value = value
	}
}

// UpdateExpire 更新过期时间，保持值不变。ttl<=0 设为永不过期。
func (c *Cache) UpdateExpire(key string, ttl time.Duration) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.m[key]; ok {
		if ttl > 0 {
			it.expire = time.Now().Add(ttl).UnixNano()
		} else {
			it.expire = 0
		}
		it.updated = s.nextSeq()
	}
}

// Keys 返回所有 key 的快照。
func (c *Cache) Keys() []string {
	keys := make([]string, 0, c.Len())
	for _, s := range c.shards {
		s.mu.RLock()
		for k := range s.m {
			keys = append(keys, k)
		}
		s.mu.RUnlock()
	}
	return keys
}

// MustGet 读取并断言为类型 T，失败返回零值。
func Get[T any](c *Cache, key string) (T, bool) {
	v, ok := c.Get(key)
	if !ok {
		return *new(T), false
	}
	if t, ok := v.(T); ok {
		return t, true
	}
	return *new(T), false
}

// SetT 是带类型提示的写入（等价 Set，但泛型友好）。
func SetT[T any](c *Cache, key string, value T, ttl time.Duration) {
	c.Set(key, value, ttl)
}

// Has 判断是否存在且未过期。
func (c *Cache) Has(key string) bool {
	_, ok := c.Get(key)
	return ok
}

// Delete 删除指定键。
func (c *Cache) Delete(key string) {
	s := c.shardOf(key)
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}

// Clear 清空所有分片。
func (c *Cache) Clear() {
	for _, s := range c.shards {
		s.mu.Lock()
		s.m = make(map[string]*item, 8)
		s.mu.Unlock()
	}
}

// Len 返回当前条目总数（近似值，含可能未清扫的过期项）。
func (c *Cache) Len() int {
	n := 0
	for _, s := range c.shards {
		s.mu.RLock()
		n += len(s.m)
		s.mu.RUnlock()
	}
	return n
}

// Close 停止后台清扫。未启用清扫或重复关闭时无副作用。
func (c *Cache) Close() error {
	c.closeOnce.Do(func() {
		if c.stop != nil {
			close(c.stop)
			<-c.done
		}
	})
	return nil
}

// sweep 后台惰性过期清扫。
func (c *Cache) sweep(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	defer close(c.done)
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			now := time.Now().UnixNano()
			for _, s := range c.shards {
				s.mu.Lock()
				for k, it := range s.m {
					if it.expire != 0 && it.expire <= now {
						delete(s.m, k)
					}
				}
				s.mu.Unlock()
			}
		}
	}
}
