package dm

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndDialect(t *testing.T) {
	driver, ok := tdb.DriverFor("dm")
	if !ok {
		t.Fatal("dm driver is not registered")
	}
	if driver.Metadata() == nil || !driver.Capabilities().Metadata {
		t.Fatalf("invalid metadata capability: %+v", driver.Capabilities())
	}
	if got := driver.Dialect().Quote("users.id"); got != `"users"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
	if got := driver.Dialect().Placeholder(0); got != "?" {
		t.Fatalf("Placeholder() = %q", got)
	}
}

func TestDMLimit(t *testing.T) {
	if got := dmLimit(10, 20); got != " OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY" {
		t.Fatalf("dmLimit() = %q", got)
	}
}
