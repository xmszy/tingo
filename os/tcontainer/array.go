package tcontainer

import "sort"

// Array 泛型动态数组。
type Array[T any] struct {
	data   []T
	sorted bool
}

// NewArray 创建数组。
func NewArray[T any](elems ...T) *Array[T] {
	if len(elems) > 0 {
		d := make([]T, len(elems))
		copy(d, elems)
		return &Array[T]{data: d}
	}
	return &Array[T]{data: make([]T, 0)}
}

// Get 获取指定索引值。索引越界时返回零值，不 panic。
func (a *Array[T]) Get(index int) T {
	if index < 0 || index >= len(a.data) {
		var zero T
		return zero
	}
	return a.data[index]
}

// Set 设置指定索引值。索引越界时直接返回，不 panic。
func (a *Array[T]) Set(index int, value T) {
	if index < 0 || index >= len(a.data) {
		return
	}
	a.data[index] = value
	a.sorted = false
}

// Append 追加元素。
func (a *Array[T]) Append(values ...T) { a.data = append(a.data, values...); a.sorted = false }

// Len 返回长度。
func (a *Array[T]) Len() int { return len(a.data) }

// Slice 返回底层切片。
func (a *Array[T]) Slice() []T { return a.data }

// Range 遍历。
func (a *Array[T]) Range(fn func(index int, value T) bool) {
	for i, v := range a.data {
		if !fn(i, v) {
			break
		}
	}
}

// Remove 按索引移除。索引越界时返回零值，不 panic。
func (a *Array[T]) Remove(index int) T {
	if index < 0 || index >= len(a.data) {
		var zero T
		return zero
	}
	v := a.data[index]
	a.data = append(a.data[:index], a.data[index+1:]...)
	return v
}

// Pop 弹出末尾元素。数组为空时返回零值，不 panic。
func (a *Array[T]) Pop() T {
	if len(a.data) == 0 {
		var zero T
		return zero
	}
	v := a.data[len(a.data)-1]
	a.data = a.data[:len(a.data)-1]
	return v
}

// PopFront 弹出前端元素。数组为空时返回零值，不 panic。
func (a *Array[T]) PopFront() T { return a.Remove(0) }

// PushFront 在前端插入。
func (a *Array[T]) PushFront(v T) {
	a.data = append([]T{v}, a.data...)
	a.sorted = false
}

// Insert 在指定位置插入。
func (a *Array[T]) Insert(index int, values ...T) {
	a.data = append(a.data[:index], append(values, a.data[index:]...)...)
	a.sorted = false
}

// Clear 清空。
func (a *Array[T]) Clear() { a.data = a.data[:0] }

// Contains 判断是否包含。
func (a *Array[T]) Contains(value T, eq func(a, b T) bool) bool {
	for _, v := range a.data {
		if eq(v, value) {
			return true
		}
	}
	return false
}

// Sort 排序（要求 T 实现 sort.Interface）。
func (a *Array[T]) Sort(less func(i, j int) bool) {
	if a.sorted {
		return
	}
	sort.Slice(a.data, less)
	a.sorted = true
}

// SortFunc 使用 sort.Interface 排序。
func (a *Array[T]) SortFunc(data sort.Interface) { sort.Sort(data); a.sorted = true }

// Unique 去重。
func (a *Array[T]) Unique(eq func(a, b T) bool) *Array[T] {
	result := &Array[T]{data: make([]T, 0, len(a.data))}
	seen := NewSet(eq)
	for _, v := range a.data {
		if seen.AddIfNotExist(v) {
			result.Append(v)
		}
	}
	return result
}

// Clone 浅拷贝。
func (a *Array[T]) Clone() *Array[T] {
	d := make([]T, len(a.data))
	copy(d, a.data)
	return &Array[T]{data: d, sorted: a.sorted}
}

// Search 二分查找（要求已排序）。
func (a *Array[T]) Search(target T, cmp func(T, T) int) int {
	return sort.Search(a.Len(), func(i int) bool { return cmp(a.data[i], target) >= 0 })
}
