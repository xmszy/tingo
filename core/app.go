// Package core 提供 tingo 的运行时内核。
//
// 设计原则：
//  1. 零成本抽象 —— 类型定义零拷贝，指针转换是编译期 no-op。
//  2. 零额外分配 —— 不在请求路径上包装、不逃逸、不反射。
//  3. 注册期展开 —— 多应用/中间件在启动时全部展开进路由树。
package core

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

/* ------------------------------------------------------------------ */
/* 生命周期状态                                                           */
/* ------------------------------------------------------------------ */

// State 描述框架 App 的生命周期阶段。
type State uint8

const (
	StateCreated State = iota
	StateRegistering
	StateRegistered
	StateBooting
	StateBooted
	StateRunning
	StateStopping
	StateStopped
	StateFailed
)

func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateRegistering:
		return "registering"
	case StateRegistered:
		return "registered"
	case StateBooting:
		return "booting"
	case StateBooted:
		return "booted"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

/* ------------------------------------------------------------------ */
/* Service 接口                                                         */
/* ------------------------------------------------------------------ */

// Service 是框架能力的生命周期单元。
// Register 仅声明容器绑定；Boot 才建立连接、启动 goroutine 或挂载资源。
type Service interface {
	Name() string
	DependsOn() []string
	Register(*App) error
	Boot(context.Context, *App) error
	Shutdown(context.Context) error
}

/* ------------------------------------------------------------------ */
/* App 结构体                                                           */
/* ------------------------------------------------------------------ */

// App 是实例级框架内核。每个 App 拥有独立容器和服务实例。
type App struct {
	opMu sync.Mutex
	mu   sync.RWMutex

	state        State
	container    *Container
	services     map[string]Service
	serviceOrder []string
	booted       []Service
	shutdownDone bool

	applications map[string]*appEntry
	routeMeta    map[string]*RouteMeta
	// routeMetaSnap 是 routeMeta 的只读快照（注册期写入、运行时只读），
	// 用 atomic.Pointer 发布，使 routeMetaOf 免每请求加锁。
	routeMetaSnap atomic.Pointer[map[string]*RouteMeta]
}

// NewApp 创建一个新的 App 实例。
func NewApp() *App {
	return &App{
		state:        StateCreated,
		container:    NewContainer(),
		services:     make(map[string]Service),
		applications: make(map[string]*appEntry, 8),
		routeMeta:    make(map[string]*RouteMeta, 64),
	}
}

var (
	defaultAppMu sync.RWMutex
	defaultApp   = NewApp()
)

// DefaultApp 返回兼容旧门面的进程级 App。新项目应显式持有 NewApp 的返回值。
func DefaultApp() *App {
	defaultAppMu.RLock()
	a := defaultApp
	defaultAppMu.RUnlock()
	return a
}

/* ------------------------------------------------------------------ */
/* App 基础方法                                                          */
/* ------------------------------------------------------------------ */

// Container 返回 App 的容器。
func (a *App) Container() *Container { return a.container }

// State 返回当前生命周期阶段。
func (a *App) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *App) setState(state State) {
	a.mu.Lock()
	a.state = state
	a.mu.Unlock()
}

// HasService 判断指定名称的生命周期服务是否已注册。
func (a *App) HasService(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.services[name]
	return ok
}

/* ------------------------------------------------------------------ */
/* Router / Application 接口                                           */
/* ------------------------------------------------------------------ */

// Router 定义了路由注册接口。
type Router interface {
	Use(handler ...Handler) Router
	Group(relativePath string, fn func(Router), mws ...Handler) Router
	GET(path string, handler any) Router
	POST(path string, handler any) Router
	PUT(path string, handler any) Router
	DELETE(path string, handler any) Router
	PATCH(path string, handler any) Router
	HEAD(path string, handler any) Router
	OPTIONS(path string, handler any) Router
	Any(path string, handler any) Router
	Controller(path string, controller any) Router
	Resource(path string, controller any) Router
}

// Application 代表一个可挂载的应用。
type Application interface {
	Routes(r Router)
}

// AppConfig 是每个应用的配置。
type AppConfig struct {
	Path       string
	Prefix     string
	Domain     string // 域名绑定（可选）
	Disabled   bool   // 是否禁用
	Priority   int    // 优先级（越大越优先注册）
	Default    bool   // 是否为默认应用
	Middleware []Handler
}

// AppConfigurer 可选接口：应用实现它来提供配置。
type AppConfigurer interface {
	Config() AppConfig
}

// AppMiddlewarer 可选接口：应用实现它来声明专属中间件。
type AppMiddlewarer interface {
	Middlewares() []Handler
}

// AppBooter 可选接口：应用实现它可在启动时执行初始化。
type AppBooter interface {
	Boot() error
}

// AppInfo 包含已注册应用的信息。
type AppInfo struct {
	App    Application
	Config *AppConfig
	Name   string
}

