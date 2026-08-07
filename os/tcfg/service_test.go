package tcfg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tcfg"
)

func TestFlatConventionLoadsDefaultApp(t *testing.T) {
	t.Setenv(tcfg.ExtensionEnv, tcfg.DefaultExtension)
	root := t.TempDir()
	writeConfig(t, root, "app.toml", "[app]\ndebug = true\n[server]\naddr = ':8080'\n")

	registry, err := tcfg.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Global().Bool("app.debug", false); !got {
		t.Fatalf("app.debug = %v", got)
	}
	if got, _ := registry.Global().Lookup("server.addr"); got != ":8080" {
		t.Fatalf("server.addr = %#v", got)
	}
}

func TestMultiAppDirectoryConvention(t *testing.T) {
	t.Setenv(tcfg.ExtensionEnv, tcfg.DefaultExtension)
	root := t.TempDir()
	writeConfig(t, root, "app/app.toml", "[app]\nname = 'global'\n[server]\naddr = ':8080'\n")
	writeConfig(t, root, "admin/app.toml", "[app]\nname = 'admin'\n[server]\naddr = ':9090'\n")

	registry, err := tcfg.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	globalApp := registry.Global()
	if got := globalApp.String("app.name"); got != "global" {
		t.Fatalf("global app name = %q", got)
	}
	if got, _ := globalApp.Lookup("server.addr"); got != ":8080" {
		t.Fatalf("global server.addr = %#v", got)
	}

	adminApp := registry.ApplicationFor("admin")
	if got := adminApp.String("app.name"); got != "admin" {
		t.Fatalf("admin app name = %q", got)
	}
	if got, _ := adminApp.Lookup("server.addr"); got != ":9090" {
		t.Fatalf("admin server.addr = %#v", got)
	}
	if got, _ := registry.Global().Lookup("app.name"); got == "admin" {
		t.Fatal("admin config leaked into global scope")
	}
}

func TestRegistryReturnsIndependentViews(t *testing.T) {
	t.Setenv(tcfg.ExtensionEnv, tcfg.DefaultExtension)
	root := t.TempDir()
	writeConfig(t, root, "app.toml", "[app]\ndebug = true\n")

	registry, err := tcfg.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	first := registry.Global().Data()
	first["app"].(map[string]any)["debug"] = false
	second := registry.Global()
	if got := second.Bool("app.debug", false); !got {
		t.Fatal("caller mutated registry state")
	}
}

func TestRegistryUsesConfiguredExtension(t *testing.T) {
	t.Setenv(tcfg.ExtensionEnv, ".json")
	root := t.TempDir()
	writeConfig(t, root, "app.toml", "name = 'ignored'\n")
	writeConfig(t, root, "app.json", `{"app":{"name":"global"},"server":{"addr":":8080"}}`)
	writeConfig(t, root, "admin/app.json", `{"app":{"name":"admin"}}`)

	registry, err := tcfg.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	app := registry.ApplicationFor("app")
	if got := app.String("app.name"); got != "global" {
		t.Fatalf("application name = %q", got)
	}
	if got := app.String("server.addr"); got != ":8080" {
		t.Fatalf("server addr = %q", got)
	}
	admin := registry.ApplicationFor("admin")
	if got := admin.String("app.name"); got != "admin" {
		t.Fatalf("frontend name = %q", got)
	}
}

func TestServiceRegistration(t *testing.T) {
	t.Setenv(tcfg.ExtensionEnv, tcfg.DefaultExtension)
	root := t.TempDir()
	writeConfig(t, root, "app.toml", "[app]\ndebug = true\n")
	app := core.NewApp()
	app.RegisterApplication("app", testApplication{})
	svc := tcfg.NewService(root)
	if svc.Name() != tcfg.ServiceName {
		t.Fatalf("service name = %q", svc.Name())
	}
	if err := app.Register(svc); err != nil {
		t.Fatal(err)
	}
	if !app.HasService(tcfg.ServiceName) {
		t.Fatal("service should be registered")
	}
}

type testApplication struct{}

func (testApplication) Routes(core.Router) {}

func writeConfig(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, "config", filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
