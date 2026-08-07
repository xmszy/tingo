package tcontainer

// Pool 泛型对象池。提前创建对象，减少 GC 压力。
type Pool[T any] struct {
	data  []T
	ctor  func() T
	reset func(*T)
}

// NewPool 创建对象池。ctor 为对象构造函数，reset 可选重置函数。
func NewPool[T any](ctor func() T, reset ...func(*T)) *Pool[T] {
	p := &Pool[T]{data: make([]T, 0), ctor: ctor}
	if len(reset) > 0 {
		p.reset = reset[0]
	}
	return p
}

// Get 从池中取出对象，池空则新建。
func (p *Pool[T]) Get() T {
	if len(p.data) == 0 {
		return p.ctor()
	}
	v := p.data[len(p.data)-1]
	p.data = p.data[:len(p.data)-1]
	return v
}

// Put 归还对象到池中。
func (p *Pool[T]) Put(v T) {
	if p.reset != nil {
		p.reset(&v)
	}
	p.data = append(p.data, v)
}

// Len 返回池中当前对象数。
func (p *Pool[T]) Len() int { return len(p.data) }

// Clear 清空池。
func (p *Pool[T]) Clear() { p.data = p.data[:0] }
