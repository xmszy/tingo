package tenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileSupportsThinkPHPSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := `[APP]
DEBUG = (true)
DEFAULT_TIMEZONE = Asia/Shanghai

[DATABASE]
TYPE = mysql
HOSTNAME = 127.0.0.1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"APP_DEBUG", "APP_DEFAULT_TIMEZONE", "DATABASE_TYPE", "DATABASE_HOSTNAME"} {
		_ = os.Unsetenv(key)
		key := key
		t.Cleanup(func() { _ = os.Unsetenv(key) })
	}
	if err := LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if !Get("app.debug", false) || Get("app.default_timezone", "") != "Asia/Shanghai" {
		t.Fatal("APP section was not loaded")
	}
	if Get("database.type", "") != "mysql" || Get("database.hostname", "") != "127.0.0.1" {
		t.Fatal("DATABASE section was not loaded")
	}
}

func TestLoadFileKeepsSystemEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("EXISTING=file\nNEW_VALUE=loaded\nQUOTED=\"hello world\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXISTING", "system")
	t.Cleanup(func() {
		_ = os.Unsetenv("NEW_VALUE")
		_ = os.Unsetenv("QUOTED")
	})
	if err := LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("EXISTING"); got != "system" {
		t.Fatalf("system environment overwritten: %q", got)
	}
	if got := os.Getenv("NEW_VALUE"); got != "loaded" {
		t.Fatalf("NEW_VALUE = %q", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Fatalf("QUOTED = %q", got)
	}
}
