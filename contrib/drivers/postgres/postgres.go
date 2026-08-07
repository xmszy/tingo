package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{}

type dialect struct{ tdb.Dialect }

func (d dialect) UpsertClause(spec tdb.UpsertSpec) (string, error) {
	if len(spec.ConflictColumns) == 0 {
		return "", fmt.Errorf("postgres: upsert requires at least one conflict column")
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
		assignments[i] = quoted + " = EXCLUDED." + quoted
	}
	return clause + " DO UPDATE SET " + strings.Join(assignments, ", "), nil
}

func (d dialect) ReturningClause(columns []string) (tdb.ReturningClause, error) {
	if len(columns) == 0 {
		return tdb.ReturningClause{}, fmt.Errorf("postgres: returning requires at least one column")
	}
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = d.Quote(column)
	}
	return tdb.ReturningClause{SQL: " RETURNING " + strings.Join(quoted, ", "), Position: tdb.ReturningSuffix}, nil
}

func init() {
	base, _ := tdb.DialectFor("postgres")
	tdb.RegisterDialect(dialect{Dialect: base})
	tdb.RegisterSchemaDriver(Driver{}.Name(), Driver{})
	tdb.MustRegisterDriver(tdb.NewDriverWithConnector("postgres", tdb.SQLConnector("pgx"), Driver{}, tdb.Capabilities{
		Returning: true, Upsert: true, Savepoint: true,
	}))
}

func (Driver) Name() string { return "postgres" }

func (Driver) Columns(db *tdb.DB, table, schema string) ([]tdb.Column, error) {
	primaryKeys, err := loadPrimaryKeys(db, table, schema)
	if err != nil {
		return nil, err
	}
	const query = `SELECT c.column_name,
		CASE
			WHEN c.character_maximum_length IS NOT NULL THEN c.udt_name || '(' || c.character_maximum_length || ')'
			WHEN c.numeric_precision IS NOT NULL AND c.numeric_scale IS NOT NULL THEN c.udt_name || '(' || c.numeric_precision || ',' || c.numeric_scale || ')'
			ELSE c.udt_name
		END,
		c.is_nullable, c.column_default,
		COALESCE(pg_catalog.col_description(cls.oid, c.ordinal_position), ''),
		c.is_identity
		FROM information_schema.columns c
		JOIN pg_catalog.pg_namespace ns ON ns.nspname = c.table_schema
		JOIN pg_catalog.pg_class cls ON cls.relnamespace = ns.oid AND cls.relname = c.table_name
		WHERE c.table_schema = COALESCE(NULLIF($1, ''), current_schema()) AND c.table_name = $2
		ORDER BY c.ordinal_position`
	rows, err := db.SQL().Query(query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]tdb.Column, 0)
	for rows.Next() {
		var column tdb.Column
		var nullable, identity string
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &defaultValue, &comment, &identity); err != nil {
			return nil, err
		}
		column.Nullable = strings.EqualFold(nullable, "YES")
		column.Default = valueOf(defaultValue)
		column.Comment = valueOf(comment)
		if primaryKeys[column.Name] {
			column.Key = "PRI"
		}
		if strings.EqualFold(identity, "YES") || strings.Contains(strings.ToLower(column.Default), "nextval(") {
			column.Extra = "auto_increment"
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func loadPrimaryKeys(db *tdb.DB, table, schema string) (map[string]bool, error) {
	const query = `SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON kcu.constraint_schema = tc.constraint_schema AND kcu.constraint_name = tc.constraint_name
		WHERE tc.table_schema = COALESCE(NULLIF($1, ''), current_schema())
		  AND tc.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position`
	rows, err := db.SQL().Query(query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		keys[name] = true
	}
	return keys, rows.Err()
}

func (Driver) Tables(db *tdb.DB, schema string) ([]string, error) {
	const query = `SELECT table_name FROM information_schema.tables
		WHERE table_schema = COALESCE(NULLIF($1, ''), current_schema()) AND table_type = 'BASE TABLE'
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
	const query = `SELECT cls.relname, ns.nspname,
		COALESCE(pg_catalog.obj_description(cls.oid, 'pg_class'), ''),
		CASE cls.relkind WHEN 'p' THEN 'PARTITIONED TABLE' ELSE 'BASE TABLE' END
		FROM pg_catalog.pg_class cls
		JOIN pg_catalog.pg_namespace ns ON ns.oid = cls.relnamespace
		WHERE ns.nspname = COALESCE(NULLIF($1, ''), current_schema())
		  AND cls.relname = $2 AND cls.relkind IN ('r', 'p')`
	var descriptor tdb.Table
	if err := db.SQL().QueryRow(query, schema, table).Scan(
		&descriptor.Name, &descriptor.Schema, &descriptor.Comment, &descriptor.Type,
	); err != nil {
		return tdb.Table{}, err
	}
	return descriptor, nil
}

func (Driver) Indexes(db *tdb.DB, table, schema string) ([]tdb.Index, error) {
	const query = `SELECT idx.relname, attr.attname, ind.indisunique, ind.indisprimary
		FROM pg_catalog.pg_index ind
		JOIN pg_catalog.pg_class tbl ON tbl.oid = ind.indrelid
		JOIN pg_catalog.pg_namespace ns ON ns.oid = tbl.relnamespace
		JOIN pg_catalog.pg_class idx ON idx.oid = ind.indexrelid
		JOIN LATERAL unnest(ind.indkey) WITH ORDINALITY key(attnum, position) ON true
		JOIN pg_catalog.pg_attribute attr ON attr.attrelid = tbl.oid AND attr.attnum = key.attnum
		WHERE ns.nspname = COALESCE(NULLIF($1, ''), current_schema()) AND tbl.relname = $2
		ORDER BY ind.indisprimary DESC, idx.relname, key.position`
	return tdb.QueryIndexes(db, query, schema, table)
}

func valueOf(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
