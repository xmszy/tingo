package tdb

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Schema 提供数据库结构变更 DSL（Schema Builder / Migration）。
//
// 使用 DB.SchemaTool() 获取 Schema 实例，然后链式调用 CreateTable/DropTable 等。
//
// 用法：
//
//	type User struct {
//	    Id   int    `tdb:"id,primaryKey,autoIncrement"`
//	    Name string `tdb:"name,type:varchar(100)"`
//	    Age  int    `tdb:"age,type:int,nullable"`
//	}
//
//	err := db.SchemaTool().CreateTableFrom(User{})
type Schema struct {
	db     *DB
	tx     *Tx
	dial   Dialect
	dryRun bool // 仅生成 SQL，不执行
}

// SchemaTool 从 DB 实例创建 Schema 工具。
func (db *DB) SchemaTool() *Schema {
	return &Schema{db: db, dial: db.Dialect()}
}

// AutoMigrate 按模型自动建表/补列（schema 自动对齐）。详见 Schema.AutoMigrate。
func (db *DB) AutoMigrate(models ...any) error {
	return db.SchemaTool().AutoMigrate(models...)
}

// SchemaFromTx 从 Tx 实例创建 Schema（操作在同一事务中）。
func (tx *Tx) Schema() *Schema {
	return &Schema{tx: tx, dial: tx.dial, db: tx.db}
}

// DryRun 设置为仅生成 SQL 不执行。
func (s *Schema) DryRun() *Schema {
	s.dryRun = true
	return s
}

