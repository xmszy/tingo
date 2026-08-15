package thttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
	terrors "github.com/xmszy/tingo/errors"
	"github.com/xmszy/tingo/os/tcfg"
)

/* ------------------------------------------------------------------ */
/* Engine                                                              */
/* ------------------------------------------------------------------ */

// Engine 是 tingo 的 HTTP 服务引擎，内嵌 gin.Engine。
type Engine struct {
	// app 是拥有该 HTTP 引擎生命周期与业务应用的框架实例。
	app *core.App

	// gin 是底层引擎，直接复用其 radix 路由树与渲染能力。
	gin *gin.Engine

	// cfg 是服务配置。
	cfg Config

	// options 是显式代码配置，Boot 时在约定配置之后重放。
	options []Option

	// configurators 在配置注册表加载后、业务应用启动前装配组件。
	configurators []func(tcfg.Reader) error

	// bindMeta 决定是否在请求上下文中绑定应用/控制器/方法名。
	// 关闭可省去一层闭包，用于极致性能场景。
	bindMeta bool

	// appKeyBound 决定是否在入口中间件把 App 写入 gin.Context.Keys。
	// 该写入会惰性分配 Keys map（每请求 2 alloc / 约 336B）。
	// 单 App 且 app 即 DefaultApp 时可跳过，由 Framework() 的
	// DefaultApp() fallback 直接返回，从而消除这部分分配。
	appKeyBound bool

	// routes 是注册期收集的路由表，用于 CLI 展示与调试。
	mu     sync.Mutex
	routes []RouteInfo

	// hooks 是请求生命周期钩子。
	hooks map[HookType]*hookEntry

	// srv 是底层 http.Server，用于优雅关闭。
	srv *http.Server

	// booted 保证只启动一次。
	booted sync.Once
	// bootErr 记录启动期错误。
	bootErr error
	// adminConfigured 防止重复调用 EnableAdmin。
	adminConfigured bool
}

// RouteInfo 是一条已注册路由的信息。
type RouteInfo struct {
	// Method 是 HTTP 方法，ANY 表示全部方法。
	Method string
	// Path 是完整路径。
	Path string
	// App 是所属应用名。
	App string
	// Handler 是处理器的可读名称。
	Handler string
}

/* ------------------------------------------------------------------ */
/* 构造                                                                */
/* ------------------------------------------------------------------ */

// New 创建绑定默认 App 的 Engine。
func New(opts ...Option) *Engine {
	return NewWithApp(core.DefaultApp(), opts...)
}

// NewWithApp 创建绑定到指定框架 App 的 Engine。
func NewWithApp(app *core.App, opts ...Option) *Engine {
	if app == nil {
		panic("tingo: HTTP engine app must not be nil")
	}
	cfg := defaultConfig()
	for _, option := range opts {
		option(&cfg)
	}

	gin.DebugPrintFunc = func(string, ...any) {}
	gin.DebugPrintRouteFunc = func(string, string, string, int) {}
	gin.SetMode(ginModeFromEnv())

	g := gin.New()
	g.RedirectTrailingSlash = cfg.RedirectTrailingSlash
	g.RedirectFixedPath = cfg.RedirectFixedPath
	g.HandleMethodNotAllowed = cfg.HandleMethodNotAllowed
	g.MaxMultipartMemory = cfg.MaxMultipartMemory
	if len(cfg.TrustedProxies) > 0 {
		_ = g.SetTrustedProxies(cfg.TrustedProxies)
	}

	e := &Engine{
		app:         app,
		gin:         g,
		cfg:         cfg,
		options:     append([]Option(nil), opts...),
		bindMeta:    cfg.BindRouteMeta,
		appKeyBound: app != core.DefaultApp(),
		routes:      make([]RouteInfo, 0, 64),
	}

	if e.appKeyBound {
		g.Use(func(ctx *gin.Context) {
			core.BindFrameworkApp(core.FromGin(ctx), app)
			ctx.Next()
		})
	}
	e.installNotFound()
	return e
}

// Gin 返回底层 *gin.Engine，用于使用 tingo 尚未封装的能力。
func (e *Engine) Gin() *gin.Engine { return e.gin }

// Config 返回当前配置的副本。
func (e *Engine) Config() Config { return e.cfg }

func (e *Engine) applyConfig(cfg Config) error {
	e.cfg = cfg
	e.bindMeta = cfg.BindRouteMeta
	gin.SetMode(ginModeFromEnv())
	e.gin.RedirectTrailingSlash = cfg.RedirectTrailingSlash
	e.gin.RedirectFixedPath = cfg.RedirectFixedPath
	e.gin.HandleMethodNotAllowed = cfg.HandleMethodNotAllowed
	e.gin.MaxMultipartMemory = cfg.MaxMultipartMemory
	if err := e.gin.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return fmt.Errorf("tingo: invalid trusted proxies: %w", err)
	}
	return nil
}