// AppConfigProvider 由上层（如 frame/t）注入，用于根据框架配置动态解析每个应用的
// AppConfig（对标 ThinkPHP config/app.php 的 default_app/app_map/domain_bind/deny_app）。
// 接收应用名与注册期的基础配置，返回最终生效的配置。在引擎 Boot 阶段（配置已加载）调用。
// 未注入则直接使用注册期设定的 AppConfig。
var AppConfigProvider func(name string, base AppConfig) AppConfig

// appEntry 是已注册应用的内部描述。
type appEntry struct {
	app        Application
	config     *AppConfig
	normalized string // 规范化后的路由前缀
}

// normalizePath 规范化路径前缀。
func normalizePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	return path.Clean(raw)
}

/* ------------------------------------------------------------------ */
/* 应用注册                                                              */
/* ------------------------------------------------------------------ */

// RegisterApplication 登记一个应用（仅存储元数据，不注册路由）。
func (a *App) RegisterApplication(name string, app Application) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.applications[name]; exists && a.applications[name] != nil {
		panic(fmt.Errorf("tingo: application %q already registered", name))
	}
	cfg := &AppConfig{Path: name}
	if configurer, ok := app.(AppConfigurer); ok {
		appCfg := configurer.Config()
		appCfg.Path = name
		cfg = &appCfg
	}
	a.applications[name] = &appEntry{
		app:    app,
		config: cfg,
	}
}

// ConfigureApplication 合并应用配置元数据。已注册应用的显式值不会被覆盖。
func (a *App) ConfigureApplication(name string, config AppConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.applications[name]
	if !ok {
		return fmt.Errorf("tingo: application %q not registered", name)
	}
	// 默认前缀：/应用名
	if config.Prefix == "" {
		config.Prefix = "/" + name
	}
	existing := entry.config
	merged := mergeAppConfig(*existing, config)
	entry.config = &merged
	return nil
}

// mergeAppConfig 合并两份配置。base 的显式值优先，fill 仅填充空位。
func mergeAppConfig(base, fill AppConfig) AppConfig {
	if fill.Path != "" {
		base.Path = fill.Path
	}
	if base.Prefix == "" {
		base.Prefix = fill.Prefix
	}
	if base.Domain == "" {
		base.Domain = fill.Domain
	}
	if base.Priority == 0 {
		base.Priority = fill.Priority
	}
	if fill.Disabled {
		base.Disabled = true
	}
	if fill.Default {
		base.Default = true
	}
	if len(base.Middleware) == 0 && len(fill.Middleware) > 0 {
		base.Middleware = fill.Middleware
	}
	return base
}

// LookupApplication 按名称查找已注册应用。
func (a *App) LookupApplication(name string) (AppInfo, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	entry, ok := a.applications[name]
	if !ok {
		return AppInfo{}, false
	}
	return AppInfo{
		App:    entry.app,
		Config: entry.config,
		Name:   name,
	}, true
}

// ApplicationNames 返回所有已注册应用的名称。
func (a *App) ApplicationNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	names := make([]string, 0, len(a.applications))
	for name := range a.applications {
		names = append(names, name)
	}
	return names
}

// Applications 返回所有已注册应用的信息。
func (a *App) Applications() []AppInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]AppInfo, 0, len(a.applications))
	for name, entry := range a.applications {
		result = append(result, AppInfo{
			App:    entry.app,
			Config: entry.config,
			Name:   name,
		})
	}
	return result
}

// MountApplication 注册应用到路由。这是路由注册入口。
func (a *App) MountApplication(r Router, app Application, cfg *AppConfig) Router {
	name := cfg.Path
	normalized := normalizePath(cfg.Prefix)
	a.mu.Lock()
	if _, exists := a.applications[name]; exists && a.applications[name] != nil && a.applications[name].app != nil {
		panic(fmt.Errorf("tingo: application %q already registered", name))
	}
	a.applications[name] = &appEntry{
		app:        app,
		config:     cfg,
		normalized: normalized,
	}
	a.mu.Unlock()
	return a.mountApplication(r, cfg, normalized)
}

// mountApplication 将应用的路由注册到 Router 上。
func (a *App) mountApplication(r Router, cfg *AppConfig, normalized string) Router {
	r = r.Group(normalized, nil)
	for _, h := range cfg.Middleware {
		r.Use(h)
	}
	return r
}

// MustConfigureApplication 根据配置注册应用，失败 panic。
func MustConfigureApplication(r Router, cfg *AppConfig, apps ...Application) {
	a := DefaultApp()
	for _, app := range apps {
		a.MountApplication(r, app, cfg)
	}
}

// MustConfigureApps 根据 tcfg 配置注册应用集。
func MustConfigureApps(r Router, cfg PageConfig, apps ...Application) {
	a := DefaultApp()
	for _, app := range apps {
		a.MountApplication(r, app, &AppConfig{
			Path:       cfg.Path(),
			Prefix:     cfg.Prefix(),
			Middleware: cfg.Middleware(),
		})
	}
}

// ResetApps 清除所有已注册的应用（仅用于测试）。
func ResetApps() {
	defaultAppMu.Lock()
	defaultApp = NewApp()
	defaultAppMu.Unlock()
}

// RegisterApp 注册应用元数据。
func RegisterApp(name string, app Application) {
	DefaultApp().RegisterApplication(name, app)
}

