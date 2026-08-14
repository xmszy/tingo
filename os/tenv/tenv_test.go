package tenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDefault(t *testing.T) {
	t.Setenv("TENV_TEST_INT", "")
	if got := Get("TENV_TEST_INT", 42); got != 42 {
		t.Fatalf("expected default 42, got %d", got)
	}
}

func TestGetInt(t *testing.T) {
	t.Setenv("TENV_TEST_INT", "7")
	if got := Get("TENV_TEST_INT", 0); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestGetBool(t *testing.T) {
	t.Setenv("TENV_TEST_BOOL", "true")
	if got := Get("TENV_TEST_BOOL", false); !got {
		t.Fatalf("expected true")
	}
}

func TestMustGetPanic(t *testing.T) {
	if _, ok := func() (int, bool) {
		defer func() { _ = recover() }()
		return MustGet[int]("TENV_NOT_EXIST"), true
	}(); ok {
		t.Fatal("expected panic")
	}
}

func TestSetGetenv(t *testing.T) {
	if err := Set("TENV_WR", "v"); err != nil {
		t.Fatal(err)
	}
	if Getenv("TENV_WR", "x") != "v" {
		t.Fatal("Set/Getenv mismatch")
	}
}

func TestEnvStyleKeysAndValues(t *testing.T) {
	t.Setenv("DATABASE_HOSTNAME", "127.0.0.1")
	t.Setenv("APP_DEBUG", "(true)")
	t.Setenv("APP_EMPTY", "")
	t.Setenv("APP_NULL", "(null)")

	if got := Get("database.hostname", ""); got != "127.0.0.1" {
		t.Fatalf("dot path = %q", got)
	}
	if got := Get("app.debug", false); !got {
		t.Fatal("Tingo boolean literal was not parsed")
	}
	if !Has("app.empty") {
		t.Fatal("empty environment variable should still exist")
	}
	if got, ok := Lookup("app.empty"); !ok || got != "" {
		t.Fatalf("empty lookup = %q, %v", got, ok)
	}
	if got := Value("app.null", "fallback"); got != nil {
		t.Fatalf("null value = %#v", got)
	}
	if got := Value("app.missing", "fallback"); got != "fallback" {
		t.Fatalf("missing default = %#v", got)
	}
}

func TestUnsetAndAll(t *testing.T) {
	t.Setenv("TENV_SCOPE_ONE", "1")
	if got := All("tenv.scope"); got["TENV_SCOPE_ONE"] != "1" {
		t.Fatalf("filtered env = %#v", got)
	}
	if err := Unset("tenv.scope.one"); err != nil {
		t.Fatal(err)
	}
	if Has("TENV_SCOPE_ONE") {
		t.Fatal("environment variable was not removed")
	}
}

func TestGetMap(t *testing.T) {
	t.Setenv("TENV_MAP", "a=1, b=2,c=3")
	m := GetMap("TENV_MAP")
	if m["a"] != "1" || m["b"] != "2" || m["c"] != "3" {
		t.Fatalf("map parse wrong: %v", m)
	}
}

func TestLoadFileSupportsEnvSections(t *testing.T) {
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
