package tcontainer

// Queue 泛型队列。FIFO，线程不安全。
type Queue[T any] struct {
	data []T
}

// NewQueue 创建队列。
func NewQueue[T any]() *Queue[T] { return &Queue[T]{data: make([]T, 0)} }

// Push 入队。
func (q *Queue[T]) Push(v T) { q.data = append(q.data, v) }

// Pop 出队。
func (q *Queue[T]) Pop() (T, bool) {
	if len(q.data) == 0 {
		var zero T
		return zero, false
	}
	v := q.data[0]
	q.data = q.data[1:]
	return v, true
}

// Peek 查看队首但不移除。
func (q *Queue[T]) Peek() (T, bool) {
	if len(q.data) == 0 {
		var zero T
		return zero, false
	}
	return q.data[0], true
}

// Len 返回长度。
func (q *Queue[T]) Len() int { return len(q.data) }

// Slice 返回底层切片。
func (q *Queue[T]) Slice() []T { return q.data }

// Clear 清空。
func (q *Queue[T]) Clear() { q.data = q.data[:0] }

// ──────────────── 线程安全队列 ────────────────

// SafeQueue 线程安全队列。
type SafeQueue[T any] struct {
	*Queue[T]
	_ [0]int // 占位
}
