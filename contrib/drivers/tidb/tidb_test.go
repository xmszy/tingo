package tidb

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("tidb")
	if !ok {
		t.Fatal("tidb driver is not registered")
	}
	if driver.Name() != "tidb" {
		t.Fatalf("Name() = %q", driver.Name())
	}
	cap := driver.Capabilities()
	if !cap.Upsert || !cap.Savepoint || !cap.LastInsertID {
		t.Fatalf("capabilities = %+v", cap)
	}
	if cap.Returning || cap.NamedParameters {
		t.Fatalf("unexpected capabilities = %+v", cap)
	}
}

func TestInheritsMysqlDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("tidb")
	d := driver.Dialect()
	if got := d.Name(); got != "mysql" {
		t.Fatalf("dialect Name() = %q, want \"mysql\"", got)
	}
	if got := d.Quote("t.id"); got != "`t`.`id`" {
		t.Fatalf("Quote() = %q", got)
	}
}
