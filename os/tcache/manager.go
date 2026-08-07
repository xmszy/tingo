package tcache

import (
	"context"
	"fmt"
	"time"

	"github.com/xmszy/tingo/core"
)

// Driver 是缓存后端的稳定协议。远程缓存实现应通过 error 返回网络或序列化错误。
type Driver interface {
	Get(context.Context, string) (any, bool, error)
	Set(context.Context, string, any, time.Duration) error
	Delete(context.Context, string) error
	Clear(context.Context) error
	Close() error
}

// MemoryDriver 将现有内存 Cache 适配为可插拔驱动。
type MemoryDriver struct{ cache *Cache }

func NewMemoryDriver(options Options) *MemoryDriver {
	return &MemoryDriver{cache: New(options)}
}

func (d *MemoryDriver) Get(_ context.Context, key string) (any, bool, error) {
	value, ok := d.cache.Get(key)
	return value, ok, nil
}
func (d *MemoryDriver) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	d.cache.Set(key, value, ttl)
	return nil
}
func (d *MemoryDriver) Delete(_ context.Context, key string) error {
	d.cache.Delete(key)
	return nil
}
func (d *MemoryDriver) Clear(context.Context) error { d.cache.Clear(); return nil }
func (d *MemoryDriver) Close() error                { return d.cache.Close() }
func (d *MemoryDriver) Cache() *Cache               { return d.cache }

// ConnectionConfig 描述一个命名缓存连接。
type ConnectionConfig struct {
	Driver string
	Memory Options
}

// ManagerConfig 配置默认连接、命名连接和可扩展工厂注册表。
type ManagerConfig struct {
	Default     string
	Connections map[string]ConnectionConfig
	Registry    *core.DriverRegistry[ConnectionConfig, Driver]
}

// Manager 按名称惰性创建并缓存 Driver，创建过程由 core.DriverManager 保证单航次。
type Manager struct {
	manager *core.DriverManager[ConnectionConfig, Driver]
}

func NewManager(config ManagerConfig) (*Manager, error) {
	if config.Default == "" {
		config.Default = "default"
	}
	if config.Connections == nil {
		config.Connections = map[string]ConnectionConfig{
			config.Default: {Driver: "memory"},
		}
	}
	registry := config.Registry
	if registry == nil {
		registry = core.NewDriverRegistry[ConnectionConfig, Driver]()
		if err := registry.Register("memory", func(_ context.Context, cfg ConnectionConfig) (Driver, error) {
			return NewMemoryDriver(cfg.Memory), nil
		}); err != nil {
			return nil, err
		}
	}
	inner := core.NewDriverManager(core.DriverManagerConfig[ConnectionConfig, Driver]{
		Registry: registry,
		Default:  func() string { return config.Default },
		Resolve: func(name string) (ConnectionConfig, error) {
			connection, ok := config.Connections[name]
			if !ok {
				return ConnectionConfig{}, fmt.Errorf("tcache: connection %q is not configured", name)
			}
			return connection, nil
		},
		Type: func(connection ConnectionConfig) string { return connection.Driver },
	})
	return &Manager{manager: inner}, nil
}

func (m *Manager) Connection(ctx context.Context, names ...string) (Driver, error) {
	return m.manager.Connection(ctx, names...)
}
func (m *Manager) Forget(ctx context.Context, names ...string) error {
	return m.manager.Forget(ctx, names...)
}
func (m *Manager) CloseContext(ctx context.Context) error { return m.manager.Close(ctx) }
func (m *Manager) Close() error                           { return m.CloseContext(context.Background()) }

// Service 将缓存 Manager 纳入 App 生命周期。
type Service struct {
	manager *Manager
}

func NewService(config ManagerConfig) (*Service, error) {
	manager, err := NewManager(config)
	if err != nil {
		return nil, err
	}
	return &Service{manager: manager}, nil
}

func (*Service) Name() string        { return "cache" }
func (*Service) DependsOn() []string { return nil }
func (s *Service) Register(app *core.App) error {
	core.BindValue(app.Container(), s.manager)
	return nil
}
func (s *Service) Boot(ctx context.Context, _ *core.App) error {
	_, err := s.manager.Connection(ctx)
	return err
}
func (s *Service) Shutdown(ctx context.Context) error { return s.manager.CloseContext(ctx) }
