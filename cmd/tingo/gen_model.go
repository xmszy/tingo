package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	// 数据库驱动：tingo gen model 需真实连接数据库做反向工程，
	_ "github.com/xmszy/tingo-contrib/drivers/mariadb"
	_ "github.com/xmszy/tingo-contrib/drivers/mysql"
	_ "github.com/xmszy/tingo-contrib/drivers/postgres"
	_ "github.com/xmszy/tingo-contrib/drivers/sqlite"
	_ "github.com/xmszy/tingo-contrib/drivers/sqlserver"

	"github.com/xmszy/tingo/database/tdb"
)

// runModelGeneration 按约定数据库配置反向生成单层模型。
func runModelGeneration(args []string) {
	fs := flag.NewFlagSet("model", flag.ExitOnError)
	connection := fs.String("connection", "", "连接名称，默认使用 config/database.toml 的 default")
	driver := fs.String("driver", "", "临时覆盖数据库类型")
	dsn := fs.String("dsn", "", "临时覆盖数据库连接串")
	tables := fs.String("tables", "", "要生成的表名，逗号分隔；留空生成全部表")
	dir := fs.String("dir", "app/model", "模型生成目录")
	pkg := fs.String("pkg", "model", "模型包名")
	fs.Usage = func() {
		fmt.Print(`用法: tingo gen model [选项]

默认读取 config/database.toml 并反向生成 app/model 单层模型。

选项:
  --connection <name> 使用指定命名连接
  --tables <list>     表名，逗号分隔；留空表示全部表
  --dir <path>        生成目录（默认 app/model）
  --pkg <name>        包名（默认 model）
  --driver <name>     临时覆盖数据库类型
  --dsn <dsn>         临时覆盖数据库连接串
`)
	}
	fs.Parse(args)

	appConfig, err := tdb.LoadConfig("config/database.toml")
	if err != nil {
		failModelGeneration("读取数据库配置失败", err)
	}
	dbConfig, resolveErr := appConfig.Resolve(*connection)
	if *driver != "" {
		dbConfig.Driver, dbConfig.Dialect = *driver, *driver
	}
	if *dsn != "" {
		dbConfig.DSN = *dsn
	}
	if resolveErr != nil && *dsn == "" {
		failModelGeneration("数据库连接未配置", resolveErr)
	}
	if dbConfig.Driver == "" {
		dbConfig.Driver, dbConfig.Dialect = "mysql", "mysql"
	}

	db, err := tdb.Open(dbConfig)
	if err != nil {
		if strings.Contains(err.Error(), "unknown driver") {
			fmt.Fprintf(os.Stderr, "连接数据库失败：%v\n", err)
			fmt.Fprintln(os.Stderr, "提示：tingo gen model 已内置 mysql/postgres/sqlite3 驱动；")
			fmt.Fprintln(os.Stderr, "      若使用其它驱动（如 SQL Server），请在其 Go 文件中 import 对应驱动后再运行。")
			os.Exit(1)
		}
		failModelGeneration("连接数据库失败", err)
	}
	defer db.Close()

	want := splitModelTables(*tables)
	if len(want) == 0 {
		want, err = db.Tables()
		if err != nil {
			failModelGeneration("列举数据表失败", err)
		}
	}

	generated := 0
	connectionName := *connection
	if connectionName == "" {
		connectionName = appConfig.Default
	}
	prefix := appConfig.Prefix(connectionName)
	for _, table := range want {
		meta, inspectErr := db.InspectTable(table)
		if inspectErr != nil {
			fmt.Fprintf(os.Stderr, "跳过表 %s: %v\n", table, inspectErr)
			continue
		}
		generateModelFile(*dir, *pkg, meta, prefix)
		generated++
	}
	fmt.Printf("gen model 完成：%d 张表 -> %s\n", generated, *dir)
}

func splitModelTables(value string) []string {
	var tables []string
	for _, table := range strings.Split(value, ",") {
		if table = strings.TrimSpace(table); table != "" {
			tables = append(tables, table)
		}
	}
	return tables
}

