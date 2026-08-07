// driver 提供类型安全的驱动注册和实例管理。
package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DriverFactory 是创建驱动的工厂函数。
type DriverFactory[Config, Driver any] func(context.Context, Config) (Driver, error)

// DriverRegistry 是泛型驱动注册表，按名称管理工厂函数。
type DriverRegistry[Config, Driver any] struct {
	mu        sync.RWMutex
	factories map[string]DriverFactory[Config, Driver]
}

// NewDriverRegistry 创建一个新的驱动注册表。
func NewDriverRegistry[Config, Driver any]() *DriverRegistry[Config, Driver] {
	return &DriverRegistry[Config, Driver]{factories: make(map[string]DriverFactory[Config, Driver])}
}

// Register 注册一个驱动工厂。名称不区分大小写。
func (r *DriverRegistry[Config, Driver]) Register(name string, factory DriverFactory[Config, Driver]) error {
	name = normalizeDriverName(name)
	if name == "" {
		return errors.New("tingo driver: factory name must not be empty")
	}
	if factory == nil {
		return fmt.Errorf("tingo driver: factory %q must not be nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("tingo driver: factory %q already registered", name)
	}
	r.factories[name] = factory
	return nil
}

// Factory 按名称获取驱动工厂。
func (r *DriverRegistry[Config, Driver]) Factory(name string) (DriverFactory[Config, Driver], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[normalizeDriverName(name)]
	return factory, ok
}

// Names 返回所有已注册的驱动名称，排序后返回。
func (r *DriverRegistry[Config, Driver]) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DriverManagerConfig 是 DriverManager 的配置项。
type DriverManagerConfig[Config, Driver any] struct {
	Registry *DriverRegistry[Config, Driver]
	Default  func() string
	Resolve  func(name string) (Config, error)
	Type     func(Config) string
	Close    func(context.Context, Driver) error
}

type driverCreation[Driver any] struct {
	done  chan struct{}
	value Driver
	err   error
}

// DriverManager 按连接名惰性创建并缓存驱动实例。
type DriverManager[Config, Driver any] struct {
	cfg DriverManagerConfig[Config, Driver]

	mu        sync.Mutex
	instances map[string]Driver
	creating  map[string]*driverCreation[Driver]
}

// NewDriverManager 创建一个驱动管理器。
func NewDriverManager[Config, Driver any](cfg DriverManagerConfig[Config, Driver]) *DriverManager[Config, Driver] {
	if cfg.Registry == nil || cfg.Resolve == nil || cfg.Type == nil {
		panic("tingo driver: Registry, Resolve and Type are required")
	}
	return &DriverManager[Config, Driver]{
		cfg:       cfg,
		instances: make(map[string]Driver),
		creating:  make(map[string]*driverCreation[Driver]),
	}
}

// Connection 获取指定连接名的驱动实例。未指定则使用默认连接。
func (m *DriverManager[Config, Driver]) Connection(ctx context.Context, names ...string) (Driver, error) {
	var zero Driver
	name := ""
	if len(names) > 0 {
		name = names[0]
	} else if m.cfg.Default != nil {
		name = m.cfg.Default()
	}
	name = normalizeDriverName(name)
	if name == "" {
		return zero, errors.New("tingo driver: default connection name is empty")
	}

	m.mu.Lock()
	if instance, ok := m.instances[name]; ok {
		m.mu.Unlock()
		return instance, nil
	}
	if pending, ok := m.creating[name]; ok {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-pending.done:
			return pending.value, pending.err
		}
	}
	pending := &driverCreation[Driver]{done: make(chan struct{})}
	m.creating[name] = pending
	m.mu.Unlock()

	config, err := m.cfg.Resolve(name)
	if err == nil {
		driverType := normalizeDriverName(m.cfg.Type(config))
		factory, found := m.cfg.Registry.Factory(driverType)
		if !found {
			err = fmt.Errorf("tingo driver: type %q for connection %q is not registered", driverType, name)
		} else {
			pending.value, err = factory(ctx, config)
		}
	}
	pending.err = err

	m.mu.Lock()
	delete(m.creating, name)
	if err == nil {
		m.instances[name] = pending.value
	}
	close(pending.done)
	m.mu.Unlock()
	return pending.value, pending.err
}

// Forget 释放并关闭指定连接名的驱动实例。
func (m *DriverManager[Config, Driver]) Forget(ctx context.Context, names ...string) error {
	if len(names) == 0 && m.cfg.Default != nil {
		names = []string{m.cfg.Default()}
	}
	var result error
	for _, rawName := range names {
		name := normalizeDriverName(rawName)
		m.mu.Lock()
		instance, ok := m.instances[name]
		if ok {
			delete(m.instances, name)
		}
		m.mu.Unlock()
		if ok {
			result = errors.Join(result, m.closeDriver(ctx, instance))
		}
	}
	return result
}

// Close 关闭所有已创建的驱动实例（逆序关闭）。
func (m *DriverManager[Config, Driver]) Close(ctx context.Context) error {
	m.mu.Lock()
	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	m.mu.Unlock()
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return m.Forget(ctx, names...)
}

func (m *DriverManager[Config, Driver]) closeDriver(ctx context.Context, instance Driver) error {
	if m.cfg.Close != nil {
		return m.cfg.Close(ctx, instance)
	}
	if closer, ok := any(instance).(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func normalizeDriverName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
