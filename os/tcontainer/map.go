package tcontainer

import "github.com/xmszy/tingo/os/tmutex"

// Map 泛型有序 Map。基于简单的线性存储，适用于小数据量场景。
// 大数据量建议使用 Go 内置 map 或 sync.Map。
type Map[K comparable, V any] struct {
	keys   []K
	values []V
}

// NewMap 创建有序 Map。
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{keys: make([]K, 0), values: make([]V, 0)}
}

// Set 设置键值。
func (m *Map[K, V]) Set(key K, value V) {
	for i, k := range m.keys {
		if k == key {
			m.values[i] = value
			return
		}
	}
	m.keys = append(m.keys, key)
	m.values = append(m.values, value)
}

// Get 获取键值。
func (m *Map[K, V]) Get(key K) (V, bool) {
	for i, k := range m.keys {
		if k == key {
			return m.values[i], true
		}
	}
	var zero V
	return zero, false
}

// MustGet 获取键值，不存在时返回零值。
func (m *Map[K, V]) MustGet(key K) V {
	v, _ := m.Get(key)
	return v
}

// Remove 删除键。
func (m *Map[K, V]) Remove(key K) {
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			m.values = append(m.values[:i], m.values[i+1:]...)
			return
		}
	}
}

// Has 判断键是否存在。
func (m *Map[K, V]) Has(key K) bool {
	_, ok := m.Get(key)
	return ok
}

// Len 返回键值对数量。
func (m *Map[K, V]) Len() int { return len(m.keys) }

// Keys 返回所有键（按插入顺序）。
func (m *Map[K, V]) Keys() []K {
	ks := make([]K, len(m.keys))
	copy(ks, m.keys)
	return ks
}

// Values 返回所有值（按插入顺序）。
func (m *Map[K, V]) Values() []V {
	vs := make([]V, len(m.values))
	copy(vs, m.values)
	return vs
}

// Range 遍历。
func (m *Map[K, V]) Range(fn func(key K, value V) bool) {
	for i, k := range m.keys {
		if !fn(k, m.values[i]) {
			break
		}
	}
}

// Clear 清空。
func (m *Map[K, V]) Clear() {
	m.keys = m.keys[:0]
	m.values = m.values[:0]
}

// Clone 浅拷贝。
func (m *Map[K, V]) Clone() *Map[K, V] {
	cp := NewMap[K, V]()
	cp.keys = make([]K, len(m.keys))
	cp.values = make([]V, len(m.values))
	copy(cp.keys, m.keys)
	copy(cp.values, m.values)
	return cp
}

// ToMap 转为内置 map[K]V。
func (m *Map[K, V]) ToMap() map[K]V {
	result := make(map[K]V, len(m.keys))
	for i, k := range m.keys {
		result[k] = m.values[i]
	}
	return result
}

// MapFrom 从内置 map 创建有序 Map。
func MapFrom[K comparable, V any](src map[K]V) *Map[K, V] {
	m := NewMap[K, V]()
	for k, v := range src {
		m.Set(k, v)
	}
	return m
}

// ──────────────── HashMap（O(1) 并发安全 Map） ────────────────

// HashMap 基于原生 map 的并发安全键值容器，读写复杂度 O(1)。
// safe 为 false 时退化为非线程安全模式（无锁开销）。
type HashMap[K comparable, V any] struct {
	mu   *tmutex.RWMutex
	data map[K]V
}

// NewHashMap 创建 HashMap。safe 为 true 时启用并发安全。
func NewHashMap[K comparable, V any](safe ...bool) *HashMap[K, V] {
	useSafe := len(safe) > 0 && safe[0]
	h := &HashMap[K, V]{data: make(map[K]V)}
	if useSafe {
		h.mu = tmutex.NewRWMutex()
	}
	return h
}

func (h *HashMap[K, V]) locked() bool { return h.mu != nil }

// Set 设置键值。
func (h *HashMap[K, V]) Set(key K, value V) {
	if h.locked() {
		h.mu.Lock()
		defer h.mu.Unlock()
	}
	h.data[key] = value
}

// Get 获取键值。
func (h *HashMap[K, V]) Get(key K) (V, bool) {
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	v, ok := h.data[key]
	return v, ok
}

// MustGet 获取键值，不存在时返回零值。
func (h *HashMap[K, V]) MustGet(key K) V {
	v, _ := h.Get(key)
	return v
}

// GetOrSet 若 key 存在返回其值；否则设置 value 并返回。
func (h *HashMap[K, V]) GetOrSet(key K, value V) V {
	if h.locked() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if v, ok := h.data[key]; ok {
			return v
		}
		h.data[key] = value
		return value
	}
	if v, ok := h.data[key]; ok {
		return v
	}
	h.data[key] = value
	return value
}

// SetIfNotExist 当 key 不存在时才设置，返回是否设置成功。
func (h *HashMap[K, V]) SetIfNotExist(key K, value V) bool {
	if h.locked() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.data[key]; ok {
			return false
		}
		h.data[key] = value
		return true
	}
	if _, ok := h.data[key]; ok {
		return false
	}
	h.data[key] = value
	return true
}

// Remove 删除键。
func (h *HashMap[K, V]) Remove(key K) {
	if h.locked() {
		h.mu.Lock()
		defer h.mu.Unlock()
	}
	delete(h.data, key)
}

// Has 判断键是否存在。
func (h *HashMap[K, V]) Has(key K) bool {
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	_, ok := h.data[key]
	return ok
}

// Len 返回键值对数量。
func (h *HashMap[K, V]) Len() int {
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	return len(h.data)
}

// Keys 返回所有键。
func (h *HashMap[K, V]) Keys() []K {
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	keys := make([]K, 0, len(h.data))
	for k := range h.data {
		keys = append(keys, k)
	}
	return keys
}

// Values 返回所有值。
func (h *HashMap[K, V]) Values() []V {
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	vals := make([]V, 0, len(h.data))
	for _, v := range h.data {
		vals = append(vals, v)
	}
	return vals
}

// Range 遍历所有键值对（遍历期间不应修改）。
func (h *HashMap[K, V]) Range(fn func(key K, value V) bool) {
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	for k, v := range h.data {
		if !fn(k, v) {
			break
		}
	}
}

// Clear 清空。
func (h *HashMap[K, V]) Clear() {
	if h.locked() {
		h.mu.Lock()
		defer h.mu.Unlock()
	}
	h.data = make(map[K]V)
}

// Clone 浅拷贝。
func (h *HashMap[K, V]) Clone() *HashMap[K, V] {
	cp := NewHashMap[K, V](h.locked())
	if h.locked() {
		h.mu.RLock()
		defer h.mu.RUnlock()
	}
	cp.data = make(map[K]V, len(h.data))
	for k, v := range h.data {
		cp.data[k] = v
	}
	return cp
}

// Merge 将 src 的键值合并进当前 Map（src 覆盖同 key）。
func (h *HashMap[K, V]) Merge(src *HashMap[K, V]) {
	if src == nil {
		return
	}
	if h.locked() {
		h.mu.Lock()
		defer h.mu.Unlock()
	}
	if src.locked() {
		src.mu.RLock()
		defer src.mu.RUnlock()
	}
	for k, v := range src.data {
		h.data[k] = v
	}
}

// Flip 键值翻转，返回新 HashMap（值需为 comparable 才可作键）。
func Flip[K comparable, V comparable](src *HashMap[K, V]) *HashMap[V, K] {
	out := NewHashMap[V, K](src.locked())
	src.Range(func(k K, v V) bool {
		out.Set(v, k)
		return true
	})
	return out
}