// Apps 注册一组应用到路由。
// Deprecated: 使用 MustConfigureApplication。
func Apps(r Router, cfg *AppConfig, apps ...Application) {
	MustConfigureApplication(r, cfg, apps...)
}

// DefaultApps 兼容旧写法。
// Deprecated: 使用 MustConfigureApplication。
func DefaultApps(r Router, cfg PageConfig, apps ...Application) {
	MustConfigureApps(r, cfg, apps...)
}

// PageConfig 是从配置文件中读取单应用页面配置的接口。
type PageConfig interface {
	Path() string
	Prefix() string
	Middleware() []Handler
}

/* ------------------------------------------------------------------ */
/* 生命周期方法                                                          */
/* ------------------------------------------------------------------ */

// Register 添加服务定义。实际 Register 钩子在 Boot 时按依赖顺序执行。
func (a *App) Register(services ...Service) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	if a.State() != StateCreated {
		return fmt.Errorf("tingo: services cannot be registered while app is %s", a.State())
	}
	for _, service := range services {
		if service == nil {
			return errors.New("tingo: service must not be nil")
		}
		name := strings.TrimSpace(service.Name())
		if name == "" {
			return errors.New("tingo: service name must not be empty")
		}
		if _, exists := a.services[name]; exists {
			return fmt.Errorf("tingo: service %q already registered", name)
		}
		a.services[name] = service
		a.serviceOrder = append(a.serviceOrder, name)
	}
	return nil
}

// Boot 按依赖拓扑执行全部服务的 Register 和 Boot。
func (a *App) Boot(ctx context.Context) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	switch a.State() {
	case StateBooted, StateRunning:
		return nil
	case StateCreated:
	default:
		return fmt.Errorf("tingo: app cannot boot while %s", a.State())
	}

	ordered, err := a.sortServices()
	if err != nil {
		a.setState(StateFailed)
		return err
	}

	a.setState(StateRegistering)
	for _, service := range ordered {
		if err := service.Register(a); err != nil {
			a.setState(StateFailed)
			return fmt.Errorf("tingo: registering service %q: %w", service.Name(), err)
		}
	}
	a.setState(StateRegistered)
	a.setState(StateBooting)
	for _, service := range ordered {
		a.booted = append(a.booted, service)
		if err := service.Boot(ctx, a); err != nil {
			rollbackErr := a.shutdownBooted(ctx)
			a.shutdownDone = true
			a.setState(StateFailed)
			return errors.Join(fmt.Errorf("tingo: booting service %q: %w", service.Name(), err), rollbackErr)
		}
	}
	a.setState(StateBooted)
	return nil
}

// Start 将已完成 Boot 的 App 标记为运行中。
// HTTP、CLI 或其他宿主应在开始对外提供服务前调用。
func (a *App) Start() error {
	a.opMu.Lock()
	defer a.opMu.Unlock()

	switch a.State() {
	case StateBooted:
		a.setState(StateRunning)
		return nil
	case StateRunning:
		return nil
	default:
		return fmt.Errorf("tingo: app cannot start while %s", a.State())
	}
}

// Shutdown 按 Boot 的逆序释放服务和容器资源，重复调用无副作用。
func (a *App) Shutdown(ctx context.Context) error {
	a.opMu.Lock()
	defer a.opMu.Unlock()
	if a.shutdownDone || a.State() == StateStopped {
		return nil
	}
	a.setState(StateStopping)
	serviceErr := a.shutdownBooted(ctx)
	containerErr := a.container.Close()
	a.shutdownDone = true
	a.setState(StateStopped)
	return errors.Join(serviceErr, containerErr)
}

// shutdownBooted 按注册逆序关闭已启动的服务。
func (a *App) shutdownBooted(ctx context.Context) error {
	var result error
	for _, service := range slices.Backward(a.booted) {
		if err := service.Shutdown(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("tingo: shutting down service %q: %w", service.Name(), err))
		}
	}
	a.booted = nil
	return result
}

/* ------------------------------------------------------------------ */
/* 拓扑排序                                                              */
/* ------------------------------------------------------------------ */

// sortServices 按依赖拓扑排序服务列表，检测循环依赖。
func (a *App) sortServices() ([]Service, error) {
	const (
		unvisited uint8 = iota
		visiting
		visited
	)
	marks := make(map[string]uint8, len(a.services))
	ordered := make([]Service, 0, len(a.services))
	var visit func(string) error
	visit = func(name string) error {
		switch marks[name] {
		case visiting:
			return fmt.Errorf("tingo: cyclic service dependency at %q", name)
		case visited:
			return nil
		}
		service, ok := a.services[name]
		if !ok {
			return fmt.Errorf("tingo: service dependency %q is not registered", name)
		}
		marks[name] = visiting
		for _, dependency := range service.DependsOn() {
			if err := visit(dependency); err != nil {
				return fmt.Errorf("tingo: service %q: %w", name, err)
			}
		}
		marks[name] = visited
		ordered = append(ordered, service)
		return nil
	}
	for _, name := range a.serviceOrder {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}
