package tcache

import (
	"fmt"
	"sync"
	"time"
)

// ──────────────── Tag 支持 ────────────────
// tag→keys 映射，标签清除时批量删除关联的缓存 key。
// 设计要点：
//   - 标签集合绑定到具体 Cache 实例，不依赖全局单例。
//   - SetTag 在写入缓存时自动建立 key→tag 的反向映射（不增加额外内存分配路径）。
//   - FlushTag 原子删除该 tag 下所有 key。
//   - 支持单个 tag 下任意数量 key，底层为 set（map[string]struct{}）。

// TagSet 维护标签到缓存键的映射，用于批量失效。
type TagSet struct {
	cache *Cache
	mu    sync.RWMutex
	tags  map[string]map[string]struct{} // tag → keys
}

// NewTagSet 创建一个标签集，关联到指定 Cache 实例。
func NewTagSet(c *Cache) *TagSet {
	return &TagSet{cache: c, tags: make(map[string]map[string]struct{})}
}

// GlobalTag 返回与全局 Cache 绑定的 TagSet。
var GlobalTag = NewTagSet(Global)

// SetTag 写入带标签的缓存。ttl<=0 表示永不过期。
func (ts *TagSet) SetTag(key string, value any, ttl time.Duration, tags ...string) {
	ts.cache.Set(key, value, ttl)
	ts.linkTags(key, tags...)
}

// SetTagIfNotExist 仅当 key 不存在时写入带标签的缓存。
func (ts *TagSet) SetTagIfNotExist(key string, value any, ttl time.Duration, tags ...string) bool {
	if ts.cache.SetIfNotExist(key, value, ttl) {
		ts.linkTags(key, tags...)
		return true
	}
	return false
}

