package thttp

import (
	"fmt"
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
	if mode := tree.String("app.server.mode"); mode != "" {
		cfg.Mode = Mode(mode)
	} else if tree.Has("app.debug") && !tree.Bool("app.debug", true) {
		cfg.Mode = ModeRelease
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
	cfg.Addr = tenv.Get("TINGO_ADDR", cfg.Addr)
	cfg.Mode = Mode(tenv.Get("TINGO_MODE", string(cfg.Mode)))
	if tenv.Has("APP_DEBUG") && !tenv.Get("APP_DEBUG", true) {
		cfg.Mode = ModeRelease
	}
	cfg.PrintRoutes = tenv.Get("TINGO_PRINT_ROUTES", cfg.PrintRoutes)
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

func validateConventionConfig(cfg Config) error {
	if cfg.Mode != ModeDebug && cfg.Mode != ModeTest && cfg.Mode != ModeRelease {
		return fmt.Errorf("tingo: unsupported server mode %q", cfg.Mode)
	}
	return nil
}
