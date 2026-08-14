package tdb

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// runQuery 在 DB 或 Tx 上执行查询。
func (m *Model[T]) runQuery(sqlStr string, args ...any) (*sql.Rows, error) {
	if m.tx != nil {
		return m.tx.queryCtx(m.ctx, sqlStr, args...)
	}
	return m.db.queryCtx(m.ctx, m.useMaster, sqlStr, args...)
}

// runExec 在 DB 或 Tx 上执行写操作（始终走主库）。
func (m *Model[T]) runExec(sqlStr string, args ...any) (sql.Result, error) {
	if m.tx != nil {
		return m.tx.execCtx(m.ctx, sqlStr, args...)
	}
	return m.db.execCtx(m.ctx, sqlStr, args...)
}

// buildSelect 组装 SELECT 语句与参数（占位符按方言风格）。
func (m *Model[T]) buildSelect() (string, []any) {
	var b strings.Builder
	b.WriteString("SELECT ")
	if m.distinct {
		b.WriteString("DISTINCT ")
	}
	if len(m.fields) == 0 {
		if len(m.fieldsEx) == 0 {
			b.WriteString("*")
		} else {
			// 基于模型 T 的列清单排除指定列
			excluded := toSet(m.fieldsEx)
			all := metaFor(reflect.TypeOf(newZero[T]()).Elem()).allColumns()
			kept := make([]string, 0, len(all))
			for _, c := range all {
				if !excluded[c] {
					kept = append(kept, c)
				}
			}
			quoted := make([]string, len(kept))
			for i, f := range kept {
				quoted[i] = m.dial.Quote(f)
			}
			b.WriteString(strings.Join(quoted, ", "))
		}
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
	if m.lock != "" {
		b.WriteString(" ")
		b.WriteString(m.lock)
	}
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
	sdCol, _, hasSD := hasSoftDelete(&zero)

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
	m = m.applyScopes()

	zero := newZero[T]()
	// BeforeQuery hook
	if !m.disableHooks {
		if s, ok := zero.(BeforeQuerier); ok {
			if err := s.BeforeQuery(); err != nil {
				return nil, err
			}
		}
	}
	// BeforeQuery 模型事件。
	if err := m.fireModelEvent(m.ctx, EventBeforeQuery, zero, nil); err != nil {
		return nil, err
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
	// AfterQuery 模型事件（每行一个）。
	if err := m.fireModelEvent(m.ctx, EventAfterQuery, &result, nil); err != nil {
		return result, err
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
// 使用 m.ctx 透传超时/取消（避免预加载查询不受控）。
func (m *Model[T]) loadPreloads(items *[]T) error {
	db := m.sourceDB()
	if db == nil {
		return nil
	}
	if m.ctx != nil {
		db = db.Ctx(m.ctx)
	}

	// 分两层处理：先加载根级（无 "."），再加载嵌套（含 "."）。
	// 嵌套预加载必须在根级加载完成后才能执行，
	// 因为嵌套预加载操作的是根级关联的实例。

	// 第一步：根级预加载
	for _, p := range m.preloads {
		if strings.Contains(p.name, ".") {
			continue
		}
		if err := p.preloader.load(m.ctx, db, items, p.name); err != nil {
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

		if err := p.preloader.load(m.ctx, db, subItems, subName); err != nil {
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
	m = m.applyScopes()

	zero := newZero[T]()
	// BeforeQuery hook
	if !m.disableHooks {
		if s, ok := zero.(BeforeQuerier); ok {
			if err := s.BeforeQuery(); err != nil {
				return err
			}
		}
	}
	// BeforeQuery 模型事件。
	if err := m.fireModelEvent(m.ctx, EventBeforeQuery, zero, nil); err != nil {
		return err
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
	// AfterQuery 模型事件。
	if err := m.fireModelEvent(m.ctx, EventAfterQuery, dst, nil); err != nil {
		return err
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

// FindOrFail 查询单行，无结果时返回 ErrNoRows（便于上层决定 404）。
// 与 Find 的区别：Find 对「无行」静默返回零值，FindOrFail 显式报错。
func (m *Model[T]) FindOrFail() (T, error) {
	var dst T
	if err := m.One(&dst); err != nil {
		return dst, err
	}
	return dst, nil
}

// FirstOrCreate 查询首行；不存在时以 merge 合并默认值后插入并返回。
//
//	m.User().FirstOrCreate(
//	    tdb.WhereEQ("openid", openid),
//	    User{Name: "guest", Status: 1},
//	)
//
// cond 是作用于查询的条件（任意返回 *Model[T] 的链式调用）；
// defaults 是插入时的缺省值（struct 或 map）。命中时返回已存在记录。
func (m *Model[T]) FirstOrCreate(cond func(*Model[T]) *Model[T], defaults any) (T, error) {
	var zero T
	q := m
	if cond != nil {
		q = cond(m)
	}
	got, err := q.Find()
	if err == nil && !isZeroModel(got) {
		return got, nil
	}
	if _, err := m.Insert(mergeDefaults(q, defaults)); err != nil {
		return zero, err
	}
	// 重新查询命中记录（Insert 不回填完整字段，且并发下更稳）。
	return q.Find()
}

// mergeDefaults 在 defaults 之上叠加查询条件中的等值字段，
// 保证插入行满足后续查询条件（典型如唯一键）。
func mergeDefaults[T any](q *Model[T], defaults any) any {
	// 若 defaults 已是带条件的 struct/map，直接返回即可；
	// 仅当 defaults 缺主键外的等值条件时，复制 where 中的等值字段。
	if len(q.wheres) == 0 {
		return defaults
	}
	rv := reflect.ValueOf(defaults)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return defaults
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return defaults
	}
	out := reflect.New(rv.Type()).Elem()
	out.Set(rv)
	for _, w := range q.wheres {
		// 仅支持 `col = ?` 形式的简单等值条件复制。
		if eq := strings.Index(w.expr, " = ?"); eq > 0 && len(w.args) == 1 {
			col := strings.TrimSpace(w.expr[:eq])
			col = strings.Trim(col, "`\"")
			if f := out.FieldByNameFunc(func(name string) bool {
				return strings.EqualFold(name, col)
			}); f.IsValid() && f.CanSet() && isZero(f) {
				f.Set(reflect.ValueOf(w.args[0]).Convert(f.Type()))
			}
		}
	}
	return out.Interface()
}

// isZeroModel 判断 T 是否为零值（用于 FirstOrCreate 命中判定）。
func isZeroModel(v any) bool {
	rv := reflect.ValueOf(v)
	return isZero(rv)
}

// ChunkById 基于主键游标分批处理全表。
//
// 适用于大数据集遍历：每次取 size 行，按主键升序推进游标，避免一次性加载内存。
// 回调返回 error 立即终止；返回 false 也终止遍历。主键由 tdb tag 的 primaryKey
// 标识，缺失时回退到名为 id 的字段（与 Save 主键解析规则一致）。
func (m *Model[T]) ChunkById(size int, fn func(items []T) (bool, error)) error {
	if size <= 0 {
		size = 100
	}
	col, _, ok := primaryKeyOf(newZero[T]())
	if !ok {
		col = "id"
	}
	var last any
	for {
		// 每轮基于 m 克隆独立构造查询：避免 unsafe 模式下 Order/Limit 持续累积到
		// 同一 Model 导致 SQL 退化（Clone 出的新 Model 为 unsafe，可原地修改）。
		q := m.Clone().Order(col + " ASC").Limit(size)
		if last != nil {
			q = q.Where(col+" > ?", last)
		}
		items, err := q.All()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		cont, err := fn(items)
		if err != nil {
			return err
		}
		// 记录本批最后一行的主键值作为下一轮游标。
		_, last, _ = primaryKeyOf(items[len(items)-1])
		if !cont || len(items) < size {
			return nil
		}
	}
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

	// BeforeInsert 模型事件（监听器可返回错误中断插入）。
	if err := m.fireModelEvent(m.ctx, EventBeforeInsert, value, nil); err != nil {
		return nil, err
	}

	sqlStr, vals, _, err := m.buildInsert(value, false)
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
		// AfterSave hook：Save 最终落到 Insert 时，Insert 成功即 Save 成功。
		if s, ok := value.(AfterSaver); ok {
			if err := s.AfterSave(); err != nil {
				return res, err
			}
		}
	}
	// AfterInsert 模型事件。
	if err := m.fireModelEvent(m.ctx, EventAfterInsert, value, res); err != nil {
		return res, err
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

	sqlStr, vals, columns, err := m.buildInsert(value, false)
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

func (m *Model[T]) buildInsert(value any, ignore bool) (string, []any, []string, error) {
	cols, vals, err := decompose(value)
	if err != nil {
		return "", nil, nil, err
	}
	if len(cols) == 0 {
		return "", nil, nil, ErrInvalidTable
	}
	var b strings.Builder
	b.WriteString(insertKeyword(m.dial, ignore)) // INSERT / INSERT IGNORE / INSERT OR IGNORE
	b.WriteString(" ")
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
	// PostgreSQL 等使用 ON CONFLICT DO NOTHING 实现忽略冲突。
	if ignore && m.dial.Name() == "postgres" {
		b.WriteString(" ON CONFLICT DO NOTHING")
	}
	return b.String(), vals, cols, nil
}

// insertKeyword 返回 INSERT 关键字前缀：ignore 时按方言适配
// （mysql: INSERT IGNORE；sqlite: INSERT OR IGNORE；postgres: INSERT 配合 ON CONFLICT DO NOTHING）。
func insertKeyword(dial Dialect, ignore bool) string {
	if !ignore {
		return "INSERT INTO"
	}
	switch dial.Name() {
	case "mysql":
		return "INSERT IGNORE"
	case "sqlite":
		return "INSERT OR IGNORE"
	default:
		return "INSERT"
	}
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

// InsertIgnore 插入新记录，若发生唯一键/主键冲突则忽略（不报错）。
// 按方言适配：MySQL 使用 INSERT IGNORE，SQLite 使用 INSERT OR IGNORE，
// PostgreSQL 使用 INSERT ... ON CONFLICT DO NOTHING；其余方言退化为普通 INSERT。
// 注意：依赖数据库唯一约束，调用方需保证目标表存在相应约束。
func (m *Model[T]) InsertIgnore(value any) (sql.Result, error) {
	if !m.disableHooks {
		if s, ok := newZero[T]().(BeforeInserter); ok {
			if err := s.BeforeInsert(); err != nil {
				return nil, err
			}
		}
	}
	if err := m.fireModelEvent(m.ctx, EventBeforeInsert, value, nil); err != nil {
		return nil, err
	}

	sqlStr, vals, _, err := m.buildInsert(value, true)
	if err != nil {
		return nil, err
	}
	res, err := m.runExec(sqlStr, vals...)
	if err != nil {
		return nil, err
	}
	if !m.disableHooks {
		if s, ok := newZero[T]().(AfterInserter); ok {
			if err := s.AfterInsert(); err != nil {
				return res, err
			}
		}
	}
	if err := m.fireModelEvent(m.ctx, EventAfterInsert, value, res); err != nil {
		return res, err
	}
	return res, nil
}

// Save 保存记录：若主键非零值（已存在）则执行 Update，否则执行 Insert。
// 区别于单独调用 Insert/Update：Save 会触发 BeforeSave/AfterSave 钩子，
// 且依据主键自动路由，适合「有则更新、无则插入」的场景。
func (m *Model[T]) Save(value any) (sql.Result, error) {
	// BeforeSave 模型事件（监听器可返回错误中断保存）。
	if err := m.fireModelEvent(m.ctx, EventBeforeSave, value, nil); err != nil {
		return nil, err
	}

	var res sql.Result
	var err error
	if col, pk, ok := primaryKeyOf(value); ok && !isZero(reflect.ValueOf(pk)) {
		// 已存在记录：以主键为 WHERE 执行 Update，避免全表更新。
		res, err = m.WhereEQ(col, pk).Update(value)
	} else {
		res, err = m.Insert(value)
	}
	if err != nil {
		return res, err
	}

	// AfterSave 模型事件。
	if err := m.fireModelEvent(m.ctx, EventAfterSave, value, res); err != nil {
		return res, err
	}
	return res, nil
}

// primaryKeyOf 返回 value（struct 或指针）的主键列名与主键值。
// 主键由 tdb tag 的 primaryKey 标识；无 primaryKey 标记时回退到名为 id 的字段。
// 用于 Save 按主键路由与生成 WHERE 条件。
func primaryKeyOf(value any) (col string, pk any, ok bool) {
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", nil, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return "", nil, false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("tdb")
		isPK := strings.Contains(tag, "primaryKey")
		if !isPK && strings.EqualFold(f.Name, "id") && !strings.Contains(tag, "-") {
			isPK = true
		}
		if isPK {
			col = columnOf(f)
			return col, v.Field(i).Interface(), true
		}
	}
	return "", nil, false
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

	// BeforeUpdate 模型事件（监听器可返回错误中断更新）。
	if err := m.fireModelEvent(m.ctx, EventBeforeUpdate, value, nil); err != nil {
		return nil, err
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
		// AfterSave hook：Save 最终落到 Update 时，Update 成功即 Save 成功。
		if s, ok := value.(AfterSaver); ok {
			if err := s.AfterSave(); err != nil {
				return res, err
			}
		}
	}
	// AfterUpdate 模型事件。
	if err := m.fireModelEvent(m.ctx, EventAfterUpdate, value, res); err != nil {
		return res, err
	}
	return res, nil
}

// Delete 按当前 WHERE 条件删除。若 Model 包含 SoftDelete 字段则执行软删除。
func (m *Model[T]) Delete() (sql.Result, error) {
	// 检查是否启用软删除
	var zero T
	sdCol, sdKind, hasSD := hasSoftDelete(&zero)

	if hasSD {
		return m.softDeleteExec(sdCol, sdKind)
	}

	// 同一份零实例同时用于 hook 断言与模型事件，避免重复分配。
	hook := newZero[T]()
	// BeforeDelete hook
	if !m.disableHooks {
		// 尝试构造新实例调用钩子
		if s, ok := hook.(BeforeDeleter); ok {
			if err := s.BeforeDelete(); err != nil {
				return nil, err
			}
		}
	}

	// BeforeDelete 模型事件（监听器可返回错误中断删除）。
	if err := m.fireModelEvent(m.ctx, EventBeforeDelete, hook, nil); err != nil {
		return nil, err
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
		if s, ok := hook.(AfterDeleter); ok {
			if err := s.AfterDelete(); err != nil {
				return res, err
			}
		}
	}
	// AfterDelete 模型事件。
	if err := m.fireModelEvent(m.ctx, EventAfterDelete, hook, res); err != nil {
		return res, err
	}
	return res, nil
}

// softDeleteExec 执行软删除（UPDATE SET deleted_at = <当前时间>）。
// kind 为 "time"（SoftDelete，time.Time 语义）或 "int"（SoftDeleteInt，Unix 秒 int64 语义）。
//
// 实现说明：
//   - time 类型：优先使用方言的 Now() 表达式（服务端时间，时区由数据库保证）；
//     若方言未实现 NowDialect 可选扩展（如第三方自定义方言），则回退为绑定 time.Now() 参数
//     ——对所有已注册方言都安全，不再硬编码 Name() 分支，避免自定义驱动产生错误 SQL。
//   - int 类型：始终绑定 time.Now().Unix() 参数，因为软删除列是 BIGINT，SQL 时间戳函数
//     会产生类型不匹配；参数化写法跨驱动一致可用。
func (m *Model[T]) softDeleteExec(sdCol, kind string) (sql.Result, error) {
	zero := newZero[T]()
	// BeforeDelete hook（软删除同样视为删除，需与物理删除保持钩子对称）。
	if !m.disableHooks {
		if s, ok := zero.(BeforeDeleter); ok {
			if err := s.BeforeDelete(); err != nil {
				return nil, err
			}
		}
	}
	// BeforeDelete 模型事件。
	if err := m.fireModelEvent(m.ctx, EventBeforeDelete, zero, nil); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("UPDATE ")
	b.WriteString(m.dial.Quote(m.table))
	b.WriteString(" SET ")
	b.WriteString(m.dial.Quote(sdCol))
	b.WriteString(" = ")

	var args []any
	switch kind {
	case "int":
		// SoftDeleteInt：绑定 Unix 秒整数参数。
		b.WriteString(m.dial.Placeholder(0))
		args = []any{time.Now().Unix()}
	default:
		// SoftDelete（time 语义）：优先方言表达式，回退为绑定参数。
		if nd, ok := m.dial.(NowDialect); ok {
			b.WriteString(nd.Now())
		} else {
			b.WriteString(m.dial.Placeholder(0))
			args = []any{time.Now()}
		}
	}

	args = m.appendWheres(&b, args)
	if len(m.wheres) == 0 && !m.allowAll {
		return nil, ErrNoWhere
	}
	res, err := m.runExec(b.String(), args...)
	if err != nil {
		return res, err
	}

	// AfterDelete hook
	if !m.disableHooks {
		if s, ok := zero.(AfterDeleter); ok {
			if err := s.AfterDelete(); err != nil {
				return res, err
			}
		}
	}
	// AfterDelete 模型事件（软删除视为删除，便于统一监听）。
	if err := m.fireModelEvent(m.ctx, EventAfterDelete, zero, res); err != nil {
		return res, err
	}
	return res, nil
}

// Restore 恢复软删除记录（SET deleted_at = NULL）。
func (m *Model[T]) Restore() (sql.Result, error) {
	var zero T
	sdCol, _, hasSD := hasSoftDelete(&zero)
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
