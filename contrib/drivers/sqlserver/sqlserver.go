package sqlserver

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{}

type dialect struct{ tdb.Dialect }

func (d dialect) ReturningClause(columns []string) (tdb.ReturningClause, error) {
	if len(columns) == 0 {
		return tdb.ReturningClause{}, fmt.Errorf("sqlserver: output requires at least one column")
	}
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = "INSERTED." + d.Quote(column)
	}
	return tdb.ReturningClause{SQL: " OUTPUT " + strings.Join(quoted, ", "), Position: tdb.ReturningBeforeValues}, nil
}

func init() {
	base, _ := tdb.DialectFor("sqlserver")
	tdb.RegisterDialect(dialect{Dialect: base})
	tdb.RegisterSchemaDriver(Driver{}.Name(), Driver{})
	tdb.MustRegisterDriver(tdb.NewDriverWithConnector("sqlserver", tdb.SQLConnector("sqlserver"), Driver{}, tdb.Capabilities{
		Returning: true, Savepoint: true, NamedParameters: true,
	}))
}

func (Driver) Name() string { return "sqlserver" }

func (Driver) Columns(db *tdb.DB, table, schema string) ([]tdb.Column, error) {
	if schema == "" {
		schema = "dbo"
	}
	const query = `SELECT c.name,
		CASE
			WHEN typ.name IN ('decimal', 'numeric') THEN typ.name + '(' + CONVERT(varchar(10), c.precision) + ',' + CONVERT(varchar(10), c.scale) + ')'
			WHEN typ.name IN ('char', 'varchar', 'binary', 'varbinary') THEN typ.name + '(' + CASE WHEN c.max_length = -1 THEN 'max' ELSE CONVERT(varchar(10), c.max_length) END + ')'
			WHEN typ.name IN ('nchar', 'nvarchar') THEN typ.name + '(' + CASE WHEN c.max_length = -1 THEN 'max' ELSE CONVERT(varchar(10), c.max_length / 2) END + ')'
			ELSE typ.name
		END,
		c.is_nullable,
		CASE WHEN pk.column_id IS NULL THEN '' ELSE 'PRI' END,
		CONVERT(nvarchar(max), ep.value), dc.definition, c.is_identity
		FROM sys.columns c
		JOIN sys.objects obj ON obj.object_id = c.object_id AND obj.type = 'U'
		JOIN sys.schemas sch ON sch.schema_id = obj.schema_id
		JOIN sys.types typ ON typ.user_type_id = c.user_type_id
		LEFT JOIN sys.default_constraints dc ON dc.object_id = c.default_object_id
		LEFT JOIN sys.extended_properties ep
		  ON ep.major_id = c.object_id AND ep.minor_id = c.column_id AND ep.name = 'MS_Description'
		LEFT JOIN (
			SELECT ic.object_id, ic.column_id FROM sys.index_columns ic
			JOIN sys.indexes idx ON idx.object_id = ic.object_id AND idx.index_id = ic.index_id
			WHERE idx.is_primary_key = 1
		) pk ON pk.object_id = c.object_id AND pk.column_id = c.column_id
		WHERE sch.name = @schema AND obj.name = @table
		ORDER BY c.column_id`
	rows, err := db.SQL().Query(query, sql.Named("schema", schema), sql.Named("table", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]tdb.Column, 0)
	for rows.Next() {
		var column tdb.Column
		var nullable, identity bool
		var key string
		var comment, defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &key, &comment, &defaultValue, &identity); err != nil {
			return nil, err
		}
		column.Nullable = nullable
		column.Key = key
		column.Comment = valueOf(comment)
		column.Default = valueOf(defaultValue)
		if identity {
			column.Extra = "identity"
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (Driver) Tables(db *tdb.DB, schema string) ([]string, error) {
	if schema == "" {
		schema = "dbo"
	}
	const query = `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = @schema AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`
	rows, err := db.SQL().Query(query, sql.Named("schema", schema))
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
	if schema == "" {
		schema = "dbo"
	}
	const query = `SELECT obj.name, sch.name, CONVERT(nvarchar(max), ep.value), 'BASE TABLE'
		FROM sys.objects obj
		JOIN sys.schemas sch ON sch.schema_id = obj.schema_id
		LEFT JOIN sys.extended_properties ep
		  ON ep.major_id = obj.object_id AND ep.minor_id = 0 AND ep.name = 'MS_Description'
		WHERE sch.name = @schema AND obj.name = @table AND obj.type = 'U'`
	var descriptor tdb.Table
	var comment sql.NullString
	if err := db.SQL().QueryRow(query, sql.Named("schema", schema), sql.Named("table", table)).Scan(
		&descriptor.Name, &descriptor.Schema, &comment, &descriptor.Type,
	); err != nil {
		return tdb.Table{}, err
	}
	descriptor.Comment = valueOf(comment)
	return descriptor, nil
}

func (Driver) Indexes(db *tdb.DB, table, schema string) ([]tdb.Index, error) {
	if schema == "" {
		schema = "dbo"
	}
	const query = `SELECT idx.name, col.name, idx.is_unique, idx.is_primary_key
		FROM sys.indexes idx
		JOIN sys.objects obj ON obj.object_id = idx.object_id AND obj.type = 'U'
		JOIN sys.schemas sch ON sch.schema_id = obj.schema_id
		JOIN sys.index_columns pos ON pos.object_id = idx.object_id AND pos.index_id = idx.index_id
		JOIN sys.columns col ON col.object_id = pos.object_id AND col.column_id = pos.column_id
		WHERE sch.name = @schema AND obj.name = @table AND idx.is_hypothetical = 0
		ORDER BY idx.is_primary_key DESC, idx.name, pos.key_ordinal`
	return tdb.QueryIndexes(db, query, sql.Named("schema", schema), sql.Named("table", table))
}

func valueOf(value sql.NullString) string {
	if value.Valid {
		return strings.TrimSpace(value.String)
	}
	return ""
}
