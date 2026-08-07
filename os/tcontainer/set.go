package tcontainer

// Set 泛型集合。使用自定义相等函数，支持任意类型。
type Set[T any] struct {
	data []T
	eq   func(a, b T) bool
}

// NewSet 创建集合。
func NewSet[T any](eq func(a, b T) bool) *Set[T] {
	return &Set[T]{data: make([]T, 0), eq: eq}
}

// Add 添加元素。
func (s *Set[T]) Add(value T) {
	if s.eq == nil {
		s.data = append(s.data, value)
		return
	}
	for _, v := range s.data {
		if s.eq(v, value) {
			return
		}
	}
	s.data = append(s.data, value)
}

// AddIfNotExist 添加不存在的元素，返回是否新增。
func (s *Set[T]) AddIfNotExist(value T) bool {
	if s.eq == nil {
		s.data = append(s.data, value)
		return true
	}
	for _, v := range s.data {
		if s.eq(v, value) {
			return false
		}
	}
	s.data = append(s.data, value)
	return true
}

// Remove 移除元素。
func (s *Set[T]) Remove(value T) {
	if s.eq == nil {
		return
	}
	for i, v := range s.data {
		if s.eq(v, value) {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return
		}
	}
}

// Contains 判断是否存在。
func (s *Set[T]) Contains(value T) bool {
	if s.eq == nil {
		return false
	}
	for _, v := range s.data {
		if s.eq(v, value) {
			return true
		}
	}
	return false
}

// Len 返回元素数量。
func (s *Set[T]) Len() int { return len(s.data) }

// Slice 返回底层切片。
func (s *Set[T]) Slice() []T { return s.data }

// Range 遍历。
func (s *Set[T]) Range(fn func(value T) bool) {
	for _, v := range s.data {
		if !fn(v) {
			break
		}
	}
}

// Clear 清空。
func (s *Set[T]) Clear() { s.data = s.data[:0] }

// Clone 浅拷贝。
func (s *Set[T]) Clone() *Set[T] {
	d := make([]T, len(s.data))
	copy(d, s.data)
	return &Set[T]{data: d, eq: s.eq}
}

// Intersect 交集。
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {
	result := NewSet(s.eq)
	s.Range(func(v T) bool {
		if other.Contains(v) {
			result.Add(v)
		}
		return true
	})
	return result
}

// Union 并集。
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := s.Clone()
	other.Range(func(v T) bool {
		result.Add(v)
		return true
	})
	return result
}

// Diff 差集（s 有而 other 无）。
func (s *Set[T]) Diff(other *Set[T]) *Set[T] {
	result := NewSet(s.eq)
	s.Range(func(v T) bool {
		if !other.Contains(v) {
			result.Add(v)
		}
		return true
	})
	return result
}

// ──────────────── 便捷构造 ────────────────

// NewSetFromSlice 从切片创建集合。
func NewSetFromSlice[T comparable](slice []T) *Set[T] {
	s := NewSet(func(a, b T) bool { return a == b })
	for _, v := range slice {
		s.Add(v)
	}
	return s
}
