package tdb

import (
	"reflect"
	"testing"
)

func TestSchemaDriverRegistry(t *testing.T) {
	driver := customSchema{}
	name := "custom_metadata"
	RegisterSchemaDriver(name, driver)

	got, ok := SchemaDriverFor(name)
	if !ok {
		t.Fatal("registered schema driver was not found")
	}
	_ = got // driver found
}

func TestUnknownSchemaDriverDoesNotFallback(t *testing.T) {
	if _, ok := SchemaDriverFor("not-registered"); ok {
		t.Fatal("unknown schema driver unexpectedly resolved")
	}
}

func TestRegisterSchemaDriverRejectsInvalidDriver(t *testing.T) {
	assertPanic(t, func() { RegisterSchemaDriver("nil", nil) })
}

func TestColumnMetadata(t *testing.T) {
	primary := Column{Name: "id", Key: "pri", Extra: "IDENTITY(1,1)"}
	if !primary.IsPrimary() {
		t.Fatal("primary key was not recognized")
	}
	if !primary.IsAutoIncrement() {
		t.Fatal("identity column was not recognized as auto increment")
	}

	meta := &TableMeta{Columns: []Column{
		{Name: "tenant_id", Key: "PRI"},
		{Name: "id", Key: "pri"},
		{Name: "name"},
	}}
	keys := meta.PrimaryKeys()
	if len(keys) != 2 || keys[0] != "tenant_id" || keys[1] != "id" {
		t.Fatalf("primary keys = %v", keys)
	}
}

func TestQueryIndexesAggregatesCompositeIndexes(t *testing.T) {
	const connection = "query-indexes"
	seedTable(connection, "indexes",
		map[string]any{"index_name": "PRIMARY", "column_name": "tenant_id", "unique": true, "primary": true},
		map[string]any{"index_name": "PRIMARY", "column_name": "id", "unique": true, "primary": true},
		map[string]any{"index_name": "users_email", "column_name": "email", "unique": true, "primary": false},
		map[string]any{"index_name": "users_name_created", "column_name": "name", "unique": false, "primary": false},
		map[string]any{"index_name": "users_name_created", "column_name": "created_at", "unique": false, "primary": false},
	)
	db := openIndexTestDB(t, connection)
	defer db.Close()

	got, err := QueryIndexes(db, "SELECT index_name, column_name, unique, primary FROM indexes")
	if err != nil {
		t.Fatal(err)
	}
	want := []Index{
		{Name: "PRIMARY", Columns: []string{"tenant_id", "id"}, Unique: true, Primary: true, Kind: IndexKindPrimary},
		{Name: "users_email", Columns: []string{"email"}, Unique: true, Kind: IndexKindUnique},
		{Name: "users_name_created", Columns: []string{"name", "created_at"}, Kind: IndexKindRegular},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("indexes = %#v, want %#v", got, want)
	}
}

func TestQueryIndexesEmptyResult(t *testing.T) {
	const connection = "query-indexes-empty"
	seedTable(connection, "indexes")
	db := openIndexTestDB(t, connection)
	defer db.Close()

	indexes, err := QueryIndexes(db, "SELECT index_name, column_name, unique, primary FROM indexes")
	if err != nil {
		t.Fatal(err)
	}
	if len(indexes) != 0 {
		t.Fatalf("indexes = %#v, want empty result", indexes)
	}
}

func TestQueryIndexesReturnsScanError(t *testing.T) {
	const connection = "query-indexes-scan-error"
	seedTable(connection, "indexes",
		map[string]any{"index_name": "users_email", "column_name": "email", "unique": "invalid", "primary": false},
	)
	db := openIndexTestDB(t, connection)
	defer db.Close()

	if _, err := QueryIndexes(db, "SELECT index_name, column_name, unique, primary FROM indexes"); err == nil {
		t.Fatal("expected scan error")
	}
}

func openIndexTestDB(t *testing.T, connection string) *DB {
	t.Helper()
	db, err := Open(Config{Driver: "tdb_mem", DSN: connection, Dialect: "mysql"})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestExtendedTableMetadata(t *testing.T) {
	meta := &TableMeta{
		Name:   "users",
		Schema: "public",
		Columns: []Column{
			{Name: "tenant_id", Key: "PRI"},
			{Name: "id", Key: "PRI"},
		},
		Indexes: []Index{{Name: "users_name", Columns: []string{"name"}}},
	}
	if hasPrimaryIndex(meta.Indexes) {
		t.Fatal("non-primary index was classified as primary")
	}
	keys := meta.PrimaryKeys()
	if !reflect.DeepEqual(keys, []string{"tenant_id", "id"}) {
		t.Fatalf("column primary keys = %v", keys)
	}
	meta.Indexes = append([]Index{{Name: "PRIMARY", Columns: []string{"id", "tenant_id"}, Unique: true, Primary: true, Kind: IndexKindPrimary}}, meta.Indexes...)
	if !hasPrimaryIndex(meta.Indexes) || !reflect.DeepEqual(meta.PrimaryKeys(), []string{"id", "tenant_id"}) {
		t.Fatalf("indexes = %+v", meta.Indexes)
	}
}

func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

type customSchema struct{}

func (customSchema) Columns(*DB, string, string) ([]Column, error) { return nil, nil }
func (customSchema) Tables(*DB, string) ([]string, error)          { return nil, nil }
