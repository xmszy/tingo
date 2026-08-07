package tdb

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// runQuery 在 DB 或 Tx 上执行查询。
func (m *Model[T]) runQuery(sqlStr string, args ...any) (*sql.Rows, error) {
	if m.tx != nil {
		return m.tx.query(sqlStr, args...)
	}
	return m.db.query(sqlStr, args...)
}

// runExec 在 DB 或 Tx 上执行写操作。
func (m *Model[T]) runExec(sqlStr string, args ...any) (sql.Result, error) {
	if m.tx != nil {
		return m.tx.exec(sqlStr, args...)
	}
	return m.db.exec(sqlStr, args...)
}

// buildSelect 组装 SELECT 语句与参数（占位符按方言风格）。
func (m *Model[T]) buildSelect() (string, []any) {
	var b strings.Builder
	b.WriteString("SELECT ")
	if m.distinct {
		b.WriteString("DISTINCT ")
	}
	if len(m.fields) == 0 {
		b.WriteString("*")
	} else {
		quoted := make([]string, len(m.fields))
		for i, f := range m.fields {
			quoted[i] = m.dial.Quote(f)
		}
		b.WriteString(strings.Join(quoted, ", "))
	}
	b.WriteString(" FROM ")
	b.WriteString(m.dial.Quote(m.table))

	args := m.appendJoins(&b)
	args = m.appendWheres(&b, args)
	args = append(args, m.appendGroups(&b)...)
	m.appendOrders(&b)
	b.WriteString(m.dial.Limit(m.limitN, m.offsetN))
	return b.String(), args
}

func (m *Model[T]) appendJoins(b *strings.Builder) []any {
	var args []any
	for _, j := range m.joins {
		b.WriteString(" ")
		b.WriteString(j.typ)
		b.WriteString(" JOIN ")
		b.WriteString(m.dial.Quote(j.table))
		b.WriteString(" ON ")
		b.WriteString(j.on)
		args = append(args, j.args...)
	}
	return args
}

func (m *Model[T]) appendWheres(b *strings.Builder, args []any) []any {
	// 软删除过滤
	var zero T
	sdCol, hasSD := hasSoftDelete(&zero)

	var sdExpr string
	if hasSD && !m.withTrashed && !m.onlyTrashed {
		sdExpr = softDeleteWhere(sdCol, m.dial)
	} else if hasSD && m.onlyTrashed {
		sdExpr = m.dial.Quote(sdCol) + " IS NOT NULL"
	}

	allCount := len(m.wheres)
	if sdExpr != "" {
		allCount++
	}
	if allCount == 0 {
		return args
	}

	b.WriteString(" WHERE ")
	parts := make([]string, 0, allCount)
	if sdExpr != "" {
		parts = append(parts, sdExpr)
	}
	for _, w := range m.wheres {
		parts = append(parts, w.expr)
		args = append(args, w.args...)
	}
	b.WriteString(strings.Join(parts, " AND "))
	return args
}

func (m *Model[T]) appendGroups(b *strings.Builder) []any {
	if len(m.groups) == 0 {
		return nil
	}
	quoted := make([]string, len(m.groups))
	for i, g := range m.groups {
		quoted[i] = m.dial.Quote(g)
	}
	b.WriteString(" GROUP BY ")
	b.WriteString(strings.Join(quoted, ", "))
	if len(m.havings) > 0 {
		b.WriteString(" HAVING ")
		var havingArgs []any
		for i, h := range m.havings {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(h.expr)
			havingArgs = append(havingArgs, h.args...)
		}
		return havingArgs
	}
	return nil
}

func (m *Model[T]) appendOrders(b *strings.Builder) {
	if len(m.orders) == 0 {
		return
	}
	b.WriteString(" ORDER BY ")
	b.WriteString(strings.Join(m.orders, ", "))
}

