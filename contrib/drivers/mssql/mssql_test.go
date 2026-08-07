package mssql

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("mssql")
	if !ok {
		t.Fatal("mssql driver is not registered")
	}
	if driver.Name() != "mssql" {
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

func TestInheritsSqlserverDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("mssql")
	d := driver.Dialect()
	if got := d.Name(); got != "sqlserver" {
		t.Fatalf("dialect Name() = %q, want \"sqlserver\"", got)
	}
	if got := d.Quote("t.id"); got != `"t"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
}
