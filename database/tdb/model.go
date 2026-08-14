package tdb

import (
	"context"
	"reflect"
	"strings"
	"time"
)

// newModel 构造泛型模型。tx 为 nil 表示基于 DB。
func newModel[T any](db *DB, dial Dialect, tx *Tx, table ...string) *Model[T] {
	m := &Model[T]{
		db:   db,
		dial: dial,
		tx:   tx,
	}
	m.table = resolveTable[T](table...)
	if db != nil && db.cfg.Prefix != "" && !strings.HasPrefix(m.table, db.cfg.Prefix) {
		m.table = db.cfg.Prefix + m.table
	}
	return m
}

// tableNamer 若 T 实现该接口则用于决定表名（优先级高于类型名推断）。
type tableNamer interface{ TableName() string }

// resolveTable 推断表名：显式参数 > TableName() 接口 > 类型名 snake_case。
func resolveTable[T any](table ...string) string {
	if len(table) > 0 && table[0] != "" {
		return table[0]
	}
	var zero T
	if tn, ok := any(zero).(tableNamer); ok {
		if name := tn.TableName(); name != "" {
			return name
		}
	}
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return toSnake(t.Name())
}

// Model 是绑定到表与类型 T 的泛型查询/操作模型。
//
// Safe 双模式：
//   - 默认 unsafe：链式方法原地修改 Model，高性能，适合一次性构建查询。
//   - Safe() 后可复用：调用 Safe() 后，每个链式方法返回 Clone 副本，
//     原始 Model 保持洁净可当作"查询模板"反复使用。
type Model[T any] struct {
	db       *DB
	tx       *Tx
	dial     Dialect
	table    string
	fields   []string // SELECT 字段，空表示 *
	wheres   []whereClause
	orders   []string
	groups   []string
	havings  []havingClause
	joins    []joinClause
	limitN   int
	offsetN  int
	distinct bool
	allowAll bool // 解除 "无 WHERE 禁止整表写" 护栏
	safe     bool // true 时链式方法返回 Clone，原 Model 不被修改

	// 关联预加载
	preloads []preloadEntry

	// 软删除
	withTrashed  bool // 查询时包含已软删除记录
	onlyTrashed  bool // 仅查询已软删除记录
	disableHooks bool // 禁用模型钩子

	// 自动时间戳
	autoCreateTime string // 创建时间字段列名（空=禁用）
	autoUpdateTime string // 更新时间字段列名（空=禁用）

	// 查询缓存
	cacheKey     string
	cacheTTL     time.Duration
	cacheEnabled bool

	// 悲观锁子句（如 "FOR UPDATE"），仅对 SELECT 生效。
	lock string
	// 排除列（FieldsEx），与 fields 互斥（fields 优先）。
	fieldsEx []string
	// 查询上下文（用于超时/取消透传），nil 表示不携带。
	ctx context.Context
	// 强制走主库读（读写分离场景下，默认读走从库；事务内自动走主库）。
	useMaster bool

	// 查询作用域，在查询执行前自动应用。
	scopes []ScopeFunc[T]
}

// ScopeFunc 是查询作用域函数：接收当前模型，返回附加了查询条件的新模型。
//
// 用法：
//
//	func OnlyActive(m *tdb.Model[User]) *tdb.Model[User] {
//	    return m.Where("status", 1)
//	}
//	func Recent(m *tdb.Model[User]) *tdb.Model[User] {
//	    return m.Order("id desc")
//	}
//
//	users, _ := db.Model[User]().Scopes(OnlyActive, Recent).All()
type ScopeFunc[T any] func(*Model[T]) *Model[T]

// AutoTimestamp 启用自动时间戳（列名为蛇形，如 "create_time"、"update_time"）。
//
// 启用后 Insert 自动填充 createCol，Update/Save 自动填充 updateCol。
// 仅当对应字段值为零值时才填充（不覆盖业务方显式设置的时间）。
func (m *Model[T]) AutoTimestamp(createCol, updateCol string) *Model[T] {
	model := m.getModel()
	model.autoCreateTime = createCol
	model.autoUpdateTime = updateCol
	return model
}

type preloadEntry struct {
	name      string
	preloader preloader
}

// With 注册关联预加载（单个）。
// 用法：model.With(RelationName, relationPreloader)
func (m *Model[T]) With(name string, loader preloader) *Model[T] {
	model := m.getModel()
	loader.setConfig(preloaderConfig{
		WithTrashed:  m.withTrashed,
		OnlyTrashed:  m.onlyTrashed,
		DisableHooks: m.disableHooks,
		CacheEnabled: m.cacheEnabled,
		CacheKey:     m.cacheKey,
		CacheTTL:     m.cacheTTL,
	})
	model.preloads = append(model.preloads, preloadEntry{name: name, preloader: loader})
	return model
}

