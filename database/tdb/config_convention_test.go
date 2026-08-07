package tdb_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestLoadConfigResolvesThinkPHPStyleConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.toml")
	t.Setenv("TEST_DB_HOST", "db.internal")
	content := `default = "primary"

[connections.primary]
type = "mysql"
hostname = "${TEST_DB_HOST}"
database = "demo"
username = "root"
password = "secret"
hostport = "3307"
charset = "utf8mb4"
max_open = 12
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := tdb.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := cfg.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if connection.Driver != "mysql" || connection.MaxOpen != 12 {
		t.Fatalf("unexpected connection: %+v", connection)
	}
	if !strings.Contains(connection.DSN, "tcp(db.internal:3307)/demo") {
		t.Fatalf("unexpected DSN: %s", connection.DSN)
	}
}

func TestLoadConfigUsesThinkPHPEnvironmentNamesAndDefaults(t *testing.T) {
	for _, name := range []string{"DB_DRIVER", "DB_TYPE", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASS", "DB_PORT", "DB_CHARSET", "DB_PREFIX"} {
		t.Setenv(name, "")
	}
	t.Setenv("DB_HOST", "db.internal")
	path := filepath.Join(t.TempDir(), "database.toml")
	content := `default = "${DB_DRIVER:-mysql}"

[connections.mysql]
type = "${DB_TYPE:-mysql}"
hostname = "${DB_HOST:-127.0.0.1}"
database = "${DB_NAME:-}"
username = "${DB_USER:-root}"
password = "${DB_PASS:-}"
hostport = "${DB_PORT:-3306}"
charset = "${DB_CHARSET:-utf8mb4}"
prefix = "${DB_PREFIX:-}"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := tdb.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := cfg.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if connection.Driver != "mysql" || !strings.Contains(connection.DSN, "tcp(db.internal:3306)/") {
		t.Fatalf("unexpected TP default connection: %+v", connection)
	}
}

func TestLoadConfigAcceptsNumericINIHostport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.ini")
	content := `default=mysql

[connections.mysql]
type=mysql
hostname=127.0.0.1
hostport=3306
database=demo
username=root
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := tdb.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := cfg.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(connection.DSN, "tcp(127.0.0.1:3306)/demo") {
		t.Fatalf("unexpected DSN: %s", connection.DSN)
	}
}

func TestLoadConfigAllowsMissingOptionalFile(t *testing.T) {
	cfg, err := tdb.LoadConfig(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Resolve(); err == nil {
		t.Fatal("expected missing default connection error")
	}
}
