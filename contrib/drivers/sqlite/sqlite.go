package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/xmszy/tingo/database/tdb"
	_ "modernc.org/sqlite"
)

type Driver struct{}

type dialect struct{ tdb.Dialect }

func (d dialect) UpsertClause(spec tdb.UpsertSpec) (string, error) {
	if len(spec.ConflictColumns) == 0 {
		return "", fmt.Errorf("sqlite: upsert requires at least one conflict column")
	}
	conflicts := make([]string, len(spec.ConflictColumns))
	for i, column := range spec.ConflictColumns {
		conflicts[i] = d.Quote(column)
	}
	clause := " ON CONFLICT (" + strings.Join(conflicts, ", ") + ")"
	if len(spec.UpdateColumns) == 0 {
		return clause + " DO NOTHING", nil
	}
	assignments := make([]string, len(spec.UpdateColumns))
	for i, column := range spec.UpdateColumns {
		quoted := d.Quote(column)
		assignments[i] = quoted + " = excluded." + quoted
	}
	return clause + " DO UPDATE SET " + strings.Join(assignments, ", "), nil
}

func (d dialect) ReturningClause(columns []string) (tdb.ReturningClause, error) {
	if len(columns) == 0 {
		return tdb.ReturningClause{}, fmt.Errorf("sqlite: returning requires at least one column")
	}
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = d.Quote(column)
	}
	return tdb.ReturningClause{SQL: " RETURNING " + strings.Join(quoted, ", "), Position: tdb.ReturningSuffix}, nil
}

func init() {
	base, _ := tdb.DialectFor("sqlite")
	tdb.RegisterDialect(dialect{Dialect: base})
	tdb.RegisterSchemaDriver(Driver{}.Name(), Driver{})
	tdb.MustRegisterDriver(tdb.NewDriverWithConnector("sqlite", tdb.SQLConnector("sqlite"), Driver{}, tdb.Capabilities{
		Returning: true, Upsert: true, Savepoint: true, LastInsertID: true,
	}))
}

func (Driver) Name() string { return "sqlite" }

func (Driver) Columns(db *tdb.DB, table, schema string) ([]tdb.Column, error) {
	schema = sqliteSchema(schema)
	query := `PRAGMA ` + quoteSQLiteIdentifier(schema) + `.table_info(` + quoteSQLiteIdentifier(table) + `)`
	rows, err := db.SQL().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]tdb.Column, 0)
	primaryCount := 0
	for rows.Next() {
		var column tdb.Column
		var index, notNull, primaryPosition int
		var defaultValue sql.NullString
		if err := rows.Scan(&index, &column.Name, &column.Type, &notNull, &defaultValue, &primaryPosition); err != nil {
			return nil, err
		}
		column.Nullable = notNull == 0 && primaryPosition == 0
		column.Default = valueOf(defaultValue)
		if primaryPosition > 0 {
			column.Key = "PRI"
			primaryCount++
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if primaryCount == 1 {
		for i := range columns {
			if columns[i].IsPrimary() && strings.EqualFold(strings.TrimSpace(columns[i].Type), "INTEGER") {
				columns[i].Extra = "auto_increment"
			}
		}
	}
	return columns, nil
}

func (Driver) Tables(db *tdb.DB, schema string) ([]string, error) {
	schema = sqliteSchema(schema)
	query := `SELECT name FROM ` + quoteSQLiteIdentifier(schema) + `.sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	rows, err := db.SQL().Query(query)
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
	schema = sqliteSchema(schema)
	query := `SELECT name, upper(type) FROM ` + quoteSQLiteIdentifier(schema) + `.sqlite_master
		WHERE type = 'table' AND name = ?`
	descriptor := tdb.Table{Schema: schema}
	if err := db.SQL().QueryRow(query, table).Scan(&descriptor.Name, &descriptor.Type); err != nil {
		return tdb.Table{}, err
	}
	return descriptor, nil
}

func sqliteSchema(schema string) string {
	if schema = strings.TrimSpace(schema); schema != "" {
		return schema
	}
	return "main"
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (Driver) Indexes(db *tdb.DB, table, _ string) ([]tdb.Index, error) {
	const query = `SELECT indexes.name, fields.name, indexes."unique" != 0, indexes.origin = 'pk'
		FROM pragma_index_list(?) indexes
		JOIN pragma_index_info(indexes.name) fields
		ORDER BY indexes.origin = 'pk' DESC, indexes.name, fields.seqno`
	return tdb.QueryIndexes(db, query, table)
}

func valueOf(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
