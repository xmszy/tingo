package tdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tcfg"
)

// ServiceName 是数据库生命周期服务的稳定名称。
const ServiceName = "database"

// Service 把约定数据库配置注册到应用容器。
type Service struct {
	path string
}

// NewService 创建数据库生命周期服务。
func NewService(path string) *Service { return &Service{path: path} }

func (*Service) Name() string { return ServiceName }
func (*Service) DependsOn() []string {
	return []string{tcfg.ServiceName}
}

// Managers 保存应用作用域的惰性数据库管理器。
type Managers struct {
	applications map[string]*Manager
}

func (m *Managers) manager(name string) (*Manager, bool) {
	manager, ok := m.applications[name]
	return manager, ok
}

// Close 关闭所有已经建立的应用数据库连接。
func (m *Managers) Close() error {
	var result error
	for name, manager := range m.applications {
		if err := manager.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("tdb: closing application %q: %w", name, err))
		}
	}
	return result
}

func (s *Service) Register(app *core.App) error {
	registry, err := core.Resolve[*tcfg.Registry](app.Container())
	if err != nil {
		return err
	}
	cfg, err := ConfigFromTree(registry.Global())
	if err != nil {
		return err
	}
	managerConfig, err := cfg.managerConfig()
	if err != nil {
		return err
	}
	core.BindValue(app.Container(), NewManager(managerConfig))
	applicationManagers := &Managers{applications: make(map[string]*Manager, len(app.ApplicationNames()))}
	for _, name := range app.ApplicationNames() {
		applicationConfig, err := ConfigFromTree(registry.Application(name))
		if err != nil {
			return fmt.Errorf("tdb: application %q: %w", name, err)
		}
		managerConfig, err := applicationConfig.managerConfig()
		if err != nil {
			return fmt.Errorf("tdb: application %q: %w", name, err)
		}
		applicationManagers.applications[name] = NewManager(managerConfig)
	}
	core.BindValue(app.Container(), applicationManagers)
	core.BindValue(app.Container(), cfg)
	return nil
}

func (*Service) Boot(context.Context, *core.App) error { return nil }
func (*Service) Shutdown(context.Context) error        { return nil }

// Connection 返回默认应用的默认或命名连接。
func Connection(names ...string) (*DB, error) {
	return ConnectionFor(context.Background(), core.DefaultApp(), names...)
}

// MustConnection 返回默认应用连接，失败时 panic。
func MustConnection(names ...string) *DB {
	connection, err := Connection(names...)
	if err != nil {
		panic(err)
	}
	return connection
}

// ConnectionFor 从指定应用容器解析全局数据库连接。
func ConnectionFor(ctx context.Context, app *core.App, names ...string) (*DB, error) {
	manager, err := core.Resolve[*Manager](app.Container())
	if err != nil {
		return nil, err
	}
	return manager.Connection(ctx, names...)
}

// ConnectionForApplication 返回指定业务应用作用域的数据库连接。
func ConnectionForApplication(ctx context.Context, app *core.App, application string, names ...string) (*DB, error) {
	managers, err := core.Resolve[*Managers](app.Container())
	if err != nil {
		return nil, err
	}
	manager, ok := managers.manager(application)
	if !ok {
		return ConnectionFor(ctx, app, names...)
	}
	return manager.Connection(ctx, names...)
}

// ConnectionForContext 返回当前请求所属应用作用域的数据库连接。
func ConnectionForContext(ctx *core.Ctx, names ...string) (*DB, error) {
	if ctx == nil {
		return nil, fmt.Errorf("tdb: request context must not be nil")
	}
	return ConnectionForApplication(ctx.Request.Context(), ctx.Framework(), ctx.App(), names...)
}
