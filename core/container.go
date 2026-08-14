package core

import (
	"maps"
	stderrors "errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	terrors "github.com/xmszy/tingo/errors"
)

// Container 是类型安全的服务容器。
//
// 使用 Go 类型本身作为键，解析结果无需类型断言，且错误在编译期即可发现。
//
// 服务默认为单例且懒加载，首次 Resolve 时构造，之后复用。
//
// providers 通过 atomic.Pointer 发布只读快照：绑定只在启动期发生，
// 运行期 Resolve 读取快照免加锁。
type Container struct {
	mu          sync.RWMutex
	providers   atomic.Pointer[map[reflect.Type]*binding]
	named       atomic.Pointer[map[namedKey]*binding]
	parent      *Container
	constructed []any
	closed      bool
}

// namedKey 是命名绑定的复合键：类型 + 名称。
type namedKey struct {
	t    reflect.Type
	name string
}

// binding 是一条服务绑定记录。
type binding struct {
	// factory 构造服务实例。
	factory func(*Container) (any, error)
	// shared 为 true 时缓存实例（单例）。
	shared bool
	// once 保证单例只构造一次。
	once sync.Once
	// instance 是缓存的单例实例。
	instance any
	// err 是构造时发生的错误，与 instance 一同缓存。
	err error
}

// NewContainer 创建一个空容器。
func NewContainer() *Container {
	c := &Container{}
	p := make(map[reflect.Type]*binding, 16)
	c.providers.Store(&p)
	n := make(map[namedKey]*binding, 8)
	c.named.Store(&n)
	return c
}

// NewScope 创建继承当前容器绑定的子作用域。子作用域可覆盖父级服务。
func (c *Container) NewScope() *Container {
	child := &Container{parent: c}
	// 继承父级绑定到独立 map，避免修改子作用域影响父级。
	parent := c.providers.Load()
	p := make(map[reflect.Type]*binding, 8)
	if parent != nil {
		for k, v := range *parent {
			p[k] = v
		}
	}
	child.providers.Store(&p)
	pn := c.named.Load()
	nn := make(map[namedKey]*binding, 4)
	if pn != nil {
		maps.Copy(nn, *pn)
	}
	child.named.Store(&nn)
	return child
}

// Default 返回默认 App 所属的容器。
func Default() *Container { return DefaultApp().Container() }

/* ------------------------------------------------------------------ */
/* 绑定                                                                */
/* ------------------------------------------------------------------ */

// Bind 将类型 T 绑定到一个懒加载单例工厂。
//
//	core.Bind(c, func(c *core.Container) (*Repo, error) {
//	    return NewRepo(core.MustResolve[*DB](c)), nil
//	})
func Bind[T any](c *Container, factory func(*Container) (T, error)) {
	bindType[T](c, func(c *Container) (any, error) { return factory(c) }, true)
}

// BindTransient 绑定为瞬时服务，每次 Resolve 都重新构造。
func BindTransient[T any](c *Container, factory func(*Container) (T, error)) {
	bindType[T](c, func(c *Container) (any, error) { return factory(c) }, false)
}

// BindValue 直接绑定一个已构造好的实例。
func BindValue[T any](c *Container, v T) {
	bindType[T](c, func(*Container) (any, error) { return v, nil }, true)
}

// bindType 是绑定的内部实现。
func bindType[T any](c *Container, f func(*Container) (any, error), shared bool) {
	t := typeKey[T]()
	c.mu.Lock()
	defer c.mu.Unlock()
	// 拷贝写时快照（COW）：避免影响已发布的旧快照读者。
	cur := c.providers.Load()
	p := make(map[reflect.Type]*binding, len(*cur)+1)
	for k, v := range *cur {
		p[k] = v
	}
	p[t] = &binding{factory: f, shared: shared}
	c.providers.Store(&p)
}

// typeKey 返回类型 T 的反射类型，作为容器键。
func typeKey[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}

/* ------------------------------------------------------------------ */
/* 接口绑定                                                            */
/* ------------------------------------------------------------------ */

