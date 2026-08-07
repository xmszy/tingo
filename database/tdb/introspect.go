package tdb

import (
	"fmt"
	"strings"
)

// Column 是数据库列的统一元信息。各驱动负责把数据库特有结果归一化到该结构。
type Column struct {
	Name     string // 列名
	Type     string // 数据库原生类型（保留长度、精度等修饰）
	Nullable bool   // 是否允许 NULL
	Key      string // PRI/UNI/MUL，至少保证主键归一化为 PRI
	Comment  string // 列注释
	Default  string // 默认值；无默认值与 NULL 默认值当前均为空字符串
	Extra    string // auto_increment、identity 等驱动特有信息
}

func (c Column) IsPrimary() bool {
	return strings.EqualFold(c.Key, "PRI")
}

func (c Column) IsAutoIncrement() bool {
	extra := strings.ToLower(c.Extra)
	return strings.Contains(extra, "auto_increment") || strings.Contains(extra, "identity")
}

type IndexKind string

const (
	IndexKindRegular  IndexKind = "index"
	IndexKindUnique   IndexKind = "unique"
	IndexKindPrimary  IndexKind = "primary"
	IndexKindSorting  IndexKind = "sorting"
	IndexKindSkipping IndexKind = "skipping"
)

// Index 是数据库索引与检索键的统一元信息。
// Expression 用于无法可靠拆分为列名的函数式键；Type 和 Granularity 用于 ClickHouse 跳数索引。
type Index struct {
	Name        string
	Columns     []string
	Unique      bool
	Primary     bool
	Kind        IndexKind
	Expression  string
	Type        string
	Granularity uint64
}

// Table 是数据库表的统一元信息。
type Table struct {
	Name    string
	Schema  string
	Comment string
	Type    string
}

// QueryIndexes 执行标准化索引查询并按查询结果顺序聚合复合索引。
// query 必须依次返回 index_name、column_name、is_unique、is_primary。
func QueryIndexes(db *DB, query string, args ...any) ([]Index, error) {
	rows, err := db.SQL().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make([]Index, 0)
	positions := make(map[string]int)
	for rows.Next() {
		var name, column string
		var unique, primary bool
		if err := rows.Scan(&name, &column, &unique, &primary); err != nil {
			return nil, err
		}
		position, exists := positions[name]
		if !exists {
			position = len(indexes)
			positions[name] = position
			kind := IndexKindRegular
			switch {
			case primary:
				kind = IndexKindPrimary
			case unique:
				kind = IndexKindUnique
			}
			indexes = append(indexes, Index{Name: name, Unique: unique, Primary: primary, Kind: kind})
		}
		indexes[position].Columns = append(indexes[position].Columns, column)
	}
	return indexes, rows.Err()
}

// TableMeta 是一张表的元信息。
type TableMeta struct {
	Name       string
	Schema     string
	Comment    string
	Type       string
	Columns    []Column
	Indexes    []Index
	PrimaryKey string // 兼容单主键调用；复合主键时取第一列
}

// PrimaryKeys 优先按主键约束/索引定义顺序返回；无索引元信息时回退到列定义顺序。
func (m *TableMeta) PrimaryKeys() []string {
	for _, index := range m.Indexes {
		if index.Primary {
			return append([]string(nil), index.Columns...)
		}
	}
	keys := make([]string, 0, 1)
	for _, column := range m.Columns {
		if column.IsPrimary() {
			keys = append(keys, column.Name)
		}
	}
	return keys
}

// schemaOf 返回显式配置的 schema。数据库默认 schema 由具体驱动决定。
func (db *DB) schemaOf() string {
	if db.schema == "" {
		db.schema = db.cfg.Schema
	}
	return db.schema
}

func (db *DB) metadataDriver() (SchemaDriver, bool) {
	if db.driver != nil && db.driver.Metadata() != nil {
		return db.driver.Metadata(), true
	}
	return SchemaDriverFor(db.dial.Name())
}

// Tables 列举当前 schema 内所有基表名。
func (db *DB) Tables() ([]string, error) {
	driver, ok := db.metadataDriver()
	if !ok {
		return nil, fmt.Errorf("tdb: schema driver %q is not registered", db.dial.Name())
	}
	return driver.Tables(db, db.schemaOf())
}

// InspectTable 读取单张表的统一元信息。
func (db *DB) InspectTable(table string) (*TableMeta, error) {
	driver, ok := db.metadataDriver()
	if !ok {
		return nil, fmt.Errorf("tdb: schema driver %q is not registered", db.dial.Name())
	}
	schema := db.schemaOf()
	cols, err := driver.Columns(db, table, schema)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrTableNotFound, table)
	}

	meta := &TableMeta{Name: table, Schema: schema, Columns: cols}
	if tableDriver, supported := driver.(TableSchemaDriver); supported {
		descriptor, err := tableDriver.Table(db, table, schema)
		if err != nil {
			return nil, err
		}
		if descriptor.Name != "" {
			meta.Name = descriptor.Name
		}
		if descriptor.Schema != "" {
			meta.Schema = descriptor.Schema
		}
		meta.Comment = descriptor.Comment
		meta.Type = descriptor.Type
	}
	if indexDriver, supported := driver.(IndexSchemaDriver); supported {
		meta.Indexes, err = indexDriver.Indexes(db, table, schema)
		if err != nil {
			return nil, err
		}
	}
	keys := meta.PrimaryKeys()
	if len(keys) > 0 {
		meta.PrimaryKey = keys[0]
		if !hasPrimaryIndex(meta.Indexes) {
			meta.Indexes = append([]Index{{Name: "PRIMARY", Columns: keys, Unique: true, Primary: true, Kind: IndexKindPrimary}}, meta.Indexes...)
		}
	}
	return meta, nil
}

func hasPrimaryIndex(indexes []Index) bool {
	for _, index := range indexes {
		if index.Primary {
			return true
		}
	}
	return false
}