// All 查询多行，返回 []T。
func (m *Model[T]) All() ([]T, error) {
	// BeforeQuery hook
	if !m.disableHooks {
		hook := newZero[T]()
		if s, ok := hook.(BeforeQuerier); ok {
			if err := s.BeforeQuery(); err != nil {
				return nil, err
			}
		}
	}

	sqlStr, args := m.buildSelect()
	rows, err := m.runQuery(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	result, err := rowsToModels[T](rows)
	if err != nil {
		return nil, err
	}

	// AfterQuery hooks per row
	if !m.disableHooks {
		for i := range result {
			if s, ok := any(&result[i]).(AfterQuerier); ok {
				if err := s.AfterQuery(); err != nil {
					return result, err
				}
			}
		}
	}

	// 预加载关联
	if len(m.preloads) > 0 {
		if err := m.loadPreloads(&result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// loadPreloads 执行所有关联预加载（支持嵌套，如 "Profile.Photos"）。
func (m *Model[T]) loadPreloads(items *[]T) error {
	db := m.sourceDB()
	if db == nil {
		return nil
	}

	// 分两层处理：先加载根级（无 "."），再加载嵌套（含 "."）。
	// 嵌套预加载必须在根级加载完成后才能执行，
	// 因为嵌套预加载操作的是根级关联的实例。

	// 第一步：根级预加载
	for _, p := range m.preloads {
		if strings.Contains(p.name, ".") {
			continue
		}
		if err := p.preloader.load(context.Background(), db, items, p.name); err != nil {
			return err
		}
	}

	// 第二步：嵌套预加载
	for _, p := range m.preloads {
		if !strings.Contains(p.name, ".") {
			continue
		}
		// "Profile.Photos" → root="Profile", sub="Photos"
		parts := strings.SplitN(p.name, ".", 2)
		rootName := parts[0]
		subName := parts[1]

		// 收集根级关联的所有实例（指针形式，以便修改回写）
		subItems, err := collectNestedItems(items, rootName)
		if err != nil {
			return fmt.Errorf("tdb: nested preload %q: %w", p.name, err)
		}
		if subItems == nil {
			continue
		}

		if err := p.preloader.load(context.Background(), db, subItems, subName); err != nil {
			return err
		}
	}

	return nil
}

// sourceDB 返回底层 DB（无论是通过 DB 还是 Tx 访问）。
func (m *Model[T]) sourceDB() *DB {
	if m.tx != nil {
		return m.tx.db
	}
	return m.db
}

// One 查询单行，写入 dst。无行返回 ErrNoRows。
func (m *Model[T]) One(dst *T) error {
	// BeforeQuery hook
	if !m.disableHooks {
		hook := newZero[T]()
		if s, ok := hook.(BeforeQuerier); ok {
			if err := s.BeforeQuery(); err != nil {
				return err
			}
		}
	}

	sqlStr, args := m.buildSelect()
	// 隐式 LIMIT 1（除非已显式设置）。
	if m.limitN <= 0 {
		sqlStr += m.dial.Limit(1, 0)
	}
	rows, err := m.runQuery(sqlStr, args...)
	if err != nil {
		return err
	}
	ok, err := rowToModel(rows, dst)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoRows
	}

	// AfterQuery hook
	if !m.disableHooks {
		if s, ok := any(dst).(AfterQuerier); ok {
			if err := s.AfterQuery(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Scan 同 One，但返回 T 副本（无行返回零值与 ErrNoRows）。
func (m *Model[T]) Scan() (T, error) {
	var dst T
	err := m.One(&dst)
	return dst, err
}

// Find 同 Scan，但无行时返回零值且无错误（便于可选查询）。
func (m *Model[T]) Find() (T, error) {
	var dst T
	err := m.One(&dst)
	if err == ErrNoRows {
		return dst, nil
	}
	return dst, err
}

// Count 返回匹配行数。
func (m *Model[T]) Count() (int64, error) {
	var b strings.Builder
	b.WriteString("SELECT COUNT(*) FROM ")
	b.WriteString(m.dial.Quote(m.table))
	args := m.appendJoins(&b)
	args = m.appendWheres(&b, args)
	rows, err := m.runQuery(b.String(), args...)
	if err != nil {
		return 0, err
	}
	var n int64
	ok, err := rowToScalar(rows, &n)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoRows
	}
	return n, nil
}

// Exists 是否存在匹配行。
func (m *Model[T]) Exists() (bool, error) {
	n, err := m.Count()
	return n > 0, err
}

// rowToScalar 扫描单行单列标量（如 COUNT）。
func rowToScalar(rows *sql.Rows, dst any) (bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	if err := rows.Scan(dst); err != nil {
		return false, err
	}
	return true, nil
}

// ---- 写操作 ----

// Insert 插入一条记录（struct 或 map）。返回自增 ID（若驱动支持）。
func (m *Model[T]) Insert(value any) (sql.Result, error) {
	// Auto-timestamp
	if m.autoCreateTime != "" {
		injectAutoTimestamp(value, m.autoCreateTime)
	}
	if m.autoUpdateTime != "" {
		injectAutoTimestamp(value, m.autoUpdateTime)
	}

	// BeforeSave hook
	if !m.disableHooks {
		if s, ok := value.(BeforeSaver); ok {
			if err := s.BeforeSave(); err != nil {
				return nil, err
			}
		}
	}
	// BeforeInsert hook
	if !m.disableHooks {
		if s, ok := value.(BeforeInserter); ok {
			if err := s.BeforeInsert(); err != nil {
				return nil, err
			}
		}
	}

	sqlStr, vals, _, err := m.buildInsert(value)
	if err != nil {
		return nil, err
	}
	res, err := m.runExec(sqlStr, vals...)
	if err != nil {
		return nil, err
	}

	// AfterInsert hook
	if !m.disableHooks {
		if s, ok := value.(AfterInserter); ok {
			if err := s.AfterInsert(); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// Upsert 插入记录，并在唯一键冲突时更新非冲突列。
// PostgreSQL/SQLite 必须传 conflictColumns；MySQL 由数据库唯一键自动识别冲突目标。
func (m *Model[T]) Upsert(value any, conflictColumns ...string) (sql.Result, error) {
	// Auto-timestamp
	if m.autoCreateTime != "" {
		injectAutoTimestamp(value, m.autoCreateTime)
	}
	if m.autoUpdateTime != "" {
		injectAutoTimestamp(value, m.autoUpdateTime)
	}

	sqlStr, vals, columns, err := m.buildInsert(value)
	if err != nil {
		return nil, err
	}
	clause, err := m.db.BuildUpsertClause(UpsertSpec{
		ConflictColumns: conflictColumns,
		UpdateColumns:   withoutColumns(columns, conflictColumns),
	})
	if err != nil {
		return nil, err
	}
	return m.runExec(sqlStr+clause, vals...)
}

func (m *Model[T]) buildInsert(value any) (string, []any, []string, error) {
	cols, vals, err := decompose(value)
	if err != nil {
		return "", nil, nil, err
	}
	if len(cols) == 0 {
		return "", nil, nil, ErrInvalidTable
	}
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(m.dial.Quote(m.table))
	b.WriteString(" (")
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = m.dial.Quote(c)
	}
	b.WriteString(strings.Join(quoted, ", "))
	b.WriteString(") VALUES (")
	ph := make([]string, len(cols))
	for i := range cols {
		ph[i] = m.dial.Placeholder(i)
	}
	b.WriteString(strings.Join(ph, ", "))
	b.WriteString(")")
	return b.String(), vals, cols, nil
}

func withoutColumns(columns, excluded []string) []string {
	if len(excluded) == 0 {
		return append([]string(nil), columns...)
	}
	skip := make(map[string]struct{}, len(excluded))
	for _, column := range excluded {
		skip[column] = struct{}{}
	}
	filtered := make([]string, 0, len(columns))
	for _, column := range columns {
		if _, exists := skip[column]; !exists {
			filtered = append(filtered, column)
		}
	}
	return filtered
}

// InsertIgnore 同 Insert，但冲突忽略（MySQL: INSERT IGNORE）。
func (m *Model[T]) InsertIgnore(value any) (sql.Result, error) {
	res, err := m.Insert(value)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Update 按当前 WHERE 条件更新。value 为 struct（非零字段）或 map[string]any。
func (m *Model[T]) Update(value any) (sql.Result, error) {
	// Auto-timestamp（仅 update_time）
	if m.autoUpdateTime != "" {
		injectAutoTimestamp(value, m.autoUpdateTime)
	}

	// BeforeSave hook
	if !m.disableHooks {
		if s, ok := value.(BeforeSaver); ok {
			if err := s.BeforeSave(); err != nil {
				return nil, err
			}
		}
	}
	// BeforeUpdate hook
	if !m.disableHooks {
		if s, ok := value.(BeforeUpdater); ok {
			if err := s.BeforeUpdate(); err != nil {
				return nil, err
			}
		}
	}

	cols, vals, err := decompose(value)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, ErrInvalidTable
	}
	var b strings.Builder
	b.WriteString("UPDATE ")
	b.WriteString(m.dial.Quote(m.table))
	b.WriteString(" SET ")
	setCols := make([]string, len(cols))
	for i, c := range cols {
		setCols[i] = m.dial.Quote(c) + " = " + m.dial.Placeholder(i)
	}
	b.WriteString(strings.Join(setCols, ", "))
	args := m.appendWheres(&b, vals)
	if len(m.wheres) == 0 && !m.allowAll {
		// 安全护栏：无 WHERE 的整表更新需显式调用 AllowAll。
		return nil, ErrNoWhere
	}
	res, err := m.runExec(b.String(), args...)
	if err != nil {
		return nil, err
	}

	// AfterUpdate hook
	if !m.disableHooks {
		if s, ok := value.(AfterUpdater); ok {
			if err := s.AfterUpdate(); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// Delete 按当前 WHERE 条件删除。若 Model 包含 SoftDelete 字段则执行软删除。
func (m *Model[T]) Delete() (sql.Result, error) {
	// 检查是否启用软删除
	var zero T
	sdCol, hasSD := hasSoftDelete(&zero)

	if hasSD {
		return m.softDeleteExec(sdCol)
	}

	// BeforeDelete hook
	if !m.disableHooks {
		// 尝试构造新实例调用钩子
		if s, ok := newZero[T]().(BeforeDeleter); ok {
			if err := s.BeforeDelete(); err != nil {
				return nil, err
			}
		}
	}

	var b strings.Builder
	b.WriteString("DELETE FROM ")
	b.WriteString(m.dial.Quote(m.table))
	args := m.appendWheres(&b, nil)
	if len(m.wheres) == 0 && !m.allowAll {
		return nil, ErrNoWhere
	}
	res, err := m.runExec(b.String(), args...)
	if err != nil {
		return nil, err
	}

	// AfterDelete hook
	if !m.disableHooks {
		if s, ok := newZero[T]().(AfterDeleter); ok {
			if err := s.AfterDelete(); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// softDeleteExec 执行软删除（UPDATE SET deleted_at = NOW()）。
func (m *Model[T]) softDeleteExec(sdCol string) (sql.Result, error) {
	var b strings.Builder
	b.WriteString("UPDATE ")
	b.WriteString(m.dial.Quote(m.table))
	b.WriteString(" SET ")
	b.WriteString(m.dial.Quote(sdCol))
	b.WriteString(" = ")
	switch m.dial.Name() {
	case "mysql":
		b.WriteString("NOW()")
	case "postgres":
		b.WriteString("CURRENT_TIMESTAMP")
	case "sqlite":
		b.WriteString("datetime('now')")
	default:
		b.WriteString("CURRENT_TIMESTAMP")
	}

	args := m.appendWheres(&b, nil)
	if len(m.wheres) == 0 && !m.allowAll {
		return nil, ErrNoWhere
	}
	return m.runExec(b.String(), args...)
}

// Restore 恢复软删除记录（SET deleted_at = NULL）。
func (m *Model[T]) Restore() (sql.Result, error) {
	var zero T
	sdCol, hasSD := hasSoftDelete(&zero)
	if !hasSD {
		return nil, ErrInvalidTable
	}

	var b strings.Builder
	b.WriteString("UPDATE ")
	b.WriteString(m.dial.Quote(m.table))
	b.WriteString(" SET ")
	b.WriteString(m.dial.Quote(sdCol))
	b.WriteString(" = NULL")
	args := m.appendWheres(&b, nil)
	if len(m.wheres) == 0 && !m.allowAll {
		return nil, ErrNoWhere
	}
	return m.runExec(b.String(), args...)
}

// ForceDelete 强制执行物理删除，忽略软删除。
func (m *Model[T]) ForceDelete() (sql.Result, error) {
	var b strings.Builder
	b.WriteString("DELETE FROM ")
	b.WriteString(m.dial.Quote(m.table))
	args := m.appendWheres(&b, nil)
	if len(m.wheres) == 0 && !m.allowAll {
		return nil, ErrNoWhere
	}
	return m.runExec(b.String(), args...)
}

// newZero 创建 T 类型的零值指针。
func newZero[T any]() any {
	var zero T
	return &zero
}

// decompose 把 struct 或 map 拆成列名与值切片（跳过零值字段）。
// struct 模式仅取非零字段（便于部分更新）；map 模式取全部键值。
func decompose(value any) ([]string, []any, error) {
	switch v := value.(type) {
	case map[string]any:
		cols := make([]string, 0, len(v))
		for key := range v {
			cols = append(cols, key)
		}
		sort.Strings(cols)
		vals := make([]any, len(cols))
		for i, key := range cols {
			vals[i] = v[key]
		}
		return cols, vals, nil
	default:
		return decomposeStruct(value)
	}
}
