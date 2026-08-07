package tcfg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type ServerConf struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type AppConf struct {
	Server   ServerConf      `json:"server"`
	Name     string          `json:"name"`
	Features map[string]bool `json:"features"`
}

const tomlContent = `name = "demo"
[server]
host = "0.0.0.0"
port = 8080
[features]
auth = true
cache = false
`

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func unsetEnvironment(t *testing.T, name string) {
	t.Helper()
	previous, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}

func TestConfigWithFileAdapter(t *testing.T) {
	path := writeTemp(t, "config.toml", tomlContent)
	adapter, err := NewFileAdapter(path)
	if err != nil {
		t.Fatal(err)
	}
	config := New(adapter)
	if config.String("name") != "demo" || config.Int("server.port") != 8080 {
		t.Fatalf("config = %#v", config.Data())
	}
	first := config.Data()
	first["name"] = "changed"
	if config.String("name") != "demo" {
		t.Fatal("caller mutated adapter cache")
	}
}

func TestContentAdapterAndLoader(t *testing.T) {
	config, err := NewFromBytes("toml", []byte(tomlContent))
	if err != nil {
		t.Fatal(err)
	}
	loader, err := NewLoader[ServerConf](config, "server")
	if err != nil {
		t.Fatal(err)
	}
	if got := loader.Get(); got.Host != "0.0.0.0" || got.Port != 8080 {
		t.Fatalf("server = %+v", got)
	}
}

func TestLoaderCustomConverter(t *testing.T) {
	config, err := NewFromBytes("toml", []byte(tomlContent))
	if err != nil {
		t.Fatal(err)
	}
	loader, err := NewLoader[AppConf](config, "")
	if err != nil {
		t.Fatal(err)
	}
	loader.SetConverter(func(data any, target *AppConf) error {
		if err := Decode(data, target); err != nil {
			return err
		}
		target.Name = strings.ToUpper(target.Name)
		return nil
	})
	if err := loader.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if loader.Get().Name != "DEMO" {
		t.Fatalf("config = %+v", loader.Get())
	}
}

func TestLoaderNamedWatch(t *testing.T) {
	path := writeTemp(t, "config.toml", tomlContent)
	adapter, err := NewFileAdapter(path)
	if err != nil {
		t.Fatal(err)
	}
	loader, err := NewLoader[AppConf](New(adapter), "")
	if err != nil {
		t.Fatal(err)
	}
	changed := make(chan AppConf, 1)
	loader.OnChange(func(value AppConf) error {
		changed <- value
		return nil
	})
	ctx := t.Context()
	if err := loader.Watch(ctx, "app-test", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !loader.IsWatching() {
		t.Fatal("loader is not watching")
	}
	updated := `name = "changed"
[server]
host = "127.0.0.1"
port = 9090
`
	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-changed:
		if value.Server.Port != 9090 {
			t.Fatalf("config = %+v", value)
		}
	case <-time.After(time.Second):
		t.Fatal("watch did not fire")
	}
	if !loader.StopWatch() || loader.IsWatching() {
		t.Fatal("watcher was not removed")
	}
}

func TestLoaderWatchReportsReloadError(t *testing.T) {
	path := writeTemp(t, "config.toml", tomlContent)
	adapter, err := NewFileAdapter(path)
	if err != nil {
		t.Fatal(err)
	}
	loader, err := NewLoader[AppConf](New(adapter), "")
	if err != nil {
		t.Fatal(err)
	}
	watchErrors := make(chan error, 1)
	loader.OnWatchError(func(_ context.Context, err error) {
		select {
		case watchErrors <- err:
		default:
		}
	})
	ctx := t.Context()
	if err := loader.Watch(ctx, "reload-error-test", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	defer loader.StopWatch()

	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(path, []byte("invalid = ["), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-watchErrors:
		if err == nil {
			t.Fatal("watch error callback received nil")
		}
	case <-time.After(time.Second):
		t.Fatal("reload parse error was not reported")
	}
}

func TestGetEffectivePriority(t *testing.T) {
	config := NewFromTree(Tree{"app": map[string]any{"name": "file"}})
	t.Setenv("APP_NAME", "environment")
	value, err := config.GetEffective(context.Background(), "app.name", "default")
	if err != nil {
		t.Fatal(err)
	}
	if value != "environment" {
		t.Fatalf("effective value = %#v", value)
	}
	value, err = config.GetEffective(context.Background(), "app.missing", "default")
	if err != nil || value != "default" {
		t.Fatalf("default value = %#v, %v", value, err)
	}
}

func TestEnvironmentExpansionDefaultsAndOverrides(t *testing.T) {
	const (
		typeKey = "TINGO_TCFG_TEST_DB_TYPE"
		hostKey = "TINGO_TCFG_TEST_DB_HOST"
		passKey = "TINGO_TCFG_TEST_DB_PASS"
	)
	unsetEnvironment(t, hostKey)
	unsetEnvironment(t, passKey)
	t.Setenv(typeKey, "postgres")
	content := []byte(`type = "${` + typeKey + `:-mysql}"
host = "${` + hostKey + `:-127.0.0.1}"
password = "${` + passKey + `:-}"
`)

	config, err := NewFromBytes("toml", content)
	if err != nil {
		t.Fatal(err)
	}
	if config.String("type") != "postgres" || config.String("host") != "127.0.0.1" || config.String("password", "missing") != "" {
		t.Fatalf("expanded content = %#v", config.Data())
	}

	path := writeTemp(t, "environment.toml", string(content))
	fileConfig, err := NewFromFiles(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileConfig.String("type") != config.String("type") || fileConfig.String("host") != config.String("host") {
		t.Fatalf("file/content expansion differs: file=%#v content=%#v", fileConfig.Data(), config.Data())
	}
}

func TestEnvironmentExpansionRejectsMissingVariable(t *testing.T) {
	const key = "TINGO_TCFG_TEST_REQUIRED_MISSING"
	unsetEnvironment(t, key)
	_, err := NewFromBytes("toml", []byte(`value = "${`+key+`}"`))
	if err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "content") {
		t.Fatalf("missing variable error = %v", err)
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := NewFileAdapter(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Fatal("missing file was accepted")
	}
}
