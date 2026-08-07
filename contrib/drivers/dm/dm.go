package dm

import (
	"database/sql"
	"fmt"
	"strings"

	_ "gitee.com/chunanyong/dm"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{}

func init() {
	dialect := tdb.DialectDefinition{
		DialectName: "dm",
		QuoteLeft:   `"`,
		QuoteRight:  `"`,
		LimitFunc:   dmLimit,
	}
	tdb.RegisterDialect(dialect)
	tdb.RegisterSchemaDriver(Driver{}.Name(), Driver{})
	tdb.MustRegisterDriver(tdb.DriverDefinition{
		DriverName:      "dm",
		DriverConnector: tdb.SQLConnector("dm"),
		DriverDialect:   dialect,
		MetadataDriver:  Driver{},
		DriverCapabilities: tdb.Capabilities{
			Savepoint: true, LastInsertID: true, Metadata: true,
		},
	})
}

func (Driver) Name() string { return "dm" }

func (Driver) Columns(db *tdb.DB, table, schema string) ([]tdb.Column, error) {
	const query = `SELECT c.COLUMN_NAME,
		CASE
			WHEN c.DATA_TYPE IN ('NUMBER', 'NUMERIC', 'DECIMAL') AND c.DATA_PRECISION IS NOT NULL AND NVL(c.DATA_SCALE, 0) = 0 THEN c.DATA_TYPE || '(' || c.DATA_PRECISION || ')'
			WHEN c.DATA_TYPE IN ('NUMBER', 'NUMERIC', 'DECIMAL') AND c.DATA_PRECISION IS NOT NULL THEN c.DATA_TYPE || '(' || c.DATA_PRECISION || ',' || c.DATA_SCALE || ')'
			WHEN c.DATA_TYPE IN ('CHAR', 'VARCHAR', 'VARCHAR2', 'NCHAR', 'NVARCHAR', 'NVARCHAR2', 'BINARY', 'VARBINARY') THEN c.DATA_TYPE || '(' || c.DATA_LENGTH || ')'
			ELSE c.DATA_TYPE
		END,
		c.NULLABLE,
		CASE WHEN pk.COLUMN_NAME IS NULL THEN '' ELSE 'PRI' END,
		cc.COMMENTS, c.DATA_DEFAULT,
		CASE WHEN NVL(sc.INFO2, 0) = 1 THEN 'YES' ELSE 'NO' END
		FROM ALL_TAB_COLUMNS c
		LEFT JOIN ALL_COL_COMMENTS cc
		  ON cc.OWNER = c.OWNER AND cc.TABLE_NAME = c.TABLE_NAME AND cc.COLUMN_NAME = c.COLUMN_NAME
		LEFT JOIN SYS.SYSCOLUMNS sc ON sc.ID = c.TABLE_ID AND sc.NAME = c.COLUMN_NAME
		LEFT JOIN (
			SELECT acc.OWNER, acc.TABLE_NAME, acc.COLUMN_NAME
			FROM ALL_CONSTRAINTS ac
			JOIN ALL_CONS_COLUMNS acc ON acc.OWNER = ac.OWNER AND acc.CONSTRAINT_NAME = ac.CONSTRAINT_NAME
			WHERE ac.CONSTRAINT_TYPE = 'P'
		) pk ON pk.OWNER = c.OWNER AND pk.TABLE_NAME = c.TABLE_NAME AND pk.COLUMN_NAME = c.COLUMN_NAME
		WHERE c.OWNER = COALESCE(NULLIF(?, ''), USER) AND c.TABLE_NAME = ?
		ORDER BY c.COLUMN_ID`
	rows, err := db.SQL().Query(query,
		strings.TrimSpace(schema),
		strings.TrimSpace(table),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]tdb.Column, 0)
	for rows.Next() {
		var column tdb.Column
		var nullable, key string
		var comment, defaultValue, identity sql.NullString
		if err := rows.Scan(&column.Name, &column.Type, &nullable, &key, &comment, &defaultValue, &identity); err != nil {
			return nil, err
		}
		column.Nullable = strings.EqualFold(nullable, "Y")
		column.Key = key
		column.Comment = nullString(comment)
		column.Default = nullString(defaultValue)
		if identity.Valid && strings.EqualFold(strings.TrimSpace(identity.String), "YES") {
			column.Extra = "identity"
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (Driver) Tables(db *tdb.DB, schema string) ([]string, error) {
	const query = `SELECT TABLE_NAME FROM ALL_TABLES
		WHERE OWNER = COALESCE(NULLIF(?, ''), USER) ORDER BY TABLE_NAME`
	rows, err := db.SQL().Query(query, strings.TrimSpace(schema))
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
	const query = `SELECT tab.TABLE_NAME, tab.OWNER, comments.COMMENTS, 'BASE TABLE'
		FROM ALL_TABLES tab
		LEFT JOIN ALL_TAB_COMMENTS comments
		  ON comments.OWNER = tab.OWNER AND comments.TABLE_NAME = tab.TABLE_NAME
		WHERE tab.OWNER = COALESCE(NULLIF(?, ''), USER) AND tab.TABLE_NAME = ?`
	var descriptor tdb.Table
	var comment sql.NullString
	if err := db.SQL().QueryRow(query,
		strings.TrimSpace(schema), strings.TrimSpace(table),
	).Scan(&descriptor.Name, &descriptor.Schema, &comment, &descriptor.Type); err != nil {
		return tdb.Table{}, err
	}
	descriptor.Comment = nullString(comment)
	return descriptor, nil
}

func (Driver) Indexes(db *tdb.DB, table, schema string) ([]tdb.Index, error) {
	const query = `SELECT idx.INDEX_NAME, cols.COLUMN_NAME,
		CASE WHEN idx.UNIQUENESS = 'UNIQUE' THEN 1 ELSE 0 END,
		CASE WHEN cons.CONSTRAINT_TYPE = 'P' THEN 1 ELSE 0 END
		FROM ALL_INDEXES idx
		JOIN ALL_IND_COLUMNS cols ON cols.INDEX_OWNER = idx.OWNER AND cols.INDEX_NAME = idx.INDEX_NAME
		LEFT JOIN ALL_CONSTRAINTS cons ON cons.OWNER = idx.OWNER AND cons.INDEX_NAME = idx.INDEX_NAME
		WHERE idx.OWNER = COALESCE(NULLIF(?, ''), USER) AND idx.TABLE_NAME = ?
		ORDER BY CASE WHEN cons.CONSTRAINT_TYPE = 'P' THEN 0 ELSE 1 END, idx.INDEX_NAME, cols.COLUMN_POSITION`
	return tdb.QueryIndexes(db, query,
		strings.TrimSpace(schema),
		strings.TrimSpace(table),
	)
}

func dmLimit(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	var b strings.Builder
	if offset > 0 {
		fmt.Fprintf(&b, " OFFSET %d ROWS", offset)
	}
	if limit > 0 {
		fmt.Fprintf(&b, " FETCH NEXT %d ROWS ONLY", limit)
	}
	return b.String()
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return strings.TrimSpace(value.String)
	}
	return ""
}
