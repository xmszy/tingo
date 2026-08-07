package postgres

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("postgres")
	if !ok {
		t.Fatal("postgres driver is not registered")
	}
	if driver.Name() != "postgres" {
		t.Fatalf("Name() = %q", driver.Name())
	}
	cap := driver.Capabilities()
	if !cap.Returning || !cap.Upsert || !cap.Savepoint {
		t.Fatalf("capabilities = %+v", cap)
	}
	if cap.LastInsertID || cap.NamedParameters {
		t.Fatalf("unexpected capabilities = %+v", cap)
	}
}

func TestPostgresDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("postgres")
	d := driver.Dialect()
	if got := d.Name(); got != "postgres" {
		t.Fatalf("Name() = %q", got)
	}
	if got := d.Quote("users.id"); got != `"users"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
	if got := d.Placeholder(0); got != "$1" {
		t.Fatalf("Placeholder() = %q", got)
	}
}
