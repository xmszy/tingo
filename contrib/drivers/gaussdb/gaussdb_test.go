package gaussdb

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("gaussdb")
	if !ok {
		t.Fatal("gaussdb driver is not registered")
	}
	if driver.Name() != "gaussdb" {
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

func TestInheritsPostgresDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("gaussdb")
	d := driver.Dialect()
	if got := d.Name(); got != "postgres" {
		t.Fatalf("dialect Name() = %q, want \"postgres\"", got)
	}
	if got := d.Quote("t.id"); got != `"t"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
}