/* ------------------------------------------------------------------ */
/* 中间件与路由                                                          */
/* ------------------------------------------------------------------ */

// ConfigureAtBoot 注册配置消费者，在注册表加载后、业务路由装配前执行。
func (e *Engine) ConfigureAtBoot(configurators ...func(tcfg.Reader) error) *Engine {
	e.configurators = append(e.configurators, configurators...)
	return e
}

// Use 注册全局中间件。必须在 Boot/Run 之前调用。
func (e *Engine) Use(mws ...core.Handler) *Engine {
	e.gin.Use(core.GinChain(mws)...)
	return e
}

// UseGin 注册原生 gin 中间件，用于复用 gin 生态。
func (e *Engine) UseGin(mws ...gin.HandlerFunc) *Engine {
	e.gin.Use(mws...)
	return e
}

// Router 返回根路由器，用于注册不属于任何应用的全局路由。
func (e *Engine) Router() core.Router {
	return &router{group: e.gin, app: "", engine: e, basePath: ""}
}

// record 记录一条路由到路由表。
func (e *Engine) record(method, path, app, handler string) {
	e.mu.Lock()
	e.routes = append(e.routes, RouteInfo{
		Method: method, Path: path, App: app, Handler: handler,
	})
	e.mu.Unlock()
}

// Routes 返回已注册的全部路由，按路径排序。
func (e *Engine) Routes() []RouteInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]RouteInfo, len(e.routes))
	copy(out, e.routes)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// PrintRouteTable 以表格形式打印所有注册的路由。
// 支持 TINGO_LIST_ROUTES_METHOD 环境变量按方法筛选。
func (e *Engine) PrintRouteTable() {
	routes := e.Routes()
	methodFilter := strings.ToUpper(os.Getenv("TINGO_LIST_ROUTES_METHOD"))

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%-7s  %-40s  %s\n", "Method", "Path", "Handler")
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 78))
	count := 0
	for _, r := range routes {
		if methodFilter != "" && r.Method != methodFilter {
			continue
		}
		fmt.Fprintf(os.Stdout, "%-7s  %-40s  %s\n", r.Method, r.Path, r.Handler)
		count++
	}
	if count == 0 {
		if methodFilter != "" {
			fmt.Fprintf(os.Stdout, "(no routes match method %s)\n", methodFilter)
		} else {
			fmt.Fprintln(os.Stdout, "(no routes registered)")
		}
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "total: %d route(s)\n", count)
}

/* ------------------------------------------------------------------ */
/* 多应用装配                                                           */
/* ------------------------------------------------------------------ */

// Boot 装配全部已注册的应用。幂等，重复调用无副作用。
//
// 装配顺序：
//  1. 按 Priority 排序应用
//  2. 逐个执行 Boot 钩子
//  3. 建立应用路由组（域名绑定 / 前缀）
//  4. 挂载应用级中间件
//  5. 注册应用路由
//
// 全部工作在启动期完成，运行时不再有任何应用维度的解析。
func (e *Engine) Boot() error {
	e.booted.Do(func() { e.bootErr = e.boot() })
	return e.bootErr
}

// boot 是 Boot 的实际实现。
func (e *Engine) boot() error {
	// 安装生命周期钩子中间件（必须在任何业务路由之前）。
	e.installHookMiddleware()

	if err := e.app.Boot(context.Background()); err != nil {
		return err
	}
	var tree tcfg.Reader = tcfg.NewFromTree(nil)
	if registry, err := core.Resolve[*tcfg.Registry](e.app.Container()); err == nil {
		tree = registry.Global()
	}
	cfg := defaultConfig()
	if err := loadConventionConfig(tree, &cfg); err != nil {
		return err
	}
	for _, option := range e.options {
		option(&cfg)
	}
	if err := validateConventionConfig(cfg); err != nil {
		return err
	}
	if err := e.applyConfig(cfg); err != nil {
		return err
	}
	// 全局请求体大小限制：在业务路由之前注册，读取 e.cfg.MaxBody（请求期生效）。
	if cfg.MaxBody > 0 {
		e.gin.Use(func(c *gin.Context) {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxBody)
			c.Next()
		})
	}
	for _, configure := range e.configurators {
		if err := configure(tree); err != nil {
			return err
		}
	}
	for _, info := range e.app.Applications() {
		// 配置驱动解析：根据框架配置（default_app/app_map/domain_bind/deny_app）
		// 计算每个应用最终的 AppConfig。
		if core.AppConfigProvider != nil && info.Config != nil {
			resolved := core.AppConfigProvider(info.Name, *info.Config)
			*info.Config = resolved
		}

		if b, ok := info.App.(core.AppBooter); ok {
			if err := b.Boot(); err != nil {
				return terrors.ErrInternal.
					WithMessagef("tingo: booting application %q failed", info.Name).
					Wrap(err)
			}
		}

		r, err := e.mountApp(info)
		if err != nil {
			return err
		}
		info.App.Routes(r)
	}
	return nil
}

