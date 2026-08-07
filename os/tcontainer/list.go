package tcontainer

// List 泛型双向链表。
type List[T any] struct {
	head, tail *listNode[T]
	size       int
}

type listNode[T any] struct {
	value      T
	prev, next *listNode[T]
}

// NewList 创建链表。
func NewList[T any]() *List[T] { return &List[T]{} }

// Len 返回链表长度。
func (l *List[T]) Len() int { return l.size }

// PushBack 在尾部追加。
func (l *List[T]) PushBack(value T) {
	n := &listNode[T]{value: value}
	if l.tail == nil {
		l.head = n
		l.tail = n
	} else {
		n.prev = l.tail
		l.tail.next = n
		l.tail = n
	}
	l.size++
}

// PushFront 在头部插入。
func (l *List[T]) PushFront(value T) {
	n := &listNode[T]{value: value}
	if l.head == nil {
		l.head = n
		l.tail = n
	} else {
		n.next = l.head
		l.head.prev = n
		l.head = n
	}
	l.size++
}

// PopBack 弹出尾部。
func (l *List[T]) PopBack() (T, bool) {
	if l.tail == nil {
		var zero T
		return zero, false
	}
	v := l.tail.value
	if l.head == l.tail {
		l.head = nil
		l.tail = nil
	} else {
		l.tail = l.tail.prev
		l.tail.next = nil
	}
	l.size--
	return v, true
}

// PopFront 弹出头部。
func (l *List[T]) PopFront() (T, bool) {
	if l.head == nil {
		var zero T
		return zero, false
	}
	v := l.head.value
	if l.head == l.tail {
		l.head = nil
		l.tail = nil
	} else {
		l.head = l.head.next
		l.head.prev = nil
	}
	l.size--
	return v, true
}

// Front 返回头部值。
func (l *List[T]) Front() (T, bool) {
	if l.head == nil {
		var zero T
		return zero, false
	}
	return l.head.value, true
}

// Back 返回尾部值。
func (l *List[T]) Back() (T, bool) {
	if l.tail == nil {
		var zero T
		return zero, false
	}
	return l.tail.value, true
}

// Range 正向遍历。
func (l *List[T]) Range(fn func(value T) bool) {
	for n := l.head; n != nil; n = n.next {
		if !fn(n.value) {
			break
		}
	}
}

// Reverse 反向遍历。
func (l *List[T]) Reverse(fn func(value T) bool) {
	for n := l.tail; n != nil; n = n.prev {
		if !fn(n.value) {
			break
		}
	}
}

// Slice 转为切片。
func (l *List[T]) Slice() []T {
	result := make([]T, 0, l.size)
	for n := l.head; n != nil; n = n.next {
		result = append(result, n.value)
	}
	return result
}

// Clear 清空。
func (l *List[T]) Clear() {
	l.head = nil
	l.tail = nil
	l.size = 0
}