func generateModelFile(dir, pkg string, meta *tdb.TableMeta, prefix string) {
	modelName := strings.TrimPrefix(meta.Name, prefix)
	if modelName == "" {
		modelName = meta.Name
	}
	path := filepath.Join(dir, modelName+".go")
	// 若表含名为 table_name 的列，pascalIdentifier 会生成字段 TableName，
	// 与下方自动生成的 TableName() 方法同名导致编译失败。此时通过 modelFieldName
	// 把该字段重命名为 TableNameField，既避免冲突，又保留 TableName() 方法返回表名
	// （列映射依赖 tdb tag，与字段 Go 名无关，重命名不影响 ORM 行为）。
	views, hasTimestamp := buildFieldViews(meta.Columns, meta.Name)
	enums := buildEnums(views)
	// 动态推导自动时间戳的创建/更新列名（兼容 create_time 与 created_at 等约定）。
	createCol, updateCol := "", ""
	for _, v := range views {
		if v.TimestampTag == "create" {
			createCol = v.Name
		}
		if v.TimestampTag == "update" {
			updateCol = v.Name
		}
	}
	var out bytes.Buffer
	if err := generatedModelTemplate.Execute(&out, map[string]any{
		"Package":       pkg,
		"Struct":        pascalIdentifier(modelName),
		"Table":         meta.Name,
		"Fields":        views,
		"Enums":         enums,
		"HasTimestamp":  hasTimestamp,
		"CreateColName": createCol,
		"UpdateColName": updateCol,
	}); err != nil {
		failModelGeneration("模型模板错误", err)
	}
	writeGeneratedModelFile(path, out.String())
	fmt.Printf("  created  %s\n", path)
}

func failModelGeneration(message string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	os.Exit(1)
}

func writeGeneratedModelFile(path, src string) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		failModelGeneration("格式化模型失败", fmt.Errorf("%s: %w", path, err))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		failModelGeneration("创建模型目录失败", err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		failModelGeneration("写入模型失败", err)
	}
}

// modelColumnGoType 将数据库类型映射为 Go 类型（含可空处理）。
// 软删除列由 modelFieldView.IsSoftDelete 单独处理（返回 tdb.SoftDelete/SoftDeleteInt）。
func modelColumnGoType(c tdb.Column) string {
	base := strings.ToLower(strings.TrimSpace(c.Type))
	if base == "tinyint(1)" {
		if c.Nullable {
			return "*bool"
		}
		return "bool"
	}
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	var valueType string
	switch base {
	case "bool", "boolean":
		valueType = "bool"
	case "int", "integer", "tinyint", "smallint", "mediumint", "year":
		valueType = "int"
	case "bigint":
		valueType = "int64"
	case "unsigned bigint":
		valueType = "uint64"
	case "float", "real", "double", "decimal", "numeric", "dec":
		valueType = "float64"
	case "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		valueType = "[]byte"
	case "date", "datetime", "timestamp", "time":
		valueType = "time.Time"
	case "json", "jsonb":
		// tdb 无专用 JSON 类型；用 *string 承载原始 JSON 文本（指针本身已表示空值，
		// 不再叠加可空前缀，避免 **string）。
		return "*string"
	default:
		valueType = "string"
	}
	if c.Nullable {
		valueType = "*" + valueType
	}
	return valueType
}

// isSoftDeleteColumn 判断列是否为软删除列（列名约定 delete_time/deleted_at/delete_at）。
func isSoftDeleteColumn(c tdb.Column) bool {
	name := strings.ToLower(c.Name)
	return name == "delete_time" || name == "deleted_at" || name == "delete_at"
}

// softDeleteGoType 返回软删除列对应的 Go 类型：datetime 类用 tdb.SoftDelete，
// int/bigint 时间戳类用 tdb.SoftDeleteInt。
func softDeleteGoType(c tdb.Column) string {
	base := strings.ToLower(strings.TrimSpace(c.Type))
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	if base == "int" || base == "bigint" || base == "integer" {
		return "tdb.SoftDeleteInt"
	}
	return "tdb.SoftDelete"
}

// modelTimestampTag 返回列的自动时间戳标签：created_at/create_time/update_time/updated_at
// 对应 create/update；软删除列对应 delete。无匹配返回空串。
func modelTimestampTag(c tdb.Column) string {
	name := strings.ToLower(c.Name)
	switch {
	case name == "create_time" || name == "created_at" || name == "create_at":
		return "create"
	case name == "update_time" || name == "updated_at" || name == "update_at":
		return "update"
	case isSoftDeleteColumn(c):
		return "delete"
	}
	return ""
}