// Without 排除某个预加载。
func (m *Model[T]) Without(name string) *Model[T] {
	model := m.getModel()
	filtered := make([]preloadEntry, 0, len(model.preloads))
	for _, p := range model.preloads {
		if p.name != name {
			filtered = append(filtered, p)
		}
	}
	model.preloads = filtered
	return model
}

// WithAll 一次性注册多个关联预加载。
func (m *Model[T]) WithAll(loaders map[string]preloader) *Model[T] {
	model := m.getModel()
	for name, loader := range loaders {
		model.preloads = append(model.preloads, preloadEntry{name: name, preloader: loader})
	}
	return model
}

// Together 级联/聚合模式标记。tingo 的预加载底层已采用「一次 IN 查询聚合所有父记录」
// 的批量策略（等价于 gf 的 together 模式），故 Together() 不改变执行行为，仅作为
// 链式可读性标记与 API 对齐存在。
//
// 典型用法（与 WithCount/WithSum 组合）：
//
//	q := db.Model[User]().
//	    With("Orders", tdb.HasMany[User, Order]("id", "user_id")).
//	    Together()
//	users, _ := q.All()
func (m *Model[T]) Together() *Model[T] {
	return m.getModel()
}

// WithTrashed 查询时包含已软删除的记录。
func (m *Model[T]) WithTrashed() *Model[T] {
	model := m.getModel()
	model.withTrashed = true
	model.onlyTrashed = false
	return model
}

// OnlyTrashed 仅查询已软删除的记录。
func (m *Model[T]) OnlyTrashed() *Model[T] {
	model := m.getModel()
	model.withTrashed = false
	model.onlyTrashed = true
	return model
}

// DisableHooks 禁用当前查询的模型钩子。
func (m *Model[T]) DisableHooks() *Model[T] {
	model := m.getModel()
	model.disableHooks = true
	return model
}

type whereClause struct {
	expr string
	args []any
}

type joinClause struct {
	typ   string // INNER/LEFT/RIGHT
	table string
	on    string
	args  []any
}

type havingClause struct {
	expr string
	args []any
}

// getModel 是链式方法的统一入口：safe=true 返回 Clone（并保留 safe 标志），false 返回自身。
func (m *Model[T]) getModel() *Model[T] {
	if !m.safe {
		return m
	}
	clone := m.Clone()
	clone.safe = true // Clone 出的 Model 也启用 safe，链式递归保持模式一致
	return clone
}

// Safe 设置安全模式。无参调用等同于 Safe(true)。
func (m *Model[T]) Safe(safe ...bool) *Model[T] {
	if len(safe) > 0 {
		m.safe = safe[0]
	} else {
		m.safe = true
	}
	return m
}

// Clone 深拷贝当前 Model，返回独立副本（不共享任何切片底层数组）。
// safe 标志位不会被拷贝——Clone 出的新 Model 始终是 unsafe 的。
func (m *Model[T]) Clone() *Model[T] {
	c := &Model[T]{
		db:           m.db,
		tx:           m.tx,
		dial:         m.dial,
		table:        m.table,
		allowAll:     m.allowAll,
		safe:         false, // Clone 总是 unsafe，避免无限递归
		withTrashed:  m.withTrashed,
		onlyTrashed:  m.onlyTrashed,
		disableHooks: m.disableHooks,
		cacheKey:     m.cacheKey,
		cacheTTL:     m.cacheTTL,
		cacheEnabled: m.cacheEnabled,
	}
	if len(m.fields) > 0 {
		c.fields = make([]string, len(m.fields))
		copy(c.fields, m.fields)
	}
	if len(m.wheres) > 0 {
		c.wheres = make([]whereClause, len(m.wheres))
		copy(c.wheres, m.wheres)
	}
	if len(m.orders) > 0 {
		c.orders = make([]string, len(m.orders))
		copy(c.orders, m.orders)
	}
	if len(m.groups) > 0 {
		c.groups = make([]string, len(m.groups))
		copy(c.groups, m.groups)
	}
	if len(m.havings) > 0 {
		c.havings = make([]havingClause, len(m.havings))
		copy(c.havings, m.havings)
	}
	if len(m.joins) > 0 {
		c.joins = make([]joinClause, len(m.joins))
		copy(c.joins, m.joins)
	}
	if len(m.preloads) > 0 {
		c.preloads = make([]preloadEntry, len(m.preloads))
		copy(c.preloads, m.preloads)
	}
	c.limitN = m.limitN
	c.offsetN = m.offsetN
	c.distinct = m.distinct
	c.lock = m.lock
	if len(m.fieldsEx) > 0 {
		c.fieldsEx = append([]string(nil), m.fieldsEx...)
	}
	c.ctx = m.ctx
	c.useMaster = m.useMaster
	if len(m.scopes) > 0 {
		c.scopes = append([]ScopeFunc[T](nil), m.scopes...)
	}
	return c
}

