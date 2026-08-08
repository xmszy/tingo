package thttp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tcfg"
)

func TestConventionConfigAndEnvironmentPriority(t *testing.T) {
	t.Setenv("TINGO_ADDR", ":9090")
	t.Setenv("APP_DEBUG", "true")
	tree := tcfg.Tree{
		"app": map[string]any{
			"debug": false,
			"server": map[string]any{
				"addr":            ":8088",
				"print_routes":    true,
				"read_timeout":    "5s",
				"trusted_proxies": []any{"127.0.0.1"},
			},
		},
		"route": map[string]any{
			"redirect_trailing_slash":   false,
			"handle_method_not_allowed": false,
		},
	}

	cfg := defaultConfig()
	if err := loadConventionConfig(tree, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" || !cfg.PrintRoutes {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.ReadTimeout != 5*time.Second || cfg.RedirectTrailingSlash || cfg.HandleMethodNotAllowed {
		t.Fatalf("server/route config was not applied: %+v", cfg)
	}
	if len(cfg.TrustedProxies) != 1 || cfg.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("trusted proxies = %#v", cfg.TrustedProxies)
	}
}

func TestEngineBootAppliesRegistryThenExplicitOptions(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "app.toml"), []byte("[server]\naddr = ':8088'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(tcfg.ExtensionEnv, tcfg.DefaultExtension)
	t.Setenv("TINGO_ADDR", ":9090")
	app := core.NewApp()
	if err := app.Register(tcfg.NewService(root)); err != nil {
		t.Fatal(err)
	}
	engine := NewWithApp(app, Addr(":10080"))
	if err := engine.Boot(); err != nil {
		t.Fatal(err)
	}
	if got := engine.Config().Addr; got != ":10080" {
		t.Fatalf("explicit option did not win: %q", got)
	}
}

func TestConventionConfigRejectsInvalidValues(t *testing.T) {
	cfg := defaultConfig()
	if err := loadConventionConfig(tcfg.Tree{
		"app": map[string]any{"server": map[string]any{"read_timeout": "soon"}},
	}, &cfg); err == nil {
		t.Fatal("expected invalid duration error")
	}

	cfg = defaultConfig()
	if err := loadConventionConfig(tcfg.Tree{
		"app": map[string]any{"server": map[string]any{"read_timeout": "5s"}},
	}, &cfg); err != nil {
		t.Fatalf("unexpected error for valid duration: %v", err)
	}
}
