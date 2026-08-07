//go:build oracle

package oracle

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndDialect(t *testing.T) {
	driver, ok := tdb.DriverFor("oracle")
	if !ok {
		t.Fatal("oracle driver is not registered")
	}
	if driver.Metadata() == nil || !driver.Capabilities().Metadata {
		t.Fatalf("invalid metadata capability: %+v", driver.Capabilities())
	}
	dialect := driver.Dialect()
	if got := dialect.Quote("users.id"); got != `"users"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
	if got := dialect.Placeholder(1); got != ":2" {
		t.Fatalf("Placeholder() = %q", got)
	}
}

func TestOracleLimit(t *testing.T) {
	cases := []struct {
		limit, offset int
		want          string
	}{
		{0, 0, ""},
		{10, 0, " FETCH NEXT 10 ROWS ONLY"},
		{10, 20, " OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY"},
		{0, 20, " OFFSET 20 ROWS"},
	}
	for _, test := range cases {
		if got := oracleLimit(test.limit, test.offset); got != test.want {
			t.Fatalf("oracleLimit(%d, %d) = %q, want %q", test.limit, test.offset, got, test.want)
		}
	}
}