// Scopes 追加一个或多个查询作用域。
// 作用域在查询执行（All/Find/One/Paginate 等）前按注册顺序应用。
//
// 用法：
//
//	users, _ := db.Model[User]().Scopes(OnlyActive, Recent).All()
func (m *Model[T]) Scopes(fns ...ScopeFunc[T]) *Model[T] {
	model := m.getModel()
	model.scopes = append(model.scopes, fns...)
	return model
}

// Scope 追加单个查询作用域，等价于 Scopes(fn)。
func (m *Model[T]) Scope(fn ScopeFunc[T]) *Model[T] {
	return m.Scopes(fn)
}

// applyScopes 在执行查询前应用已注册的作用域，返回应用后的模型副本。
// 已应用的作用域被清空，避免重复应用。若未注册任何作用域则原样返回。
func (m *Model[T]) applyScopes() *Model[T] {
	if len(m.scopes) == 0 {
		return m
	}
	mm := m.Clone()
	mm.scopes = nil // 防止作用域函数体内再次触发
	for _, fn := range m.scopes {
		mm = fn(mm)
	}
	return mm
}

// tableNameOf 从类型推断表名（导出，供 relation 包内使用）。
func tableNameOf(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return toSnake(t.Name())
}

// ---- 链式条件构建 ----
// 每个链式方法都通过 getModel() 获取操作对象：
//   - 默认 unsafe → 返回自身，修改原地生效（高性能）
//   - Safe() 后 → 返回 Clone 副本，原 Model 保持不变（可复用作查询模板）

// Fields 指定 SELECT 字段（多调叠加；空参数重置为 *）。
func (m *Model[T]) Fields(cols ...string) *Model[T] {
	model := m.getModel()
	if len(cols) == 0 {
		model.fields = nil
		return model
	}
	model.fields = append(model.fields, cols...)
	return model
}

// FieldsEx 排除指定列（其余列参与 SELECT）。与 Fields 互斥——同时调用时 Fields 优先生效。
// 空参数重置为不排除（即 *）。
func (m *Model[T]) FieldsEx(cols ...string) *Model[T] {
	model := m.getModel()
	if len(cols) == 0 {
		model.fieldsEx = nil
		return model
	}
	model.fieldsEx = append(model.fieldsEx, cols...)
	return model
}

// Ctx 为当前查询绑定 context.Context（用于超时/取消透传）。返回新的 Model 副本，
// 不修改原 Model。仅在底层驱动支持 Context 调用时生效（已支持 *sql.DB/Tx）。
func (m *Model[T]) Ctx(ctx context.Context) *Model[T] {
	model := m.getModel()
	model.ctx = ctx
	return model
}

// DB 返回底层数据库句柄。用于执行原生 SQL 逃生舱（如 DB().Exec / DB().Query），
// 或调用未封装的能力。注意：直接操作 DB 会绕开 Model 的钩子/事件/软删除过滤。
func (m *Model[T]) DB() *DB { return m.db }

// Master 强制本次查询走主库（读写分离场景下，默认 SELECT 走从库）。
// 适用于刚写入后需立即读到的强一致场景。返回新的 Model 副本。
func (m *Model[T]) Master() *Model[T] {
	model := m.getModel()
	model.useMaster = true
	return model
}

// Where 添加 WHERE 条件。支持两种形式：
//   - Where("age > ? AND name = ?", 18, "bob")：占位符 ?，参数顺序绑定
//   - Where("age > ?", 18)：单条件
func (m *Model[T]) Where(expr string, args ...any) *Model[T] {
	model := m.getModel()
	model.wheres = append(model.wheres, whereClause{expr: expr, args: args})
	return model
}

// WhereEQ 便捷等值条件：WhereEQ("status", 1)。
func (m *Model[T]) WhereEQ(col string, val any) *Model[T] {
	model := m.getModel()
	return model.Where(model.dial.Quote(col)+" = ?", val)
}

// WhereMap 批量等值条件。map[string]any{"status": 1, "name": "bob"} → AND key=?, key=?
func (m *Model[T]) WhereMap(mp map[string]any) *Model[T] {
	model := m.getModel()
	for k, v := range mp {
		model.wheres = append(model.wheres, whereClause{
			expr: model.dial.Quote(k) + " = ?", args: []any{v},
		})
	}
	return model
}

// WhereIn 添加 WHERE col IN (?, ?, ...) 条件。
func (m *Model[T]) WhereIn(col string, vals ...any) *Model[T] {
	model := m.getModel()
	if len(vals) == 0 {
		return model
	}
	placeholders := make([]string, len(vals))
	for i := range vals {
		placeholders[i] = "?"
	}
	model.wheres = append(model.wheres, whereClause{
		expr: model.dial.Quote(col) + " IN (" + strings.Join(placeholders, ", ") + ")",
		args: vals,
	})
	return model
}

