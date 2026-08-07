package clickhouse

import (
	"database/sql"
	"strings"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{}

func init() {
	dialect := tdb.DialectDefinition{
		DialectName: "clickhouse",
		QuoteLeft:   "`",
		QuoteRight:  "`",
	}
	tdb.RegisterDialect(dialect)
	tdb.RegisterSchemaDriver(Driver{}.Name(), Driver{})
	tdb.MustRegisterDriver(tdb.DriverDefinition{
		DriverName:      "clickhouse",
		DriverConnector: tdb.SQLConnector("clickhouse"),
		DriverDialect:   dialect,
		MetadataDriver:  Driver{},
		DriverCapabilities: tdb.Capabilities{
			Metadata: true, SortingKeyMetadata: true, SkippingIndexMetadata: true,
		},
	})
}

func (Driver) Name() string { return "clickhouse" }

func (Driver) Columns(db *tdb.DB, table, schema string) ([]tdb.Column, error) {
	const query = `SELECT name, type, is_in_primary_key, comment, default_expression
		FROM system.columns
		WHERE database = coalesce(nullIf(?, ''), currentDatabase()) AND table = ?
		ORDER BY position`
	rows, err := db.SQL().Query(query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]tdb.Column, 0)
	for rows.Next() {
		var column tdb.Column
		var primary int64
		var comment, defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &primary, &comment, &defaultValue); err != nil {
			return nil, err
		}
		column.Type, column.Nullable = normalizeType(column.Type)
		if primary != 0 {
			column.Key = "PRI"
		}
		column.Comment = nullString(comment)
		column.Default = nullString(defaultValue)
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (Driver) Tables(db *tdb.DB, schema string) ([]string, error) {
	const query = `SELECT name FROM system.tables
		WHERE database = coalesce(nullIf(?, ''), currentDatabase())
		  AND is_temporary = 0
		  AND engine NOT IN ('View', 'MaterializedView', 'LiveView', 'WindowView')
		ORDER BY name`
	rows, err := db.SQL().Query(query, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (Driver) Table(db *tdb.DB, table, schema string) (tdb.Table, error) {
	const query = `SELECT name, database, comment, engine FROM system.tables
		WHERE database = coalesce(nullIf(?, ''), currentDatabase()) AND name = ?`
	var descriptor tdb.Table
	if err := db.SQL().QueryRow(query, schema, table).Scan(
		&descriptor.Name, &descriptor.Schema, &descriptor.Comment, &descriptor.Type,
	); err != nil {
		return tdb.Table{}, err
	}
	return descriptor, nil
}

func (Driver) Indexes(db *tdb.DB, table, schema string) ([]tdb.Index, error) {
	const keysQuery = `SELECT primary_key, sorting_key FROM system.tables
		WHERE database = coalesce(nullIf(?, ''), currentDatabase()) AND name = ?`
	var primaryKey, sortingKey string
	if err := db.SQL().QueryRow(keysQuery, schema, table).Scan(&primaryKey, &sortingKey); err != nil {
		return nil, err
	}
	indexes := tableKeys(primaryKey, sortingKey)

	const skippingQuery = `SELECT name, expr, type, granularity FROM system.data_skipping_indices
		WHERE database = coalesce(nullIf(?, ''), currentDatabase()) AND table = ?
		ORDER BY name`
	rows, err := db.SQL().Query(skippingQuery, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		index := tdb.Index{Kind: tdb.IndexKindSkipping}
		if err := rows.Scan(&index.Name, &index.Expression, &index.Type, &index.Granularity); err != nil {
			return nil, err
		}
		indexes = append(indexes, index)
	}
	return indexes, rows.Err()
}

func tableKeys(primaryKey, sortingKey string) []tdb.Index {
	indexes := make([]tdb.Index, 0, 2)
	if primaryKey = strings.TrimSpace(primaryKey); primaryKey != "" {
		indexes = append(indexes, tdb.Index{
			Name: "PRIMARY KEY", Kind: tdb.IndexKindPrimary, Primary: true, Expression: primaryKey,
		})
	}
	if sortingKey = strings.TrimSpace(sortingKey); sortingKey != "" {
		indexes = append(indexes, tdb.Index{
			Name: "ORDER BY", Kind: tdb.IndexKindSorting, Expression: sortingKey,
		})
	}
	return indexes
}

func normalizeType(value string) (string, bool) {
	value = strings.TrimSpace(value)
	inner, ok := unwrap(value, "Nullable")
	if ok {
		normalized, _ := normalizeType(inner)
		return normalized, true
	}
	inner, ok = unwrap(value, "LowCardinality")
	if ok {
		normalized, nullable := normalizeType(inner)
		return "LowCardinality(" + normalized + ")", nullable
	}
	return value, false
}

func unwrap(value, wrapper string) (string, bool) {
	prefix := wrapper + "("
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ")") {
		return "", false
	}
	return strings.TrimSpace(value[len(prefix) : len(value)-1]), true
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return strings.TrimSpace(value.String)
	}
	return ""
}
