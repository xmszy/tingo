package mysql

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{}

type dialect struct{ tdb.Dialect }

func (d dialect) UpsertClause(spec tdb.UpsertSpec) (string, error) {
	if len(spec.UpdateColumns) == 0 {
		return "", fmt.Errorf("mysql: upsert requires at least one update column")
	}
	assignments := make([]string, len(spec.UpdateColumns))
	for i, column := range spec.UpdateColumns {
		quoted := d.Quote(column)
		assignments[i] = quoted + " = VALUES(" + quoted + ")"
	}
	return " ON DUPLICATE KEY UPDATE " + strings.Join(assignments, ", "), nil
}

func init() {
	base, _ := tdb.DialectFor("mysql")
	tdb.RegisterDialect(dialect{Dialect: base})
	tdb.RegisterSchemaDriver(Driver{}.Name(), Driver{})
	tdb.MustRegisterDriver(tdb.NewDriverWithConnector("mysql", tdb.SQLConnector("mysql"), Driver{}, tdb.Capabilities{
		Upsert: true, Savepoint: true, LastInsertID: true,
	}))
}

func (Driver) Name() string { return "mysql" }

func (Driver) Columns(db *tdb.DB, table, schema string) ([]tdb.Column, error) {
	const query = `SELECT column_name, column_type, is_nullable, column_key,
		column_comment, column_default, extra
		FROM information_schema.columns
		WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND table_name = ?
		ORDER BY ordinal_position`
	rows, err := db.SQL().Query(query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]tdb.Column, 0)
	for rows.Next() {
		var column tdb.Column
		var nullable string
		var key, comment, defaultValue, extra sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &key, &comment, &defaultValue, &extra); err != nil {
			return nil, err
		}
		column.Nullable = strings.EqualFold(nullable, "YES")
		column.Key = valueOf(key)
		column.Comment = valueOf(comment)
		column.Default = valueOf(defaultValue)
		column.Extra = valueOf(extra)
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (Driver) Tables(db *tdb.DB, schema string) ([]string, error) {
	const query = `SELECT table_name FROM information_schema.tables
		WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND table_type = 'BASE TABLE'
		ORDER BY table_name`
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
	const query = `SELECT table_name, table_schema, table_comment, table_type
		FROM information_schema.tables
		WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND table_name = ?`
	var descriptor tdb.Table
	if err := db.SQL().QueryRow(query, schema, table).Scan(
		&descriptor.Name, &descriptor.Schema, &descriptor.Comment, &descriptor.Type,
	); err != nil {
		return tdb.Table{}, err
	}
	return descriptor, nil
}

func (Driver) Indexes(db *tdb.DB, table, schema string) ([]tdb.Index, error) {
	const query = `SELECT index_name, column_name, non_unique = 0, index_name = 'PRIMARY'
		FROM information_schema.statistics
		WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND table_name = ?
		ORDER BY index_name = 'PRIMARY' DESC, index_name, seq_in_index`
	return tdb.QueryIndexes(db, query, schema, table)
}

func valueOf(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
