package tcontainer

// Ring 泛型环形链表。
type Ring[T any] struct {
	data []T
	pos  int
	cap  int
	size int
	full bool
}

// NewRing 创建指定容量的环形链表。
func NewRing[T any](cap int) *Ring[T] {
	if cap <= 0 {
		cap = 10
	}
	return &Ring[T]{data: make([]T, cap), cap: cap}
}

// Put 放入元素。
func (r *Ring[T]) Put(v T) {
	r.data[r.pos] = v
	r.pos = (r.pos + 1) % r.cap
	if r.size < r.cap {
		r.size++
	} else {
		r.full = true
	}
}

// Len 返回当前元素数。
func (r *Ring[T]) Len() int { return r.size }

// Cap 返回容量。
func (r *Ring[T]) Cap() int { return r.cap }

// Slice 按放入顺序返回所有元素。
func (r *Ring[T]) Slice() []T {
	if !r.full {
		result := make([]T, r.size)
		copy(result, r.data[:r.size])
		return result
	}
	result := make([]T, r.cap)
	copy(result, r.data[r.pos:])
	copy(result[r.cap-r.pos:], r.data[:r.pos])
	return result
}

// Clear 清空。
func (r *Ring[T]) Clear() {
	r.pos = 0
	r.size = 0
	r.full = false
}
