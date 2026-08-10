package tcontainer

import "github.com/xmszy/tingo/os/tmutex"

// Queue 泛型队列。FIFO，线程不安全。
// 可指定容量 limit（>0 时达到容量 Push 会丢弃并返回 false）。
type Queue[T any] struct {
	data []T
	cap  int // 0 表示不限制
}

// NewQueue 创建队列（不限容量）。
func NewQueue[T any]() *Queue[T] { return &Queue[T]{data: make([]T, 0)} }

// NewLimitQueue 创建有容量限制的队列。
func NewLimitQueue[T any](limit int) *Queue[T] {
	return &Queue[T]{data: make([]T, 0, limit), cap: limit}
}

// Capacity 返回容量（0 表示不限制）。
func (q *Queue[T]) Capacity() int { return q.cap }

// Push 入队。成功返回 true；容量已满返回 false。
func (q *Queue[T]) Push(v T) bool {
	if q.cap > 0 && len(q.data) >= q.cap {
		return false
	}
	q.data = append(q.data, v)
	return true
}

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

// IsEmpty 是否为空。
func (q *Queue[T]) IsEmpty() bool { return len(q.data) == 0 }

// Slice 返回底层切片（调用方不应修改）。
func (q *Queue[T]) Slice() []T { return q.data }

// Clear 清空。
func (q *Queue[T]) Clear() { q.data = q.data[:0] }

// ──────────────── 线程安全队列 ────────────────

// SafeQueue 线程安全队列。内嵌加锁的 Queue，所有操作均加锁。
type SafeQueue[T any] struct {
	mu  tmutex.RWMutex
	q   *Queue[T]
}

// NewSafeQueue 创建并发安全队列。
func NewSafeQueue[T any]() *SafeQueue[T] {
	return &SafeQueue[T]{q: NewQueue[T]()}
}

// NewSafeLimitQueue 创建并发安全且有容量限制的队列。
func NewSafeLimitQueue[T any](limit int) *SafeQueue[T] {
	return &SafeQueue[T]{q: NewLimitQueue[T](limit)}
}

// Push 入队（加锁）。
func (q *SafeQueue[T]) Push(v T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.q.Push(v)
}

// Pop 出队（加锁）。
func (q *SafeQueue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.q.Pop()
}

// Peek 查看队首（加读锁）。
func (q *SafeQueue[T]) Peek() (T, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.q.Peek()
}

// Len 返回长度（加读锁）。
func (q *SafeQueue[T]) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.q.Len()
}

// IsEmpty 是否为空（加读锁）。
func (q *SafeQueue[T]) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.q.IsEmpty()
}

// Clear 清空（加锁）。
func (q *SafeQueue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.q.Clear()
}
