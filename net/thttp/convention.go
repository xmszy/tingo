package thttp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xmszy/tingo/os/tcfg"
	"github.com/xmszy/tingo/os/tenv"
)

// loadConventionConfig 按“默认值 < 全局配置 < 环境变量”的顺序装配 HTTP 配置。
// 显式 Option 由 Engine 在此函数之后应用。
func loadConventionConfig(tree tcfg.Reader, cfg *Config) error {
	if addr := tree.String("app.server.addr"); addr != "" {
		cfg.Addr = addr
	}
	if err := applyDuration(tree, "app.server.read_timeout", &cfg.ReadTimeout); err != nil {
		return err
	}
	if err := applyDuration(tree, "app.server.read_header_timeout", &cfg.ReadHeaderTimeout); err != nil {
		return err
	}
	if err := applyDuration(tree, "app.server.write_timeout", &cfg.WriteTimeout); err != nil {
		return err
	}
	if err := applyDuration(tree, "app.server.idle_timeout", &cfg.IdleTimeout); err != nil {
		return err
	}
	if err := applyDuration(tree, "app.server.shutdown_timeout", &cfg.ShutdownTimeout); err != nil {
		return err
	}
	if tree.Has("app.server.max_header_bytes") {
		cfg.MaxHeaderBytes = tree.Int("app.server.max_header_bytes", cfg.MaxHeaderBytes)
	}
	if tree.Has("app.server.max_multipart_memory") {
		cfg.MaxMultipartMemory = tree.Int64("app.server.max_multipart_memory", cfg.MaxMultipartMemory)
	}
	if value := tree.String("app.server.cert_file"); value != "" {
		cfg.CertFile = value
	}
	if value := tree.String("app.server.key_file"); value != "" {
		cfg.KeyFile = value
	}
	if tree.Has("app.server.trusted_proxies") {
		cfg.TrustedProxies = tree.Strings("app.server.trusted_proxies")
	}
	if tree.Has("app.server.print_routes") {
		cfg.PrintRoutes = tree.Bool("app.server.print_routes")
	}

	applyRouteConfig(tree, cfg)
	applyEnvironmentConfig(cfg)
	return validateConventionConfig(*cfg)
}

func applyRouteConfig(tree tcfg.Reader, cfg *Config) {
	if tree.Has("route.redirect_trailing_slash") {
		cfg.RedirectTrailingSlash = tree.Bool("route.redirect_trailing_slash")
	}
	if tree.Has("route.redirect_fixed_path") {
		cfg.RedirectFixedPath = tree.Bool("route.redirect_fixed_path")
	}
	if tree.Has("route.handle_method_not_allowed") {
		cfg.HandleMethodNotAllowed = tree.Bool("route.handle_method_not_allowed")
	}
	if tree.Has("route.bind_route_meta") {
		cfg.BindRouteMeta = tree.Bool("route.bind_route_meta")
	}
}

func applyEnvironmentConfig(cfg *Config) {
	cfg.Addr = ResolveAddr(tenv.Get("SERVER_ADDR", tenv.Get("TINGO_ADDR", cfg.Addr)))
	cfg.PrintRoutes = tenv.Get("TINGO_PRINT_ROUTES", cfg.PrintRoutes)
}

// ResolveAddr 规整监听地址。允许直接写数字端口（如 "8081"），
// 自动补 ":" 前缀得到 ":8081"；含 ":" 或为空则原样返回。
func ResolveAddr(addr string) string {
	if addr == "" {
		return addr
	}
	if _, err := strconv.Atoi(strings.TrimSpace(addr)); err == nil {
		return ":" + strings.TrimSpace(addr)
	}
	return addr
}

func applyDuration(tree tcfg.Reader, path string, target *time.Duration) error {
	if !tree.Has(path) {
		return nil
	}
	value := tree.String(path)
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("tingo: invalid duration %s=%q: %w", path, value, err)
	}
	*target = duration
	return nil
}

// validateConventionConfig 校验 HTTP 配置的合法性与一致性，
// 在 Engine 启动前尽早暴露错误配置（如地址为空、超时为负、TLS 证书不对称）。
func validateConventionConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Addr) == "" {
		return fmt.Errorf("thttp: Addr must not be empty")
	}

	// 超时时间不应为负：负超时在 http.Server 中是未定义行为，
	// 会让连接立即超时或永久阻塞，这里提前拦截。
	if cfg.ReadTimeout < 0 {
		return fmt.Errorf("thttp: ReadTimeout must not be negative, got %s", cfg.ReadTimeout)
	}
	if cfg.ReadHeaderTimeout < 0 {
		return fmt.Errorf("thttp: ReadHeaderTimeout must not be negative, got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.WriteTimeout < 0 {
		return fmt.Errorf("thttp: WriteTimeout must not be negative, got %s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout < 0 {
		return fmt.Errorf("thttp: IdleTimeout must not be negative, got %s", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout < 0 {
		return fmt.Errorf("thttp: ShutdownTimeout must not be negative, got %s", cfg.ShutdownTimeout)
	}

	// TLS 证书应成对出现：仅配置其一会导致 HTTPS 启动失败。
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return fmt.Errorf("thttp: CertFile and KeyFile must be set together for HTTPS")
	}

	// 若绑定路由元信息，则上下文会依赖 Ctx.App/Controller/Action，
	// 此处无需校验；但内存/头字节上限为负无意义。
	if cfg.MaxHeaderBytes < 0 {
		return fmt.Errorf("thttp: MaxHeaderBytes must not be negative, got %d", cfg.MaxHeaderBytes)
	}
	if cfg.MaxMultipartMemory < 0 {
		return fmt.Errorf("thttp: MaxMultipartMemory must not be negative, got %d", cfg.MaxMultipartMemory)
	}

	return nil
}
