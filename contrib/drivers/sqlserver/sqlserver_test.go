package sqlserver

import (
	"database/sql"
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("sqlserver")
	if !ok {
		t.Fatal("sqlserver driver is not registered")
	}
	if driver.Name() != "sqlserver" {
		t.Fatalf("Name() = %q", driver.Name())
	}
	cap := driver.Capabilities()
	if !cap.Returning || !cap.Savepoint || !cap.NamedParameters {
		t.Fatalf("capabilities = %+v", cap)
	}
	if cap.Upsert || cap.LastInsertID {
		t.Fatalf("unexpected capabilities = %+v", cap)
	}
}

func TestSqlserverDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("sqlserver")
	d := driver.Dialect()
	if got := d.Name(); got != "sqlserver" {
		t.Fatalf("Name() = %q", got)
	}
	if got := d.Quote("users.id"); got != `"users"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
	if got := d.Placeholder(0); got != "?" {
		t.Fatalf("Placeholder() = %q", got)
	}
}

func TestValueOf(t *testing.T) {
	if got := valueOf(sql.NullString{String: " hello ", Valid: true}); got != "hello" {
		t.Fatalf("valueOf = %q", got)
	}
	if got := valueOf(sql.NullString{Valid: false}); got != "" {
		t.Fatalf("valueOf(null) = %q", got)
	}
}