// mountApp 为一个应用建立路由组。
func (e *Engine) mountApp(info core.AppInfo) (core.Router, error) {
	// 计算本应用的路由前缀：版本前缀（如 /v1）+ 应用前缀（如 /api）。
	groupPath := effectivePrefix(e.cfg.Version, info.Config.Prefix)

	var (
		group    gin.IRouter
		basePath string
	)

	switch {
	case info.Config.Domain != "":
		// 域名绑定：域名无法进入 radix tree（gin 只按路径匹配），
		// 因此挂在根路径并用中间件做一次 Host 校验，不匹配则放行给其他应用。
		// 这是唯一有运行时字符串比较的场景，且仅影响域名绑定的应用。
		g := e.gin.Group(groupPath)
		domain := info.Config.Domain
		g.Use(func(c *gin.Context) {
			if !hostMatches(c.Request.Host, domain) {
				c.Abort()
				core.CurrentResponder().Fail(core.FromGin(c),
					terrors.ErrNotFound.WithMessagef("域名未绑定该应用: %s", c.Request.Host))
				return
			}
			c.Next()
		})
		group, basePath = g, groupPath

	default:
		group, basePath = e.gin.Group(groupPath), groupPath
	}

	if mw, ok := info.App.(core.AppMiddlewarer); ok {
		if hs := mw.Middlewares(); len(hs) > 0 {
			group.(*gin.RouterGroup).Use(core.GinChain(hs)...)
		}
	}

	return &router{
		group:    group,
		app:      info.Name,
		engine:   e,
		basePath: basePath,
	}, nil
}

// effectivePrefix 合并版本前缀与业务前缀，得到最终路由组路径。
// 例如 version="v1", prefix="/api" => "/v1/api"；version="" => "/api"；都为空 => "/"。
func effectivePrefix(version, prefix string) string {
	if prefix == "" || prefix == "/" {
		prefix = ""
	}
	if version != "" {
		v := "/" + strings.Trim(version, "/")
		if prefix == "" {
			return v
		}
		return v + prefix
	}
	if prefix == "" {
		return "/"
	}
	return prefix
}

// hostMatches 判断请求 Host 是否匹配绑定域名（忽略端口）。
func hostMatches(host, domain string) bool {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host == domain
}

/* ------------------------------------------------------------------ */
/* 404 / 405                                                           */
/* ------------------------------------------------------------------ */

// installNotFound 安装统一的 404 与 405 处理器，
// 使其响应格式与业务错误保持一致。
func (e *Engine) installNotFound() {
	e.gin.NoRoute(func(c *gin.Context) {
		core.CurrentResponder().Fail(core.FromGin(c),
			terrors.ErrNotFound.WithMessagef("路由不存在: %s %s", c.Request.Method, c.Request.URL.Path))
	})
	e.gin.NoMethod(func(c *gin.Context) {
		core.CurrentResponder().Fail(core.FromGin(c),
			terrors.ErrMethodNotAllowed.WithMessagef("方法不允许: %s", c.Request.Method))
	})
}

/* ------------------------------------------------------------------ */
/* 启动与优雅关闭                                                        */
/* ------------------------------------------------------------------ */

// ServeHTTP 实现 http.Handler，便于测试与嵌入其他服务。
// 调用前会确保应用已装配。
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := e.Boot(); err != nil {
		panic(err)
	}
	e.gin.ServeHTTP(w, r)
}

// Run 启动服务并阻塞，收到 SIGINT/SIGTERM 时优雅关闭。
func (e *Engine) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return e.RunContext(ctx)
}