// modelValidRules 根据列元信息推导 tvalid 校验规则（基于 os/tvalid，零依赖，
// 与 ORM 解耦）。返回形如 "required|max:50"。
func modelValidRules(c tdb.Column) string {
	var rules []string
	name := strings.ToLower(c.Name)
	// NOT NULL 且非自增 → 必填
	if !c.Nullable && !c.IsAutoIncrement() {
		rules = append(rules, "required")
	}
	// 字符串长度限制：varchar(n)/char(n)
	base := strings.ToLower(strings.TrimSpace(c.Type))
	if idx := strings.Index(base, "varchar"); idx == 0 || strings.HasPrefix(base, "char") {
		if i := strings.Index(base, "("); i >= 0 && i+1 < len(base) {
			end := strings.IndexByte(base[i+1:], ')')
			if end > 0 {
				if n, err := strconv.Atoi(base[i+1 : i+1+end]); err == nil {
					rules = append(rules, fmt.Sprintf("max:%d", n))
				}
			}
		}
	}
	// 语义列名 → 格式校验
	switch {
	case strings.Contains(name, "email") || strings.Contains(name, "mail"):
		rules = append(rules, "email")
	case strings.Contains(name, "url"):
		rules = append(rules, "url")
	case strings.Contains(name, "phone") || strings.Contains(name, "mobile"):
		rules = append(rules, "phone")
	}
	// tinyint(1) 或语义 0/1 状态列 → 枚举范围
	if base == "tinyint(1)" {
		rules = append(rules, "in:0,1")
	}
	if len(rules) == 0 {
		return ""
	}
	return strings.Join(rules, "|")
}

// modelLabel 返回字段的中文标签（优先列注释，否则用列名）。
func modelLabel(c tdb.Column) string {
	if c.Comment != "" {
		return c.Comment
	}
	return c.Name
}

// modelFieldView 是生成模板使用的列视图，预派生 Go 字段名、最终类型、各标签。
type modelFieldView struct {
	tdb.Column
	Field        string // Go 字段名
	GoType       string // 最终 Go 类型（软删列已替换为 tdb.SoftDelete*；enum 列替换为 XxxEnum）
	TimestampTag string // 形如 "create"/"update"/"delete" 或空
	ValidTag     string // 形如 "required|max:50" 或空
	Label        string // 中文标签
	IsSoftDelete bool   // 是否为软删除列
	IsJSON       bool   // 是否为 JSON/JSONB 列（生成 Get/Set 便捷方法）
	EnumValues   string // enum(...) 的枚举值（逗号分隔），否则空
	EnumType     string // enum 列的 Go 类型名（如 GenderEnum），否则空
}

// modelEnumView 是表级枚举类型定义（一张表可能含多个 enum 列，对应多个类型）。
type modelEnumView struct {
	Type   string   // Go 类型名，如 GenderEnum
	Consts []string // 形如 "GenderMale GenderEnum = \"male\"" 的常量声明
}

// tdbTagString 返回列的 tdb tag（含 pk/ai 修饰）。
func tdbTagString(v modelFieldView) string {
	tag := v.Name
	if v.IsPrimary() {
		tag += ",pk"
	}
	if v.IsAutoIncrement() {
		tag += ",ai"
	}
	return tag
}

// buildFieldViews 将表的列转换为模板视图，并推导是否需要自动时间戳。
// table 为原始表名，用于派生全局唯一的枚举类型名（避免多表同名列冲突）。
func buildFieldViews(columns []tdb.Column, table string) (views []modelFieldView, hasTimestamp bool) {
	for _, c := range columns {
		v := modelFieldView{
			Column:       c,
			Field:        modelFieldName(c),
			TimestampTag: modelTimestampTag(c),
			ValidTag:     modelValidRules(c),
			Label:        modelLabel(c),
			IsSoftDelete: isSoftDeleteColumn(c),
		}
		if v.IsSoftDelete {
			v.GoType = softDeleteGoType(c)
		} else {
			v.GoType = modelColumnGoType(c)
			// 提取 enum(...) 枚举值用于注释与类型生成
			base := strings.ToLower(strings.TrimSpace(c.Type))
			if strings.HasPrefix(base, "enum(") {
				if i := strings.Index(base, "("); i >= 0 {
					end := strings.IndexByte(base[i+1:], ')')
					if end > 0 {
						v.EnumValues = strings.TrimSpace(base[i+1 : i+1+end])
						v.EnumType = modelEnumTypeName(c, table)
						// enum 列用专用枚举类型（可空则加指针）。
						if c.Nullable {
							v.GoType = "*" + v.EnumType
						} else {
							v.GoType = v.EnumType
						}
					}
				}
			}
			// JSON/JSONB 列生成 Get/Set 便捷方法。
			if base == "json" || base == "jsonb" {
				v.IsJSON = true
			}
		}
		if v.TimestampTag == "create" || v.TimestampTag == "update" {
			hasTimestamp = true
		}
		views = append(views, v)
	}
	return views, hasTimestamp
}

