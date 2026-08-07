package mysql

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("mysql")
	if !ok {
		t.Fatal("mysql driver is not registered")
	}
	if driver.Name() != "mysql" {
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

func TestMysqlDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("mysql")
	d := driver.Dialect()
	if got := d.Name(); got != "mysql" {
		t.Fatalf("Name() = %q", got)
	}
	if got := d.Quote("users.id"); got != "`users`.`id`" {
		t.Fatalf("Quote() = %q", got)
	}
	if got := d.Placeholder(0); got != "?" {
		t.Fatalf("Placeholder() = %q", got)
	}
}
