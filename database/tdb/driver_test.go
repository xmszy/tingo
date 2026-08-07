package tdb

import (
	"errors"
	"testing"
)

func TestDatabaseDriverRegistryAndCapabilities(t *testing.T) {
	dialect, ok := DialectFor("mysql")
	if !ok {
		t.Fatal("mysql dialect missing")
	}
	driver := DriverDefinition{
		DriverName:         "contract-test",
		DriverDialect:      dialect,
		MetadataDriver:     customSchema{},
		DriverCapabilities: Capabilities{Upsert: true, Metadata: true},
	}
	if err := RegisterDriver(driver); err != nil {
		t.Fatal(err)
	}
	got, ok := DriverFor(" CONTRACT-TEST ")
	if !ok {
		t.Fatal("registered database driver not found")
	}
	if !got.Capabilities().Upsert || !got.Capabilities().Metadata {
		t.Fatalf("capabilities = %+v", got.Capabilities())
	}
	if err := RegisterDriver(driver); err == nil {
		t.Fatal("expected duplicate driver error")
	}
}

func TestCapabilitiesSupportAndError(t *testing.T) {
	capabilities := Capabilities{Upsert: true, Metadata: true, SortingKeyMetadata: true}
	if !capabilities.Supports(CapabilityUpsert) || !capabilities.Supports(CapabilityMetadata) || !capabilities.Supports(CapabilitySortingKeyMetadata) {
		t.Fatalf("supported capabilities were rejected: %+v", capabilities)
	}
	if capabilities.Supports(CapabilityReturning) || capabilities.Supports(Capability("unknown")) {
		t.Fatal("unsupported capability was accepted")
	}

	dialect, _ := DialectFor("mysql")
	db := &DB{dial: dialect, driver: DriverDefinition{
		DriverName:         "capability-test",
		DriverDialect:      dialect,
		DriverCapabilities: capabilities,
	}}
	if err := db.RequireCapability(CapabilityUpsert); err != nil {
		t.Fatal(err)
	}
	err := db.RequireCapability(CapabilityReturning)
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("error = %v", err)
	}
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Driver != "capability-test" {
		t.Fatalf("capability error = %#v", capabilityErr)
	}
}

func TestDialectFeatureEntryPoints(t *testing.T) {
	base, _ := DialectFor("mysql")
	dialect := featureDialect{Dialect: base}
	db := &DB{dial: dialect, driver: DriverDefinition{
		DriverName: "feature-test", DriverDialect: dialect,
		DriverCapabilities: Capabilities{Upsert: true, Returning: true},
	}}
	upsert, err := db.BuildUpsertClause(UpsertSpec{ConflictColumns: []string{"id"}, UpdateColumns: []string{"name"}})
	if err != nil || upsert != " upsert" {
		t.Fatalf("BuildUpsertClause() = %q, %v", upsert, err)
	}
	returning, err := db.BuildReturningClause("id")
	if err != nil || returning.SQL != " returning" || returning.Position != ReturningBeforeValues {
		t.Fatalf("BuildReturningClause() = %#v, %v", returning, err)
	}
}

func TestDialectFeatureEntryPointsRejectUnsupportedCapability(t *testing.T) {
	base, _ := DialectFor("mysql")
	db := &DB{dial: base, driver: DriverDefinition{DriverName: "unsupported-test", DriverDialect: base}}
	if _, err := db.BuildUpsertClause(UpsertSpec{}); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("BuildUpsertClause() error = %v", err)
	}
	if _, err := db.BuildReturningClause("id"); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("BuildReturningClause() error = %v", err)
	}
}

type featureDialect struct{ Dialect }

func (featureDialect) UpsertClause(UpsertSpec) (string, error) { return " upsert", nil }
func (featureDialect) ReturningClause([]string) (ReturningClause, error) {
	return ReturningClause{SQL: " returning", Position: ReturningBeforeValues}, nil
}

func TestOpenRejectsUnknownDialect(t *testing.T) {
	if _, err := Open(Config{Driver: "missing", Dialect: "missing"}); err == nil {
		t.Fatal("expected unknown dialect error")
	}
}

func TestDialectDefinitionDefaultsAndOverrides(t *testing.T) {
	dialect := DialectDefinition{
		DialectName: "custom",
		QuoteLeft:   "[",
		QuoteRight:  "]",
		PlaceholderFunc: func(index int) string {
			return "@p" + string(rune('1'+index))
		},
		LimitFunc: func(limit, offset int) string {
			return " custom-limit"
		},
	}
	if got := dialect.Quote("users.id"); got != "[users].[id]" {
		t.Fatalf("Quote() = %q", got)
	}
	if got := dialect.Placeholder(1); got != "@p2" {
		t.Fatalf("Placeholder() = %q", got)
	}
	if got := dialect.Limit(10, 2); got != " custom-limit" {
		t.Fatalf("Limit() = %q", got)
	}

	defaults := DialectDefinition{DialectName: "defaults", QuoteLeft: "`", QuoteRight: "`"}
	if got := defaults.Placeholder(0); got != "?" {
		t.Fatalf("default Placeholder() = %q", got)
	}
	if got := defaults.Limit(10, 2); got != " LIMIT 10 OFFSET 2" {
		t.Fatalf("default Limit() = %q", got)
	}
}