// WhereNotIn 添加 WHERE col NOT IN (?, ?, ...) 条件。
func (m *Model[T]) WhereNotIn(col string, vals ...any) *Model[T] {
	model := m.getModel()
	if len(vals) == 0 {
		return model
	}
	placeholders := make([]string, len(vals))
	for i := range vals {
		placeholders[i] = "?"
	}
	model.wheres = append(model.wheres, whereClause{
		expr: model.dial.Quote(col) + " NOT IN (" + strings.Join(placeholders, ", ") + ")",
		args: vals,
	})
	return model
}

// WhereOrModel 将另一个 Model 的 WHERE 条件以 OR 形式合并到当前 Model。
// 用法：m.WhereOrModel(NewModel[User](db).Where("age > ?", 30).Where("status = ?", 1))
func (m *Model[T]) WhereOrModel(other *Model[T]) *Model[T] {
	model := m.getModel()
	if len(other.wheres) == 0 {
		return model
	}
	// 将 other 的所有 WHERE 条件打包成一个 OR 组
	var parts []string
	var allArgs []any
	for _, w := range other.wheres {
		parts = append(parts, w.expr)
		allArgs = append(allArgs, w.args...)
	}
	model.wheres = append(model.wheres, whereClause{
		expr: "(" + strings.Join(parts, " AND ") + ")",
		args: allArgs,
	})
	return model
}

// Order 添加 ORDER BY 片段（如 "created_at DESC"）。
func (m *Model[T]) Order(by string) *Model[T] {
	model := m.getModel()
	model.orders = append(model.orders, by)
	return model
}

// Group 添加 GROUP BY 字段。
func (m *Model[T]) Group(cols ...string) *Model[T] {
	model := m.getModel()
	model.groups = append(model.groups, cols...)
	return model
}

// Having 添加 HAVING 条件。args 通过占位符 ? 嵌入 expr。
func (m *Model[T]) Having(expr string, args ...any) *Model[T] {
	model := m.getModel()
	model.havings = append(model.havings, havingClause{expr: expr, args: args})
	return model
}

// Limit 设置 LIMIT。
func (m *Model[T]) Limit(n int) *Model[T] {
	model := m.getModel()
	model.limitN = n
	return model
}

// Offset 设置 OFFSET。
func (m *Model[T]) Offset(n int) *Model[T] {
	model := m.getModel()
	model.offsetN = n
	return model
}

// Page 分页：page 从 1 开始，size 每页大小。
func (m *Model[T]) Page(page, size int) *Model[T] {
	model := m.getModel()
	if page < 1 {
		page = 1
	}
	model.limitN = size
	model.offsetN = (page - 1) * size
	return model
}

// Distinct 设置 SELECT DISTINCT。
func (m *Model[T]) Distinct() *Model[T] {
	model := m.getModel()
	model.distinct = true
	return model
}

// Join 内连接。
func (m *Model[T]) Join(table, on string, args ...any) *Model[T] {
	model := m.getModel()
	model.joins = append(model.joins, joinClause{typ: "INNER", table: table, on: on, args: args})
	return model
}

// LeftJoin 左连接。
func (m *Model[T]) LeftJoin(table, on string, args ...any) *Model[T] {
	model := m.getModel()
	model.joins = append(model.joins, joinClause{typ: "LEFT", table: table, on: on, args: args})
	return model
}

// RightJoin 右连接。
func (m *Model[T]) RightJoin(table, on string, args ...any) *Model[T] {
	model := m.getModel()
	model.joins = append(model.joins, joinClause{typ: "RIGHT", table: table, on: on, args: args})
	return model
}

// Table 返回当前表名（调试/动态 SQL 用）。
func (m *Model[T]) Table() string { return m.table }

// AllowAll 显式允许无 WHERE 条件的整表 Update/Delete（危险操作护栏）。
func (m *Model[T]) AllowAll() *Model[T] {
	model := m.getModel()
	model.allowAll = true
	return model
}

// NewModel 构造绑定到 DB 的泛型模型。table 可省略（从 T 推断：类型名 snake_case 或
// tdb:"table:xxx" 标签）。这是用户创建查询入口的主要方式。
func NewModel[T any](db *DB, table ...string) *Model[T] {
	return newModel[T](db, db.dial, nil, table...)
}

// NewModelTx 构造绑定到事务的泛型模型（在 DB.Tx 回调内使用）。
func NewModelTx[T any](tx *Tx, table ...string) *Model[T] {
	return newModel[T](tx.db, tx.dial, tx, table...)
}
