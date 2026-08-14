package tdb

import (
	"fmt"
	"strings"
	"sync"
)

// Dialect 抽象不同数据库的 SQL 方言差异：标识符引用、占位符风格、LIMIT/OFFSET 语法。
//
// 注意：列/表元信息查询（information_schema / PRAGMA / 系统视图）已从 Dialect 剥离，
// 独立封装为 SchemaDriver（见 schema.go 与各 schema_*.go 文件）。
type Dialect interface {
	// Name 返回方言名（mysql/postgres/sqlite/sqlserver）。
	Name() string
	// Quote 引用标识符（表名/列名），如 MySQL -> `col`，postgres -> "col"。
	Quote(ident string) string
	// Placeholder 返回第 n 个（从 0 开始）占位符，如 mysql "?"，postgres "$1"。
	Placeholder(n int) string
	// Limit 生成 LIMIT/OFFSET 子句（已含前导空格或空串）。
	Limit(limit, offset int) string
}

// DialectDefinition 允许独立驱动声明方言差异，而无需依赖核心内部实现。
// nil PlaceholderFunc 使用问号占位符，nil LimitFunc 使用 LIMIT/OFFSET。
type DialectDefinition struct {
	DialectName     string
	QuoteLeft       string
	QuoteRight      string
	PlaceholderFunc func(int) string
	LimitFunc       func(limit, offset int) string
}

func (d DialectDefinition) Name() string { return d.DialectName }

func (d DialectDefinition) Quote(ident string) string {
	if ident == "*" {
		return ident
	}
	parts := strings.Split(ident, ".")
	for i, part := range parts {
		if part != "*" {
			parts[i] = d.QuoteLeft + part + d.QuoteRight
		}
	}
	return strings.Join(parts, ".")
}

func (d DialectDefinition) Placeholder(n int) string {
	if d.PlaceholderFunc != nil {
		return d.PlaceholderFunc(n)
	}
	return "?"
}

func (d DialectDefinition) Limit(limit, offset int) string {
	if d.LimitFunc != nil {
		return d.LimitFunc(limit, offset)
	}
	return limitOffset(limit, offset)
}

func limitOffset(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, " LIMIT %d", limit)
	if offset > 0 {
		fmt.Fprintf(&b, " OFFSET %d", offset)
	}
	return b.String()
}

// placeholderStyle 占位符风格。
type placeholderStyle int

const (
	styleQuestion placeholderStyle = iota // "?"
	styleDollar                           // "$1","$2"...
)

// limitStyle 分页语法。
type limitStyle int

const (
	limitMyStyle limitStyle = iota // LIMIT x OFFSET y
	limitFetch                     // OFFSET y ROWS FETCH NEXT x ROWS ONLY (sqlserver)
)

// baseDialect 通用实现，按字段组合方言行为。
type baseDialect struct {
	name   string
	quoteL byte
	quoteR byte
	ph     placeholderStyle
	lim    limitStyle
}

func (d baseDialect) Name() string { return d.name }

// Now 返回当前时间的 SQL 表达式（实现 NowDialect 可选扩展）。
// 标准 SQL 默认为 CURRENT_TIMESTAMP；特定方言在下方覆盖。
func (d baseDialect) Now() string { return "CURRENT_TIMESTAMP" }

func (d baseDialect) Quote(ident string) string {
	if ident == "*" {
		return ident
	}
	// 已带点的复合引用（如 t.col）逐段引用。
	if strings.Contains(ident, ".") {
		parts := strings.Split(ident, ".")
		for i, p := range parts {
			if p != "*" {
				parts[i] = d.quoteOne(p)
			}
		}
		return strings.Join(parts, ".")
	}
	return d.quoteOne(ident)
}

func (d baseDialect) quoteOne(ident string) string {
	return string(d.quoteL) + ident + string(d.quoteR)
}

func (d baseDialect) Placeholder(n int) string {
	switch d.ph {
	case styleDollar:
		return fmt.Sprintf("$%d", n+1)
	default:
		return "?"
	}
}

func (d baseDialect) Limit(limit, offset int) string {
	switch d.lim {
	case limitFetch:
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
	default:
		if limit <= 0 && offset <= 0 {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, " LIMIT %d", limit)
		if offset > 0 {
			fmt.Fprintf(&b, " OFFSET %d", offset)
		}
		return b.String()
	}
}

// dialects 已注册方言表（按 Name 查找）。注册/查找均加锁，允许运行时（如插件 init）注册。
var (
	dialectsMu sync.RWMutex
	dialects   = map[string]Dialect{
		"mysql": mysqlDialect{baseDialect: baseDialect{
			name: "mysql", quoteL: '`', quoteR: '`', ph: styleQuestion, lim: limitMyStyle,
		}},
		"sqlite": sqliteDialect{baseDialect: baseDialect{
			name: "sqlite", quoteL: '"', quoteR: '"', ph: styleQuestion, lim: limitMyStyle,
		}},
		"postgres":  baseDialect{name: "postgres", quoteL: '"', quoteR: '"', ph: styleDollar, lim: limitMyStyle},
		"sqlserver": baseDialect{name: "sqlserver", quoteL: '"', quoteR: '"', ph: styleQuestion, lim: limitFetch},
	}
)

// mysqlDialect 仅在需要覆盖默认行为时存在（占位符/分页与 base 一致）。
type mysqlDialect struct{ baseDialect }

// Now 覆盖为 MySQL 的当前时间表达式。
func (mysqlDialect) Now() string { return "NOW()" }

// RegisterDialect 注册自定义方言（驱动作者扩展用）。线程安全。
func RegisterDialect(d Dialect) {
	dialectsMu.Lock()
	defer dialectsMu.Unlock()
	dialects[d.Name()] = d
}

// DialectFor 查找已注册方言。线程安全。
func DialectFor(name string) (Dialect, bool) {
	dialectsMu.RLock()
	defer dialectsMu.RUnlock()
	dialect, ok := dialects[strings.ToLower(strings.TrimSpace(name))]
	return dialect, ok
}

// sqliteDialect 仅负责语法差异（标识符引用、占位符、LIMIT 风格）。
// 列/表元信息由 sqliteSchema 通过 PRAGMA 封装（见 schema_sqlite.go）。
type sqliteDialect struct{ baseDialect }

func (sqliteDialect) Name() string { return "sqlite" }

// Now 覆盖为 SQLite 的当前时间表达式。
func (sqliteDialect) Now() string { return "datetime('now')" }
