package tdb

import "strings"

// SchemaDriver 是数据库元信息驱动接口。
// 各数据库方言实现该接口以支持 InspectTable、Tables 等逆向元信息操作。
type SchemaDriver interface {
	// Columns 返回指定表的列信息。
	Columns(db *DB, table, schema string) ([]Column, error)
	// Tables 返回指定 schema 下的所有表名。
	Tables(db *DB, schema string) ([]string, error)
}

// TableSchemaDriver 提供表级元信息（表注释、类型等）。
type TableSchemaDriver interface {
	// Table 返回表的描述元信息。
	Table(db *DB, table, schema string) (TableDescriptor, error)
}

// TableDescriptor 是表描述元信息。
type TableDescriptor struct {
	Name    string
	Schema  string
	Comment string
	Type    string
}

// IndexSchemaDriver 提供索引元信息。
type IndexSchemaDriver interface {
	// Indexes 返回指定表的索引信息。
	Indexes(db *DB, table, schema string) ([]Index, error)
}

// schemaDriverRegistry 方言名到 SchemaDriver 的注册表。
var schemaDriverRegistry = make(map[string]SchemaDriver)

// RegisterSchemaDriver 按方言名注册 SchemaDriver 实现。
func RegisterSchemaDriver(dialectName string, driver SchemaDriver) {
	if driver == nil {
		panic("tdb: nil schema driver")
	}
	dialectName = strings.TrimSpace(dialectName)
	dialectName = strings.ToLower(dialectName)
	schemaDriverRegistry[dialectName] = driver
}

// RegisterSchemaDriverByName 按方言名注册 SchemaDriver 实现（同 RegisterSchemaDriver）。
func RegisterSchemaDriverByName(dialectName string, driver SchemaDriver) {
	RegisterSchemaDriver(dialectName, driver)
}

// SchemaDriverFor 按方言名查找 SchemaDriver。
func SchemaDriverFor(dialectName string) (SchemaDriver, bool) {
	d, ok := schemaDriverRegistry[strings.ToLower(strings.TrimSpace(dialectName))]
	return d, ok
}

// RegisteredDrivers 返回已注册的 SchemaDriver 方言名列表。
func RegisteredDrivers() []string {
	names := make([]string, 0, len(schemaDriverRegistry))
	for name := range schemaDriverRegistry {
		names = append(names, name)
	}
	return names
}