// CreateTableFrom 从结构体定义创建表。
// 使用 struct tag 定义列属性。
func (s *Schema) CreateTableFrom(model any) error {
	tableName := tableNameOf(reflectTypeOf(model))
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS ")
	b.WriteString(s.dial.Quote(tableName))
	b.WriteString(" (\n")

	cols := columnsFromModel(model, s.dial)
	b.WriteString(strings.Join(cols, ",\n"))
	b.WriteString("\n)")

	sqlStr := b.String()
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// CreateTable 手动指定表名和列定义。
func (s *Schema) CreateTable(tableName string, columns map[string]string) error {
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS ")
	b.WriteString(s.dial.Quote(tableName))
	b.WriteString(" (\n")

	var cols []string
	for col, def := range columns {
		cols = append(cols, s.dial.Quote(col)+" "+def)
	}
	b.WriteString(strings.Join(cols, ",\n"))
	b.WriteString("\n)")

	sqlStr := b.String()
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// DropTable 删除表。
func (s *Schema) DropTable(tableName string) error {
	sqlStr := "DROP TABLE IF EXISTS " + s.dial.Quote(tableName)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// AddColumn 添加列。
func (s *Schema) AddColumn(tableName, columnName, definition string) error {
	sqlStr := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		s.dial.Quote(tableName),
		s.dial.Quote(columnName),
		definition,
	)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// DropColumn 删除列。
func (s *Schema) DropColumn(tableName, columnName string) error {
	sqlStr := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s",
		s.dial.Quote(tableName),
		s.dial.Quote(columnName),
	)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// ModifyColumn 修改列。
func (s *Schema) ModifyColumn(tableName, columnName, definition string) error {
	var template string
	switch s.dial.Name() {
	case "mysql":
		template = "ALTER TABLE %s MODIFY COLUMN %s %s"
	case "postgres":
		template = "ALTER TABLE %s ALTER COLUMN %s TYPE %s"
	default:
		template = "ALTER TABLE %s MODIFY COLUMN %s %s"
	}
	sqlStr := fmt.Sprintf(template,
		s.dial.Quote(tableName),
		s.dial.Quote(columnName),
		definition,
	)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// AddIndex 创建索引。
func (s *Schema) AddIndex(tableName, indexName, columns string, unique bool) error {
	typ := "INDEX"
	if unique {
		typ = "UNIQUE INDEX"
	}
	sqlStr := fmt.Sprintf("CREATE %s %s ON %s (%s)",
		typ,
		s.dial.Quote(indexName),
		s.dial.Quote(tableName),
		columns,
	)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// AddForeignKey 添加外键约束。
//
// 用法示例：
//
//	s.SchemaTool().AddForeignKey("orders", "fk_orders_user", "user_id", "users", "id", "CASCADE", "RESTRICT")
func (s *Schema) AddForeignKey(tableName, constraintName, column, refTable, refColumn string, onDelete, onUpdate string) error {
	var b strings.Builder
	b.WriteString("ALTER TABLE ")
	b.WriteString(s.dial.Quote(tableName))
	b.WriteString(" ADD CONSTRAINT ")
	b.WriteString(s.dial.Quote(constraintName))
	b.WriteString(" FOREIGN KEY (")
	b.WriteString(s.dial.Quote(column))
	b.WriteString(") REFERENCES ")
	b.WriteString(s.dial.Quote(refTable))
	b.WriteString(" (")
	b.WriteString(s.dial.Quote(refColumn))
	b.WriteString(")")
	if onDelete != "" {
		b.WriteString(" ON DELETE ")
		b.WriteString(onDelete)
	}
	if onUpdate != "" {
		b.WriteString(" ON UPDATE ")
		b.WriteString(onUpdate)
	}

	sqlStr := b.String()
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// DropForeignKey 删除外键约束。
//
// 用法示例：
//
//	s.SchemaTool().DropForeignKey("orders", "fk_orders_user")
func (s *Schema) DropForeignKey(tableName, constraintName string) error {
	var template string
	switch s.dial.Name() {
	case "mysql":
		template = "ALTER TABLE %s DROP FOREIGN KEY %s"
	default:
		template = "ALTER TABLE %s DROP CONSTRAINT %s"
	}
	sqlStr := fmt.Sprintf(template,
		s.dial.Quote(tableName),
		s.dial.Quote(constraintName),
	)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// DropIndex 删除索引。
func (s *Schema) DropIndex(tableName, indexName string) error {
	var template string
	switch s.dial.Name() {
	case "mysql":
		template = "DROP INDEX %s ON %s"
	case "postgres":
		template = "DROP INDEX %s"
	default:
		template = "DROP INDEX %s"
	}
	sqlStr := fmt.Sprintf(template,
		s.dial.Quote(indexName),
		s.dial.Quote(tableName),
	)
	if s.dryRun {
		fmt.Println(sqlStr)
		return nil
	}
	_, err := s.exec(sqlStr)
	return err
}

// columnsMapFromModel 返回模型字段生成的「列名 -> 完整列定义」映射。
func columnsMapFromModel(model any, dial Dialect) map[string]string {
	m := make(map[string]string)
	for _, col := range columnsFromModel(model, dial) {
		// col 形如 "`id` BIGINT ..."，解析首个被引号包裹的列名为键。
		name, _, _ := strings.Cut(col, " ")
		name = strings.Trim(name, "`\"[]")
		m[name] = col
	}
	return m
}

// AutoMigrate 按模型自动建表/补列（schema 自动对齐）。
//
// 行为：
//   - 表不存在：通过 CreateTableFrom 创建（内部 CREATE TABLE IF NOT EXISTS，幂等）。
//   - 表存在且方言注册了 SchemaDriver（InspectTable 可用）：对比模型字段，
//     为缺失的列执行 AddColumn（新增列安全）；默认【不】做已有列的类型修改
//     （类型变更跨 dialect 不可靠且可能丢数据，需显式迁移处理）。
//   - 表存在但未注册 SchemaDriver（无法内省）：仅保证表存在，不补列。
//
// 调用示例：
//
//	db.AutoMigrate(&User{}, &Order{})
func (s *Schema) AutoMigrate(models ...any) error {
	for _, m := range models {
		prefix := ""
		if s.db != nil {
			prefix = s.db.cfg.Prefix
		}
		model := newZeroModel(m, prefix)
		table := model.table
		desired := columnsMapFromModel(m, s.dial)

		meta, err := s.db.InspectTable(table)
		if err != nil {
			if errors.Is(err, ErrTableNotFound) {
				// 表不存在：创建。
				if cerr := s.CreateTableFrom(m); cerr != nil {
					return cerr
				}
				continue
			}
			// 无法内省（多半未注册 SchemaDriver）：退化为仅保证表存在。
			if cerr := s.CreateTableFrom(m); cerr != nil {
				return cerr
			}
			continue
		}
		// 表存在：补齐缺失列。
		existing := make(map[string]bool, len(meta.Columns))
		for _, c := range meta.Columns {
			existing[c.Name] = true
		}
		for name, def := range desired {
			if existing[name] {
				continue
			}
			if aerr := s.AddColumn(table, def, ""); aerr != nil {
				return aerr
			}
		}
	}
	return nil
}

// newZeroModel 由任意模型指针构造一个 Model 以读取表名（与 Model 内部一致）。
func newZeroModel(m any, prefix string) *Model[any] {
	table := modelTableName(m, prefix)
	return &Model[any]{table: table}
}

// modelTableName 由模型值推断表名：TableName() 接口 > 类型名 snake_case，并应用前缀。
func modelTableName(m any, prefix string) string {
	rt := reflectTypeOf(m)
	table := resolveTableByName(rt)
	if prefix != "" && !strings.HasPrefix(table, prefix) {
		table = prefix + table
	}
	return table
}

// resolveTableByName 与 Model.resolveTable 等价的非泛型版本。
func resolveTableByName(rt reflect.Type) string {
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if tn, ok := reflect.New(rt).Interface().(tableNamer); ok {
		if name := tn.TableName(); name != "" {
			return name
		}
	}
	return toSnake(rt.Name())
}

func (s *Schema) exec(sqlStr string, args ...any) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.exec(sqlStr, args...)
	}
	return s.db.exec(sqlStr, args...)
}

// ---- 从模型生成列定义 ----

func columnsFromModel(model any, dial Dialect) []string {
	rt := reflectTypeOf(model)
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}

	var cols []string
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() || f.Anonymous {
			continue
		}
		col := columnOf(f)
		if col == "" || col == "-" {
			continue
		}
		colDef := columnSQL(dial, col, f)
		cols = append(cols, colDef)
	}
	return cols
}

func columnSQL(dial Dialect, colName string, f reflect.StructField) string {
	var parts []string
	parts = append(parts, dial.Quote(colName))

	// 类型推断
	switch f.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		parts = append(parts, "INT")
	case reflect.Int64:
		if f.Type == reflect.TypeFor[int64]() {
			parts = append(parts, "BIGINT")
		} else {
			parts = append(parts, "INT")
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		parts = append(parts, "INT UNSIGNED")
	case reflect.Uint64:
		parts = append(parts, "BIGINT UNSIGNED")
	case reflect.Float32, reflect.Float64:
		parts = append(parts, "DOUBLE")
	case reflect.String:
		parts = append(parts, "VARCHAR(255)")
	case reflect.Bool:
		parts = append(parts, "TINYINT(1)")
	default:
		if f.Type == reflect.TypeFor[*sql.NullTime]() || strings.Contains(f.Type.String(), "Time") {
			parts = append(parts, "DATETIME")
		} else if strings.Contains(f.Type.String(), "SoftDelete") {
			parts = append(parts, "DATETIME NULL")
		} else {
			parts = append(parts, "VARCHAR(255)")
		}
	}

	// 标签属性
	tag := f.Tag.Get("tdb")
	for _, attr := range strings.Split(tag, ",")[1:] {
		switch strings.TrimSpace(attr) {
		case "primaryKey":
			parts = append(parts, "PRIMARY KEY")
		case "autoIncrement":
			parts = append(parts, "AUTO_INCREMENT")
		case "nullable":
			parts = append(parts, "NULL")
		case "unique":
			parts = append(parts, "UNIQUE")
		case "notnull":
			parts = append(parts, "NOT NULL")
		}
		if strings.HasPrefix(attr, "type:") {
			typeDef := strings.TrimPrefix(attr, "type:")
			parts[len(parts)-1] = typeDef
		}
	}

	return "  " + strings.Join(parts, " ")
}

func reflectTypeOf(v any) reflect.Type {
	rt := reflect.TypeOf(v)
	if rt == nil {
		return nil
	}
	return rt
}
