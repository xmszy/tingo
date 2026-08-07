package tcontainer

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