// BindInterface 将接口类型 I 绑定到返回 T 的工厂。调用方通常用具体类型构造并断言实现 I。
//
//	core.BindInterface[Repository, *MySQLRepo](c, func(c *core.Container) (*MySQLRepo, error) {
//	    return &MySQLRepo{}, nil
//	})
//
// 之后即可 core.Resolve[Repository](c) 取到 *MySQLRepo（按接口 I 作为键）。
// 工厂返回值若不实现 I 会返回错误。
func BindInterface[I any, T any](c *Container, factory func(*Container) (T, error)) {
	bindType[I](c, func(c *Container) (any, error) {
		v, err := factory(c)
		if err != nil {
			return nil, err
		}
		if _, ok := any(v).(I); !ok {
			return nil, terrors.ErrInternal.WithMessagef(
				"tingo: %T does not implement %s", v, reflect.TypeFor[I]())
		}
		return v, nil
	}, true)
}

/* ------------------------------------------------------------------ */
/* 命名绑定                                                            */
/* ------------------------------------------------------------------ */

// BindNamed 按名称绑定类型 T 的懒加载单例工厂，与无名称绑定相互独立。
//
//	core.BindNamed[*Cache](c, "redis", func(c *core.Container) (*Cache, error) { ... })
//	v := core.MustResolveNamed[*Cache](c, "redis")
func BindNamed[T any](c *Container, name string, factory func(*Container) (T, error)) {
	bindNamed[T](c, name, func(c *Container) (any, error) { return factory(c) }, true)
}

// BindNamedTransient 按名称绑定为瞬时服务。
func BindNamedTransient[T any](c *Container, name string, factory func(*Container) (T, error)) {
	bindNamed[T](c, name, func(c *Container) (any, error) { return factory(c) }, false)
}

// BindNamedValue 按名称直接绑定一个已构造的实例。
func BindNamedValue[T any](c *Container, name string, v T) {
	bindNamed[T](c, name, func(*Container) (any, error) { return v, nil }, true)
}

// bindNamed 是命名绑定的内部实现（COW 语义与 bindType 一致）。
func bindNamed[T any](c *Container, name string, f func(*Container) (any, error), shared bool) {
	t := typeKey[T]()
	k := namedKey{t: t, name: name}
	c.mu.Lock()
	defer c.mu.Unlock()
	cur := c.named.Load()
	p := make(map[namedKey]*binding, len(*cur)+1)
	for k, v := range *cur {
		p[k] = v
	}
	p[k] = &binding{factory: f, shared: shared}
	c.named.Store(&p)
}

// ResolveNamed 按名称解析类型 T 的实例。
func ResolveNamed[T any](c *Container, name string) (T, error) {
	var zero T
	t := typeKey[T]()
	k := namedKey{t: t, name: name}

	p := c.named.Load()
	var b *binding
	var ok bool
	if p != nil {
		b, ok = (*p)[k]
	}
	if !ok {
		// 回退：命名未命中时尝试无名称绑定（常见默认实现）。
		return Resolve[T](c)
	}

	if !b.shared {
		v, err := b.factory(c)
		if err != nil {
			return zero, err
		}
		tv, ok := v.(T)
		if !ok {
			return zero, terrors.ErrInternal.WithMessagef("tingo: named service %s %s factory returned %T", name, t, v)
		}
		return tv, nil
	}

	b.once.Do(func() {
		b.instance, b.err = b.factory(c)
		if b.err == nil {
			c.mu.Lock()
			c.constructed = append(c.constructed, b.instance)
			c.mu.Unlock()
		}
	})
	if b.err != nil {
		return zero, b.err
	}
	tv, ok := b.instance.(T)
	if !ok {
		return zero, terrors.ErrInternal.WithMessagef("tingo: named service %s %s factory returned %T", name, t, b.instance)
	}
	return tv, nil
}

// MustResolveNamed 按名称解析，失败时 panic。
func MustResolveNamed[T any](c *Container, name string) T {
	v, err := ResolveNamed[T](c, name)
	if err != nil {
		panic(err)
	}
	return v
}

// HasNamed 判断命名服务是否已绑定。
func HasNamed[T any](c *Container, name string) bool {
	t := typeKey[T]()
	k := namedKey{t: t, name: name}
	p := c.named.Load()
	if p == nil {
		return false
	}
	_, ok := (*p)[k]
	return ok
}