// linkTags 建立 key→tag 关联。
func (ts *TagSet) linkTags(key string, tags ...string) {
	if len(tags) == 0 {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, tag := range tags {
		ks, ok := ts.tags[tag]
		if !ok {
			ks = make(map[string]struct{})
			ts.tags[tag] = ks
		}
		ks[key] = struct{}{}
	}
}

// FlushTag 清除指定标签下所有缓存。返回实际删除的 key 数量。
// 如果某 key 同时被其他标签引用，则仅从当前标签解绑，不删除缓存。
func (ts *TagSet) FlushTag(tags ...string) int {
	if len(tags) == 0 {
		return 0
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	n := 0
	for _, tag := range tags {
		ks, ok := ts.tags[tag]
		if !ok {
			continue
		}
		for k := range ks {
			// 检查该 key 是否被其他标签引用
			if ts.hasOtherTag(k, tag) {
				// 仅从当前标签解绑，保留缓存
				// （从 ks 删除会在循环后被 delete(ts.tags, tag) 处理，无需单独操作）
			} else {
				ts.cache.Delete(k)
				n++
			}
		}
		delete(ts.tags, tag)
	}
	return n
}

// hasOtherTag 检查 key 是否被非 excludeTag 的其他标签引用。
func (ts *TagSet) hasOtherTag(key, excludeTag string) bool {
	for tag, ks := range ts.tags {
		if tag == excludeTag {
			continue
		}
		if _, ok := ks[key]; ok {
			return true
		}
	}
	return false
}

// AppendTag 为已存在的 key 追加标签（不覆盖已有缓存值）。
func (ts *TagSet) AppendTag(key string, tags ...string) {
	if len(tags) == 0 {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, tag := range tags {
		ks, ok := ts.tags[tag]
		if !ok {
			ks = make(map[string]struct{})
			ts.tags[tag] = ks
		}
		ks[key] = struct{}{}
	}
}

// ──────────────── Remember 系列 ────────────────
// 防缓存击穿：GetOrSet 虽有双重检查，但多协程仍可能同时进入 fn。
// Remember 使用互斥锁让同一 key 只允许一个协程执行 fn，其他协程等待结果。

type rememberSlot struct {
	mu   sync.Mutex
	val  any
	err  error
	done bool
}

// Remember 读缓存；未命中时加锁执行 fn，其他协程等待而非重复执行。
// 典型场景：数据库回源、外部 API 调用等昂贵操作。
func (c *Cache) Remember(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	// 快速路径
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	// 计算锁槽（FNV hash，与 shard 选路一致）
	slot := c.rememberSlotOf(key)
	slot.mu.Lock()
	defer slot.mu.Unlock()
	// 二次检查
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	// 执行 fn（只有一个协程会走到这里）
	v, err := fn()
	if err != nil {
		return nil, err
	}
	c.Set(key, v, ttl)
	return v, nil
}

// RememberFunc 是泛型的 Remember，fn 返回值的类型更准确。
func RememberFunc[T any](c *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	if v, ok := Get[T](c, key); ok {
		return v, nil
	}
	slot := c.rememberSlotOf(key)
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if v, ok := Get[T](c, key); ok {
		return v, nil
	}
	v, err := fn()
	if err != nil {
		return *new(T), err
	}
	SetT(c, key, v, ttl)
	return v, nil
}

// rememberSlots 是与 Cache 绑定的 Remember 锁槽。
var rememberLock sync.Mutex
var rememberSlots = make(map[*Cache]map[string]*rememberSlot)

func (c *Cache) rememberSlotOf(key string) *rememberSlot {
	rememberLock.Lock()
	defer rememberLock.Unlock()
	m, ok := rememberSlots[c]
	if !ok {
		m = make(map[string]*rememberSlot)
		rememberSlots[c] = m
	}
	slot, ok := m[key]
	if !ok {
		slot = &rememberSlot{}
		m[key] = slot
	}
	return slot
}

// ──────────────── 原子计数器 ────────────────

// Increment 将缓存的数字值 +1，返回更新后的值。
// 若 key 不存在则创建并设为 1；若类型非 int64/float64 则返回错误。
func (c *Cache) Increment(key string) (int64, error) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.m[key]; ok {
		if it.expire != 0 && it.expire <= time.Now().UnixNano() {
			s.m[key] = &item{value: int64(1), expire: 0, updated: s.nextSeq()}
			return 1, nil
		}
		var n int64
		switch v := it.value.(type) {
		case int:
			n = int64(v) + 1
		case int64:
			n = v + 1
		case float64:
			n = int64(v) + 1
		default:
			return 0, fmt.Errorf("tcache: cannot increment %T", it.value)
		}
		it.value = n
		return n, nil
	}
	s.m[key] = &item{value: int64(1), expire: 0, updated: s.nextSeq()}
	return 1, nil
}

// Decrement 将缓存的数字值 -1，返回更新后的值。
func (c *Cache) Decrement(key string) (int64, error) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.m[key]; ok {
		if it.expire != 0 && it.expire <= time.Now().UnixNano() {
			s.m[key] = &item{value: int64(0), expire: 0, updated: s.nextSeq()}
			return 0, nil
		}
		var n int64
		switch v := it.value.(type) {
		case int:
			n = int64(v) - 1
		case int64:
			n = v - 1
		case float64:
			n = int64(v) - 1
		default:
			return 0, fmt.Errorf("tcache: cannot decrement %T", it.value)
		}
		it.value = n
		return n, nil
	}
	s.m[key] = &item{value: int64(0), expire: 0, updated: s.nextSeq()}
	return 0, nil
}

// IncrementBy 将缓存的数字值增加 delta，返回更新后的值。
func (c *Cache) IncrementBy(key string, delta int64) (int64, error) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.m[key]; ok {
		if it.expire != 0 && it.expire <= time.Now().UnixNano() {
			s.m[key] = &item{value: delta, expire: 0, updated: s.nextSeq()}
			return delta, nil
		}
		var n int64
		switch v := it.value.(type) {
		case int:
			n = int64(v) + delta
			it.value = n
		case int64:
			n = v + delta
			it.value = n
		case float64:
			n = int64(v) + delta
			it.value = n
		default:
			return 0, fmt.Errorf("tcache: cannot increment %T", it.value)
		}
		return n, nil
	}
	s.m[key] = &item{value: delta, expire: 0, updated: s.nextSeq()}
	return delta, nil
}

// DecrementBy 将缓存的数字值减去 delta，返回更新后的值。
func (c *Cache) DecrementBy(key string, delta int64) (int64, error) {
	return c.IncrementBy(key, -delta)
}

// Pull 获取缓存值后立即删除（原子操作）。
func (c *Cache) Pull(key string) (any, bool) {
	s := c.shardOf(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.m[key]
	if !ok {
		return nil, false
	}
	if it.expire != 0 && it.expire <= time.Now().UnixNano() {
		delete(s.m, key)
		return nil, false
	}
	delete(s.m, key)
	return it.value, true
}

// PullFunc 泛型版 Pull。
func PullFunc[T any](c *Cache, key string) (T, bool) {
	v, ok := c.Pull(key)
	if !ok {
		return *new(T), false
	}
	if t, ok := v.(T); ok {
		return t, true
	}
	return *new(T), false
}


