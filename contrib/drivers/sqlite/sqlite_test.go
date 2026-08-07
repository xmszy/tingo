package sqlite

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("sqlite")
	if !ok {
		t.Fatal("sqlite driver is not registered")
	}
	if driver.Name() != "sqlite" {
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

func TestSqliteDialect(t *testing.T) {
	driver, _ := tdb.DriverFor("sqlite")
	d := driver.Dialect()
	if got := d.Name(); got != "sqlite" {
		t.Fatalf("Name() = %q", got)
	}
	if got := d.Quote("users.id"); got != `"users"."id"` {
		t.Fatalf("Quote() = %q", got)
	}
	if got := d.Placeholder(0); got != "?" {
		t.Fatalf("Placeholder() = %q", got)
	}
}

func TestQuoteSQLiteIdentifier(t *testing.T) {
	if got := quoteSQLiteIdentifier("my_table"); got != `"my_table"` {
		t.Fatalf("quoteSQLiteIdentifier = %q", got)
	}
	if got := quoteSQLiteIdentifier(`bad"name`); got != `"bad""name"` {
		t.Fatalf("quoteSQLiteIdentifier escaped = %q", got)
	}
}

func TestSqliteSchema(t *testing.T) {
	if got := sqliteSchema(""); got != "main" {
		t.Fatalf("sqliteSchema(\"\") = %q", got)
	}
	if got := sqliteSchema("  "); got != "main" {
		t.Fatalf("sqliteSchema(\"  \") = %q", got)
	}
	if got := sqliteSchema("mydb"); got != "mydb" {
		t.Fatalf("sqliteSchema(\"mydb\") = %q", got)
	}
}
