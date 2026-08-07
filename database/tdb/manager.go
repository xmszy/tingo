package tdb

import (
	"context"
	"fmt"

	"github.com/xmszy/tingo/core"
)

// ManagerConfig 描述命名数据库连接。Default 必须指向 Connections 中的一项。
type ManagerConfig struct {
	Default     string
	Connections map[string]Config
}

// Manager 按连接名惰性创建并缓存 DB。
type Manager struct {
	manager *core.DriverManager[Config, *DB]
}

func NewManager(config ManagerConfig) *Manager {
	registry := core.NewDriverRegistry[Config, *DB]()
	if err := registry.Register("database", func(_ context.Context, connection Config) (*DB, error) {
		return Open(connection)
	}); err != nil {
		panic(err)
	}
	manager := core.NewDriverManager(core.DriverManagerConfig[Config, *DB]{
		Registry: registry,
		Default:  func() string { return config.Default },
		Resolve: func(name string) (Config, error) {
			connection, ok := config.Connections[name]
			if !ok {
				return Config{}, fmt.Errorf("tdb: connection %q is not configured", name)
			}
			return connection, nil
		},
		Type: func(Config) string { return "database" },
	})
	return &Manager{manager: manager}
}

func (m *Manager) Connection(ctx context.Context, names ...string) (*DB, error) {
	return m.manager.Connection(ctx, names...)
}

func (m *Manager) MustConnection(ctx context.Context, names ...string) *DB {
	connection, err := m.Connection(ctx, names...)
	if err != nil {
		panic(err)
	}
	return connection
}

func (m *Manager) Forget(ctx context.Context, names ...string) error {
	return m.manager.Forget(ctx, names...)
}

func (m *Manager) Close() error { return m.manager.Close(context.Background()) }
