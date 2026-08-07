package oceanbase

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("oceanbase")
	if !ok {
		t.Fatal("oceanbase driver is not registered")
	}
	if driver.Name() != "oceanbase" {
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
	driver, _ := tdb.DriverFor("oceanbase")
	d := driver.Dialect()
	if got := d.Name(); got != "mysql" {
		t.Fatalf("dialect Name() = %q, want \"mysql\"", got)
	}
	if got := d.Quote("t.id"); got != "`t`.`id`" {
		t.Fatalf("Quote() = %q", got)
	}
}