// modelEnumTypeName 由表名+列名推导 Go 枚举类型名，如 user.gender → UserGenderEnum。
// 加入表名前缀可避免同一包内多张表含同名列（如各表的 enum('N','Y') 权限列）时类型名冲突。
func modelEnumTypeName(c tdb.Column, table string) string {
	return pascalIdentifier(table) + pascalIdentifier(c.Name) + "Enum"
}

// buildEnums 收集表中所有 enum 列，生成对应的 Go 类型与常量定义。
func buildEnums(views []modelFieldView) []modelEnumView {
	var enums []modelEnumView
	for _, v := range views {
		if v.EnumValues == "" || v.EnumType == "" {
			continue
		}
		// 去重：同一枚举类型若来自同名列只生成一次（同名列不会重复，这里防御性去重）。
		dup := false
		for _, e := range enums {
			if e.Type == v.EnumType {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		ev := modelEnumView{Type: v.EnumType}
		for _, raw := range strings.Split(v.EnumValues, ",") {
			val := strings.TrimSpace(strings.Trim(raw, "'\""))
			if val == "" {
				continue
			}
			// 常量名：Type + Pascal(val)。enum 值可能是数字或带特殊字符，做安全化。
			ev.Consts = append(ev.Consts, fmt.Sprintf("%s%s %s = %q", v.EnumType, pascalIdentifier(val), v.EnumType, val))
		}
		enums = append(enums, ev)
	}
	return enums
}

func pascalIdentifier(value string) string {
	var result strings.Builder
	for _, part := range strings.Split(value, "_") {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	return result.String()
}

// modelFieldName 返回列的 Go 字段名。当列名为 table_name 时，pascalIdentifier
// 会得到 TableName，与生成的 TableName() 方法同名冲突；故重命名为 TableNameField，
// 列映射仍由 tdb tag 决定，与字段 Go 名无关，不影响 ORM 行为。
func modelFieldName(column tdb.Column) string {
	// 列名 table_name（含 Table_name/TABLE_NAME 等大小写变体）经 pascal 后会得到
	// TableName，与生成的 TableName() 方法同名冲突；统一重命名为 TableNameField。
	// 列映射依赖 tdb tag（原始列名），与字段 Go 名无关，重命名不影响 ORM 行为。
	if strings.EqualFold(column.Name, "table_name") {
		return "TableNameField"
	}
	return pascalIdentifier(column.Name)
}

var generatedModelTemplate = template.Must(template.New("model").Funcs(template.FuncMap{
	"modelColumnGoType": modelColumnGoType,
	"pascalIdentifier":  pascalIdentifier,
	"modelFieldName":    modelFieldName,
	"tdbTag": func(v modelFieldView) string {
		tag := v.Name
		if v.IsPrimary() {
			tag += ",pk"
		}
		if v.IsAutoIncrement() {
			tag += ",ai"
		}
		return tag
	},
	"fieldTags": func(v modelFieldView) string {
		var tags []string
		tags = append(tags, fmt.Sprintf("json:\"%s\"", v.Name))
		tags = append(tags, fmt.Sprintf("tdb:\"%s\"", tdbTagString(v)))
		if v.TimestampTag != "" {
			tags = append(tags, fmt.Sprintf("timestamp:\"%s\"", v.TimestampTag))
		}
		if v.ValidTag != "" {
			tags = append(tags, fmt.Sprintf("valid:\"%s\"", v.ValidTag))
			tags = append(tags, fmt.Sprintf("label:\"%s\"", v.Label))
		}
		return strings.Join(tags, " ")
	},
	"needsTime": func(views []modelFieldView) bool {
		// 仅当字段的真实 Go 类型使用 time.Time（如 datetime/timestamp/date/time，
		// 含可空 *time.Time）才需要导入 time。软删除列用的是 tdb.SoftDelete /
		// tdb.SoftDeleteInt 自定义类型，不依赖 time 包，不应触发导入。
		for _, v := range views {
			if strings.Contains(v.GoType, "time.Time") {
				return true
			}
		}
		return false
	},
	"needsTvalid": func(views []modelFieldView) bool {
		for _, v := range views {
			if v.ValidTag != "" {
				return true
			}
		}
		return false
	},
	"needsJSON": func(views []modelFieldView) bool {
		for _, v := range views {
			if v.IsJSON {
				return true
			}
		}
		return false
	},
}).Parse(`// Code generated by tingo gen model. DO NOT EDIT.
{{$struct := .Struct}}
package {{.Package}}

import (
	"github.com/xmszy/tingo/database/tdb"
	t "github.com/xmszy/tingo/frame"
{{- if needsTime .Fields}}
	"time"
{{- end}}
{{- if needsTvalid .Fields}}
	"github.com/xmszy/tingo/os/tvalid"
{{- end}}
{{- if needsJSON .Fields}}
	"encoding/json"
{{- end}}
)

// {{.Struct}} 是表 {{.Table}} 的模型。
type {{.Struct}} struct {
{{- range .Fields}}
	// {{.Name}}{{if .Comment}} {{.Comment}}{{end}}{{if .EnumValues}} (枚举: {{.EnumValues}}){{end}}
	{{.Field}} {{.GoType}} ` + "`{{fieldTags .}}`" + `
{{- end}}
}

// 表 {{.Table}} 的枚举类型定义。
{{- range .Enums}}
type {{.Type}} string

const (
{{- range .Consts}}
	{{.}}
{{- end}}
)
{{- end}}

func ({{.Struct}}) TableName() string { return "{{.Table}}" }

// Validate 使用框架内置 tvalid 校验模型字段（规则见 struct tag valid）。
func (x *{{.Struct}}) Validate() error { return tvalid.CheckStruct(x) }
{{- range .Fields}}{{if .IsJSON}}

// Get{{.Field}} 解析 {{.Name}} JSON 列（*string）为 map。
func (x *{{$struct}}) Get{{.Field}}() (map[string]any, error) {
	if x.{{.Field}} == nil {
		return nil, nil
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(*x.{{.Field}}), &v); err != nil {
		return nil, err
	}
	return v, nil
}

// Set{{.Field}} 将任意值序列化写入 {{.Name}} JSON 列（*string）。
func (x *{{$struct}}) Set{{.Field}}(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s := string(b)
	x.{{.Field}} = &s
	return nil
}
{{- end}}{{end}}

// New{{.Struct}} 返回使用默认或命名连接的查询模型（表名单数 {{.Struct}} 参与拼名）。
func New{{.Struct}}(connection ...string) *tdb.Model[{{.Struct}}] {
	mm := tdb.NewModel[{{.Struct}}](t.Database(connection...))
{{- if .HasTimestamp}}
	// 自动时间戳：Insert 填 {{.CreateColName}}，Update/Save 填 {{.UpdateColName}}（零值才填，不覆盖显式设置）。
	mm = mm.AutoTimestamp("{{.CreateColName}}", "{{.UpdateColName}}")
{{- end}}
	return mm
}

// BeforeInsert 在 Insert 执行前调用，可填充默认字段、加密密码等。
func (x *{{.Struct}}) BeforeInsert() error { return nil }

// AfterInsert 在 Insert 执行成功后调用。
func (x *{{.Struct}}) AfterInsert() error { return nil }

// BeforeUpdate 在 Update 执行前调用，可填充更新时间、清洗字段等。
func (x *{{.Struct}}) BeforeUpdate() error { return nil }

// AfterUpdate 在 Update 执行成功后调用。
func (x *{{.Struct}}) AfterUpdate() error { return nil }

// BeforeDelete 在 Delete 执行前调用。
func (x *{{.Struct}}) BeforeDelete() error { return nil }

// AfterDelete 在 Delete 执行成功后调用。
func (x *{{.Struct}}) AfterDelete() error { return nil }

// BeforeQuery 在 Select 查询执行前调用，可追加默认查询条件（如租户过滤）。
func (x *{{.Struct}}) BeforeQuery() error { return nil }

// BeforeSave 在 Save（Insert 或 Update）执行前调用。
func (x *{{.Struct}}) BeforeSave() error { return nil }

// AfterSave 在 Save（Insert 或 Update）执行成功后调用。
func (x *{{.Struct}}) AfterSave() error { return nil }

// AfterQuery 在记录查询扫描后调用，可将存储值转换为业务形态（如解密、拼接）。
func (x *{{.Struct}}) AfterQuery() error { return nil }
`))