// RunContext 启动服务并阻塞，ctx 取消时优雅关闭。
// 调用方可以用它把 HTTP 服务纳入自己的进程生命周期。
func (e *Engine) RunContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("tingo: HTTP run context must not be nil")
	}
	startedAt := time.Now()
	if err := e.Boot(); err != nil {
		return err
	}

	// 路由列表模式：TINGO_LIST_ROUTES=1 时打印路由表后退出，不启动服务。
	if os.Getenv("TINGO_LIST_ROUTES") == "1" {
		e.PrintRouteTable()
		return nil
	}

	if err := e.app.Start(); err != nil {
		return err
	}

	e.srv = &http.Server{
		Addr:              e.cfg.Addr,
		Handler:           e.gin,
		ReadTimeout:       e.cfg.ReadTimeout,
		WriteTimeout:      e.cfg.WriteTimeout,
		IdleTimeout:       e.cfg.IdleTimeout,
		ReadHeaderTimeout: e.cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    e.cfg.MaxHeaderBytes,
	}

	listener, err := net.Listen("tcp", e.cfg.Addr)
	if err != nil {
		return errors.Join(err, e.Shutdown())
	}

	errCh := make(chan error, 1)
	go func() {
		e.printStartup(listener.Addr(), time.Since(startedAt))
		var serveErr error
		if e.cfg.CertFile != "" && e.cfg.KeyFile != "" {
			serveErr = e.srv.ServeTLS(listener, e.cfg.CertFile, e.cfg.KeyFile)
		} else {
			serveErr = e.srv.Serve(listener)
		}
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		errCh <- serveErr
	}()

	select {
	case err := <-errCh:
		return errors.Join(err, e.Shutdown())
	case <-ctx.Done():
		return e.Shutdown()
	}
}

// Shutdown 使用配置的超时时间优雅关闭服务。
func (e *Engine) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.ShutdownTimeout)
	defer cancel()
	return e.ShutdownContext(ctx)
}

// ShutdownContext 使用调用方提供的上下文关闭服务器和 App。
func (e *Engine) ShutdownContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("tingo: HTTP shutdown context must not be nil")
	}
	var serverErr error
	if e.srv != nil {
		fmt.Fprintln(os.Stdout, "\ntingo: shutting down gracefully...")
		if err := e.srv.Shutdown(ctx); err != nil {
			serverErr = terrors.ErrInternal.WithMessage("tingo: graceful shutdown failed").Wrap(err)
		}
	}
	appErr := e.app.Shutdown(ctx)
	if serverErr == nil && appErr == nil && e.srv != nil {
		fmt.Fprintln(os.Stdout, "tingo: stopped")
	}
	return errors.Join(serverErr, appErr)
}

// printStartup 打印服务已就绪信息。
func (e *Engine) printStartup(boundAddr net.Addr, elapsed time.Duration) {
	fmt.Fprintf(os.Stdout, "\n[TINGO] mode=%s\n", ginModeFromEnv())
	if e.cfg.PrintRoutes {
		for _, route := range e.Routes() {
			fmt.Fprintf(os.Stdout, "[TINGO] %-7s %-32s --> %s\n", route.Method, route.Path, route.Handler)
		}
	}
	scheme := e.cfg.CertFile != "" && e.cfg.KeyFile != ""
	host, port, err := net.SplitHostPort(boundAddr.String())
	if err != nil {
		host, port = boundAddr.String(), ""
	}
	switch {
	case host == "" || host == "0.0.0.0" || host == "::":
		// 监听所有网卡：localhost + 真实局域网地址都可访问（仅 IPv4）
		fmt.Fprintf(os.Stdout, "[TINGO] Listening and serving HTTP on %s (localhost)\n", schemeURL("http", "localhost", port, scheme))
		for _, ip := range lanIPv4() {
			fmt.Fprintf(os.Stdout, "[TINGO]   LAN  http://%s\n", net.JoinHostPort(ip, port))
		}
	default:
		fmt.Fprintf(os.Stdout, "[TINGO] Listening and serving HTTP on %s\n", schemeURL("http", host, port, scheme))
	}
	fmt.Fprintf(os.Stdout, "[TINGO] Ready in %s\n\n", elapsed.Round(100*time.Microsecond))
}

func schemeURL(defaultScheme, host, port string, tls bool) string {
	scheme := defaultScheme
	if tls {
		scheme = "https"
	}
	if port == "" {
		return scheme + "://" + host
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

// lanIPv4 返回本机物理网卡上的 IPv4 局域网地址（排除回环、链路本地与虚拟/容器网卡）。
func lanIPv4() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isVirtualInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue // 仅 IPv4
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			s := ip.String()
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// isVirtualInterface 判断网卡名是否属于虚拟/容器/隧道网卡，这些地址不应作为局域网访问入口展示。
// 命中规则基于网卡名（Windows 下为「网络连接名」，如 NodeBabyLink WireGuard Tunnel）。
func isVirtualInterface(name string) bool {
	lower := strings.ToLower(name)
	prefixes := []string{"lo", "vmnet", "vmware", "vboxnet", "docker", "br-", "veth", "cni", "flannel", "cali", "wg", "tun", "tap", "ppp"}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	keywords := []string{"virtual", "hyper-v", "vethernet", "wsl", "loopback", "wireguard", "tunnel", "vpn"}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// 确保实现 http.Handler。
var _ http.Handler = (*Engine)(nil)
