package tcfg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tenv"
)

// ServiceName 配置服务在 [core.App] 容器中注册的名称。
const ServiceName = "config"

// ExtensionEnv 配置扩展标识环境变量名。
const ExtensionEnv = "TINGO_CONFIG_EXT"

// DefaultExtension 默认的配置文件扩展名。
const DefaultExtension = "toml"

// globalRegistry 全局配置注册表，Load 后由 NewService 设置。
var globalRegistry = &Registry{apps: map[string]*Config{}}

// Load 读取约定目录中所有应用的配置，返回注册表。
//
// 支持两种约定：
//   - 扁平单应用：<root>/config/app.<ext> → 注册为 "app"
//   - 多应用目录：<root>/config/<app>/app.<ext> → 注册为 <app>
//
// 环境变量 TINGO_CONFIG_EXT 覆盖文件扩展名，默认 toml。
func Load(root string) (*Registry, error) {
	ext := configExtension()
	if !supportedConfigExtension(ext) {
		return nil, fmt.Errorf("tcfg: unsupported config extension %q", ext)
	}
	dir := applicationConfigDir(root)

	registry := &Registry{apps: map[string]*Config{}}

	// 扁平约定：config/app.<ext>
	if tree, loaded, err := ReadFile(filepath.Join(dir, "app."+ext)); err != nil {
		return nil, fmt.Errorf("tcfg: decode config for app: %w", err)
	} else if loaded {
		registry.apps["app"] = NewFromTree(tree)
	}

	// 多应用约定：config/<app>/app.<ext>
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			globalRegistry = registry
			return registry, nil
		}
		return nil, fmt.Errorf("tcfg: read config directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		base := filepath.Join(dir, name)
		tree, loaded, err := ReadFile(filepath.Join(base, "app."+ext))
		if err != nil {
			return nil, fmt.Errorf("tcfg: decode config for app %s: %w", name, err)
		}
		if loaded {
			// 多应用目录优先覆盖扁平约定
			registry.apps[name] = NewFromTree(tree)
		} else if _, exists := registry.apps[name]; !exists {
			registry.apps[name] = NewFromTree(nil)
		}
	}

	globalRegistry = registry
	return registry, nil
}

// Registry 管理多个应用配置的注册表。
type Registry struct {
	apps map[string]*Config
}

// GlobalFor 返回全局应用配置（约定为 "app"）。
func (r *Registry) GlobalFor(name string) *Config {
	if r == nil {
		return NewFromTree(nil)
	}
	if app, ok := r.apps[name]; ok {
		return app
	}
	if app, ok := r.apps["app"]; ok {
		return app
	}
	return NewFromTree(nil)
}

// Global 返回根应用配置。
func (r *Registry) Global() *Config { return r.GlobalFor("app") }

// Application 返回指定应用的配置。同 [Registry.ApplicationFor]。
func (r *Registry) Application(name string) *Config { return r.ApplicationFor(name) }

// ApplicationFor 返回指定应用的配置。
func (r *Registry) ApplicationFor(name string) *Config {
	if r == nil {
		return NewFromTree(nil)
	}
	if app, ok := r.apps[name]; ok {
		return app
	}
	return NewFromTree(nil)
}

// ForContext 返回当前请求所属应用的配置。
func (r *Registry) ForContext(ctx *core.Ctx) *Config {
	if r == nil || ctx == nil {
		return NewFromTree(nil)
	}
	appName := ctx.App()
	if cfg, ok := r.apps[appName]; ok {
		return cfg
	}
	return r.Global()
}

// Service 将配置服务集成到框架生命周期中。
type Service struct {
	*Registry
	root string
}

// NewService 根据约定配置根目录创建配置服务。
func NewService(root string) *Service {
	return &Service{Registry: globalRegistry, root: root}
}

// Name 返回注册名称。
func (s *Service) Name() string { return ServiceName }

// DependsOn 配置服务不依赖其他服务。
func (s *Service) DependsOn() []string { return nil }

// Register 注册配置服务到应用容器。
func (s *Service) Register(app *core.App) error {
	root := s.root
	if root == "" {
		root = "."
	}
	registry, err := Load(root)
	if err != nil {
		return err
	}
	core.BindValue(app.Container(), registry)
	core.BindValue(app.Container(), s)
	return nil
}

// Boot 服务启动时加载配置并周期重载。
func (s *Service) Boot(_ context.Context, app *core.App) error {
	root := s.root
	if root == "" {
		root = "."
	}
	registry, err := Load(root)
	if err != nil {
		return err
	}
	s.Registry = registry
	core.BindValue(app.Container(), registry)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if reloaded, reloadErr := Load(root); reloadErr == nil {
				s.Registry = reloaded
				core.BindValue(app.Container(), reloaded)
			}
		}
	}()
	return nil
}

// Shutdown 关闭时清理。
func (s *Service) Shutdown(_ context.Context) error {
	globalRegistry = &Registry{apps: map[string]*Config{}}
	return nil
}

// GlobalFor 返回全局指定应用的配置。
func GlobalFor(name string) *Config { return globalRegistry.GlobalFor(name) }

// ApplicationFor 返回指定应用的配置。
func ApplicationFor(name string) *Config { return globalRegistry.ApplicationFor(name) }

// ─── 内部函数 ────────────────────────────────────────────────────────

func configExtension() string {
	ext := tenv.Get(ExtensionEnv, "")
	ext = strings.TrimSpace(ext)
	if ext != "" {
		return strings.TrimPrefix(ext, ".")
	}
	return DefaultExtension
}

func applicationConfigDir(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return filepath.Join(root, "config")
}
