// Package ttype 提供并发安全的基本类型。
//
// 每种类型都提供原子操作的 Get/Set 方法，
// 零外部依赖，纯 sync/atomic 实现。
//
// 用法：
//
//	var counter ttype.Int
//	counter.Add(1)
//	fmt.Println(counter.Val()) // 1
package ttype

import (
	"math"
	"sync"
	"sync/atomic"
)

// Int 并发安全的 int 类型。
type Int struct {
	val atomic.Int64
}

// NewInt 创建并发安全 int。
func NewInt(v int) *Int {
	t := &Int{}
	t.Set(v)
	return t
}

// Val 获取当前值。
func (t *Int) Val() int { return int(t.val.Load()) }

// Set 设置值。
func (t *Int) Set(v int) { t.val.Store(int64(v)) }

// Add 增加 delta 并返回新值。
func (t *Int) Add(delta int) int { return int(t.val.Add(int64(delta))) }

// Cas 比较并交换。
func (t *Int) Cas(old, new int) bool { return t.val.CompareAndSwap(int64(old), int64(new)) }

// ---- Int32 ----

// Int32 并发安全的 int32。
type Int32 struct {
	val atomic.Int32
}

func NewInt32(v int32) *Int32 {
	t := &Int32{}
	t.Set(v)
	return t
}
func (t *Int32) Val() int32           { return t.val.Load() }
func (t *Int32) Set(v int32)          { t.val.Store(v) }
func (t *Int32) Add(delta int32) int32 { return t.val.Add(delta) }

// ---- Int64 ----

// Int64 并发安全的 int64。
type Int64 struct {
	val atomic.Int64
}

func NewInt64(v int64) *Int64 {
	t := &Int64{}
	t.Set(v)
	return t
}
func (t *Int64) Val() int64           { return t.val.Load() }
func (t *Int64) Set(v int64)          { t.val.Store(v) }
func (t *Int64) Add(delta int64) int64 { return t.val.Add(delta) }

// ---- Uint32 ----

// Uint32 并发安全的 uint32。
type Uint32 struct {
	val atomic.Uint32
}

func NewUint32(v uint32) *Uint32 {
	t := &Uint32{}
	t.Set(v)
	return t
}
func (t *Uint32) Val() uint32            { return t.val.Load() }
func (t *Uint32) Set(v uint32)           { t.val.Store(v) }
func (t *Uint32) Add(delta uint32) uint32 { return t.val.Add(delta) }

// ---- Uint64 ----

// Uint64 并发安全的 uint64。
type Uint64 struct {
	val atomic.Uint64
}

func NewUint64(v uint64) *Uint64 {
	t := &Uint64{}
	t.Set(v)
	return t
}
func (t *Uint64) Val() uint64            { return t.val.Load() }
func (t *Uint64) Set(v uint64)           { t.val.Store(v) }
func (t *Uint64) Add(delta uint64) uint64 { return t.val.Add(delta) }

// ---- Float64 ----

// Float64 并发安全的 float64。
type Float64 struct {
	val atomic.Uint64
}

func NewFloat64(v float64) *Float64 {
	t := &Float64{}
	t.Set(v)
	return t
}

func (t *Float64) Val() float64 {
	return float64FromBits(t.val.Load())
}

func (t *Float64) Set(v float64) {
	t.val.Store(float64ToBits(v))
}

func (t *Float64) Add(delta float64) float64 {
	for {
		old := t.val.Load()
		cur := float64FromBits(old)
		new := float64ToBits(cur + delta)
		if t.val.CompareAndSwap(old, new) {
			return cur + delta
		}
	}
}

func float64ToBits(f float64) uint64   { return math.Float64bits(f) }
func float64FromBits(b uint64) float64 { return math.Float64frombits(b) }

// ---- Bool ----

// Bool 并发安全的 bool。
type Bool struct {
	val atomic.Int32
}

func NewBool(v bool) *Bool {
	t := &Bool{}
	t.Set(v)
	return t
}

func (t *Bool) Val() bool { return t.val.Load() != 0 }

func (t *Bool) Set(v bool) {
	if v {
		t.val.Store(1)
	} else {
		t.val.Store(0)
	}
}

// True 设置为 true 并返回之前的值。
func (t *Bool) True() bool { return t.val.Swap(1) != 0 }

// False 设置为 false 并返回之前的值。
func (t *Bool) False() bool { return t.val.Swap(0) != 0 }

// Toggle 反转并返回新值。
func (t *Bool) Toggle() bool {
	for {
		old := t.val.Load()
		new := int32(0)
		if old == 0 {
			new = 1
		}
		if t.val.CompareAndSwap(old, new) {
			return new != 0
		}
	}
}

// Cas 比较并交换。
func (t *Bool) Cas(old, new bool) bool {
	var o, n int32
	if old {
		o = 1
	}
	if new {
		n = 1
	}
	return t.val.CompareAndSwap(o, n)
}

// ---- String ----

// String 并发安全的 string。
type String struct {
	mu  sync.RWMutex
	val string
}

func NewString(v string) *String {
	t := &String{}
	t.Set(v)
	return t
}

func (t *String) Val() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.val
}

func (t *String) Set(v string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.val = v
}

func (t *String) Append(s string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.val += s
	return t.val
}

// ---- Any ----

// Any 并发安全的 any 值。
type Any struct {
	mu  sync.RWMutex
	val any
}

func NewAny(v any) *Any {
	t := &Any{}
	t.Set(v)
	return t
}

func (t *Any) Val() any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.val
}

func (t *Any) Set(v any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.val = v
}

func (t *Any) Cas(old, new any) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.val == old {
		t.val = new
		return true
	}
	return false
}

// ---- Map ----

// Map 并发安全的 map。
type Map[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{m: make(map[K]V)}
}

func (m *Map[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[key]
	return v, ok
}

func (m *Map[K, V]) Set(key K, val V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = val
}

func (m *Map[K, V]) Remove(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]K, 0, len(m.m))
	for k := range m.m {
		keys = append(keys, k)
	}
	return keys
}

func (m *Map[K, V]) Vals() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vals := make([]V, 0, len(m.m))
	for _, v := range m.m {
		vals = append(vals, v)
	}
	return vals
}

func (m *Map[K, V]) Range(f func(K, V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
}

func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m = make(map[K]V)
}