/* ------------------------------------------------------------------ */
/* 解析                                                                */
/* ------------------------------------------------------------------ */

// Resolve 解析类型 T 的实例。
func Resolve[T any](c *Container) (T, error) {
	var zero T
	t := typeKey[T]()

	p := c.providers.Load()
	var b *binding
	var ok bool
	if p != nil {
		b, ok = (*p)[t]
	}
	parent := c.parent
	if !ok {
		if parent != nil {
			return Resolve[T](parent)
		}
		return zero, terrors.ErrInternal.WithMessagef("tingo: service %s is not bound", t)
	}

	if !b.shared {
		v, err := b.factory(c)
		if err != nil {
			return zero, err
		}
		tv, ok := v.(T)
		if !ok {
			return zero, terrors.ErrInternal.WithMessagef(
				"tingo: service %s factory returned %T", t, v)
		}
		return tv, nil
	}

	b.once.Do(func() {
		b.instance, b.err = b.factory(c)
		if b.err == nil {
			c.mu.Lock()
			c.constructed = append(c.constructed, b.instance)
			c.mu.Unlock()
		}
	})
	if b.err != nil {
		return zero, b.err
	}
	tv, ok := b.instance.(T)
	if !ok {
		return zero, terrors.ErrInternal.WithMessagef(
			"tingo: service %s factory returned %T", t, b.instance)
	}
	return tv, nil
}

// ResolveUntyped 按反射类型从容器解析实例。
//
// 它复用与 Resolve[T] 完全相同的绑定查找与单例缓存逻辑，区别在于键由
// 调用方以 reflect.Type 形式提供，从而支持在反射驱动的装配（如控制器字段注入）
// 中解析任意类型，无需调用方提供编译期泛型参数。
func (c *Container) ResolveUntyped(t reflect.Type) (any, error) {
	p := c.providers.Load()
	var b *binding
	var ok bool
	if p != nil {
		b, ok = (*p)[t]
	}
	parent := c.parent
	if !ok {
		if parent != nil {
			return parent.ResolveUntyped(t)
		}
		return nil, terrors.ErrInternal.WithMessagef("tingo: service %s is not bound", t)
	}

	if !b.shared {
		v, err := b.factory(c)
		if err != nil {
			return nil, err
		}
		return v, nil
	}

	b.once.Do(func() {
		b.instance, b.err = b.factory(c)
		if b.err == nil {
			c.mu.Lock()
			c.constructed = append(c.constructed, b.instance)
			c.mu.Unlock()
		}
	})
	if b.err != nil {
		return nil, b.err
	}
	return b.instance, nil
}

// MustResolve 解析类型 T，失败时 panic。
// 适用于启动期装配，此时失败应立即终止进程。
func MustResolve[T any](c *Container) T {
	v, err := Resolve[T](c)
	if err != nil {
		panic(err)
	}
	return v
}

// Has 判断类型 T 是否已绑定。
func Has[T any](c *Container) bool {
	t := typeKey[T]()
	p := c.providers.Load()
	if p == nil {
		return false
	}
	_, ok := (*p)[t]
	return ok
}

// Close 按单例构造的逆序释放实现 Close() error 的服务。
func (c *Container) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	instances := append([]any(nil), c.constructed...)
	c.constructed = nil
	c.mu.Unlock()

	var result error
	for i := len(instances) - 1; i >= 0; i-- {
		if closer, ok := instances[i].(interface{ Close() error }); ok {
			result = stderrors.Join(result, closer.Close())
		}
	}
	return result
}

// Reset 清空容器。仅供测试使用。
func (c *Container) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := make(map[reflect.Type]*binding, 16)
	c.providers.Store(&p)
	n := make(map[namedKey]*binding, 8)
	c.named.Store(&n)
	c.constructed = nil
	c.closed = false
}

// String 返回容器中已绑定服务的概览，便于调试。
func (c *Container) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p := c.providers.Load()
	n := 0
	if p != nil {
		n = len(*p)
	}
	nn := c.named.Load()
	if nn != nil {
		n += len(*nn)
	}
	return fmt.Sprintf("Container(%d services)", n)
}
