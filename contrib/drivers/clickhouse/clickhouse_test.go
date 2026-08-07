package clickhouse

import (
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestDriverRegistrationAndCapabilities(t *testing.T) {
	driver, ok := tdb.DriverFor("clickhouse")
	if !ok {
		t.Fatal("clickhouse driver is not registered")
	}
	capabilities := driver.Capabilities()
	if !capabilities.Metadata || !capabilities.SortingKeyMetadata || !capabilities.SkippingIndexMetadata {
		t.Fatalf("metadata capabilities = %+v", capabilities)
	}
	if capabilities.Returning || capabilities.Upsert || capabilities.Savepoint || capabilities.LastInsertID {
		t.Fatalf("unsupported capability advertised: %+v", capabilities)
	}
	if got := driver.Dialect().Quote("events.id"); got != "`events`.`id`" {
		t.Fatalf("Quote() = %q", got)
	}
}

func TestTableKeysPreserveClickHouseSemantics(t *testing.T) {
	indexes := tableKeys("tenant_id, id", "tenant_id, created_at")
	if len(indexes) != 2 {
		t.Fatalf("indexes = %#v", indexes)
	}
	primary, sorting := indexes[0], indexes[1]
	if primary.Kind != tdb.IndexKindPrimary || !primary.Primary || primary.Unique || primary.Expression != "tenant_id, id" {
		t.Fatalf("primary key = %#v", primary)
	}
	if sorting.Kind != tdb.IndexKindSorting || sorting.Primary || sorting.Unique || sorting.Expression != "tenant_id, created_at" {
		t.Fatalf("sorting key = %#v", sorting)
	}
	if got := tableKeys("", ""); len(got) != 0 {
		t.Fatalf("empty keys = %#v", got)
	}
}

func TestNormalizeType(t *testing.T) {
	cases := []struct {
		value    string
		wantType string
		nullable bool
	}{
		{"UInt64", "UInt64", false},
		{"Nullable(String)", "String", true},
		{" LowCardinality(Nullable(String)) ", "LowCardinality(String)", true},
		{"LowCardinality(String)", "LowCardinality(String)", false},
	}
	for _, test := range cases {
		gotType, gotNullable := normalizeType(test.value)
		if gotType != test.wantType || gotNullable != test.nullable {
			t.Fatalf("normalizeType(%q) = (%q, %v), want (%q, %v)", test.value, gotType, gotNullable, test.wantType, test.nullable)
		}
	}
}
