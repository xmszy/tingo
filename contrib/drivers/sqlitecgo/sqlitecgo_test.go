package sqlitecgo

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("sqlitecgo")
	if !ok {
		t.Fatal("sqlitecgo driver is not registered")
	}
	if driver.Name() != "sqlitecgo" {
		t.Fatalf("Name() = %q", driver.Name())
	}
	cap := driver.Capabilities()
	if !cap.Returning || !cap.Upsert || !cap.Savepoint || !cap.LastInsertID {
		t.Fatalf("capabilities = %+v", cap)
	}
	if cap.NamedParameters {
		t.Fatalf("unexpected capabilities = %+v", cap)
	}
}

func TestInheritsSqliteDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("sqlitecgo")
	d := driver.Dialect()
	if got := d.Name(); got != "sqlite" {
		t.Fatalf("dialect Name() = %q, want \"sqlite\"", got)
	}
	if got := d.Quote("t.id"); got != `"t"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
}
