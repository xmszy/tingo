package tdb

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// 常用的悲观锁表达式，可直接传给 Lock()。
const (
	LockForUpdate  = "FOR UPDATE"
	LockForUpdateNoWait = "FOR UPDATE NOWAIT"
	LockForUpdateSkipLocked = "FOR UPDATE SKIP LOCKED"
	LockShare      = "FOR SHARE"
	LockInShareMode = "LOCK IN SHARE MODE" // MySQL 旧式共享锁
)

// Lock 追加悲观锁子句到当前 Model（仅对 SELECT 生效）。
//
// 常用取值见本包导出的 LockForUpdate / LockForUpdateNoWait /
// LockForUpdateSkipLocked / LockShare / LockInShareMode。
// 传空串可清除锁（返回新的无锁 Model）。
//
// 用法：
//
//	var users []User
//	m.Table("user").Lock(LockForUpdate).Where("balance > ?", 0).Scan(&users)
//	// SELECT * FROM `user` WHERE balance > ? FOR UPDATE
func (m *Model[T]) Lock(lock string) *Model[T] {
	n := m.Clone()
	n.lock = strings.TrimSpace(lock)
	return n
}

// BatchInsert 批量插入多条记录，单条多值 INSERT（比循环 Insert 显著减少往返）。
//
// list 必须是 []T 或 *[]T（T 为模型结构体）。可选 batch 指定每批最大行数，
// 超过则分多批执行；默认 batch=10（0 表示不分批，一次插入所有行）。
//
// 注意：
//   - 自动时间戳（created_at/updated_at）仅在单条 Insert 中注入；
//     BatchInsert 需由调用方预先在结构体中填好时间字段。
//   - 自增主键由数据库生成，不会回写。
func (m *Model[T]) BatchInsert(list any, batch ...int) (int64, error) {
	rv := reflect.ValueOf(list)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0, ErrInvalidValue
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return 0, ErrInvalidValue
	}
	n := rv.Len()
	if n == 0 {
		return 0, nil
	}

	// 统一列集合：取元素类型的全部可写列（不跳过零值，保证批量行对齐）。
	elemType := rv.Index(0).Type()
	for elemType.Kind() == reflect.Pointer {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return 0, ErrInvalidValue
	}
	meta := metaFor(elemType)
	cols := make([]string, 0, len(meta.colIndex))
	for c := range meta.colIndex {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	if len(cols) == 0 {
		return 0, ErrInvalidValue
	}

	batchSize := 10
	if len(batch) > 0 && batch[0] > 0 {
		batchSize = batch[0]
	}

	table := m.dial.Quote(m.table)
	dial := m.db.Dialect()

	// 预构建列引用片段（INSERT INTO t (c1,c2)）。
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = dial.Quote(c)
	}
	colClause := strings.Join(quotedCols, ",")

	var total int64
	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}
		affected, err := m.batchInsertChunk(rv, start, end, table, colClause, cols, meta, dial)
		if err != nil {
			return total, err
		}
		total += affected
	}
	return total, nil
}

// batchInsertChunk 插入 [start,end) 区间的一批行。
func (m *Model[T]) batchInsertChunk(rv reflect.Value, start, end int, table, colClause string, cols []string, meta *structMeta, dial Dialect) (int64, error) {
	// 构建占位符：每行列数相同的 (?,?),(?,?)
	var valueRows []string
	var args []any
	argIndex := 0
	for r := start; r < end; r++ {
		row := rv.Index(r)
		if row.Kind() == reflect.Pointer {
			if row.IsNil() {
				return 0, ErrInvalidValue
			}
			row = row.Elem()
		}
		placeholders := make([]string, len(cols))
		for i, c := range cols {
			idx, ok := meta.colIndex[c]
			if !ok {
				return 0, fmt.Errorf("tdb: column %q not found in struct %s", c, row.Type())
			}
			placeholders[i] = dial.Placeholder(argIndex)
			args = append(args, row.Field(idx).Interface())
			argIndex++
		}
		valueRows = append(valueRows, "("+strings.Join(placeholders, ",")+")")
	}

	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, colClause, strings.Join(valueRows, ","))
	res, err := m.db.exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ScanList 把原始查询结果（[]map[string]any，通常由 All/Scan 得到）按关联层级
// 组装进嵌套结构体切片。对标 gf 的 Model.ScanList。
//
// 参数：
//   - list  : 源数据，必须是 []map[string]any 或 *[]map[string]any。
//   - ptr   : 目标指针，如 *[]User（User 内嵌 Profile *Profile / Orders []Order 等关联字段）。
//   - relation: 关联字段名（struct 字段名，非列名），如 "Profile"、"Orders"。
//
// 组装规则：
//   - ptr 元素类型中名为 relation 的字段必须是 *R（hasOne）或 []R/*[]R（hasMany）。
//   - 每一行按关联字段匹配键（默认 relation 字段主键名 == relation+"Id" 的列，
//     如 Profile 用 profile_id；可用 relationKey 覆盖）。找不到则跳过该行关联填充。
//
// 典型用途：一次联表查询后，在内存中按归属键拼装出树状结构，避免多次 SQL。
func (m *Model[T]) ScanList(list any, ptr any, relation string, relationKey ...string) error {
	srcMaps, err := toMapSlice(list)
	if err != nil {
		return err
	}
	dstPtr := reflect.ValueOf(ptr)
	if dstPtr.Kind() != reflect.Pointer || dstPtr.IsNil() {
		return ErrInvalidValue
	}
	dstSlice := dstPtr.Elem()
	if dstSlice.Kind() != reflect.Slice {
		return ErrInvalidValue
	}
	elemType := dstSlice.Type().Elem()
	// 支持 []*Elem 与 []Elem 两种目标
	baseType := elemType
	if baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}
	if baseType.Kind() != reflect.Struct {
		return ErrInvalidValue
	}
	// 关联字段在目标元素中的索引（用反射直接查找，兼容 tdb:"-" 的嵌套字段）。
	relField, ok := baseType.FieldByName(relation)
	if !ok {
		return fmt.Errorf("tdb: relation field %q not found in %s", relation, baseType)
	}
	relFieldIdx := relField.Index[0]

	// 关联目标类型 R
	relType := relField.Type
	isSlice := relType.Kind() == reflect.Slice
	relBase := relType
	if isSlice {
		relBase = relType.Elem()
	}
	if relBase.Kind() == reflect.Pointer {
		relBase = relBase.Elem()
	}
	relMeta := metaFor(relBase)

	// 源行外键列名：默认 relation 的小写下划线形式 + "_id"（如 "user_id"），
	// 可通过 relationKey[0] 覆盖。
	srcKey := toSnake(relation) + "_id"
	if len(relationKey) > 0 && relationKey[0] != "" {
		srcKey = relationKey[0]
	}

	for i := 0; i < dstSlice.Len(); i++ {
		elem := dstSlice.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		// 父行主键列值（默认 "id"；可用 relationKey[1] 覆盖父键列名）。
		parentKeyCol := "id"
		if len(relationKey) > 1 && relationKey[1] != "" {
			parentKeyCol = relationKey[1]
		}
		parentKeyVal := lookupColValue(elem, parentKeyCol)
		if parentKeyVal == nil {
			continue
		}
		var matched []map[string]any
		for _, sm := range srcMaps {
			if v, ok := sm[srcKey]; ok && equalAny(v, parentKeyVal) {
				matched = append(matched, sm)
			}
		}
		if len(matched) == 0 {
			continue
		}
		target := elem.Field(relFieldIdx)
		if isSlice {
			slice := reflect.MakeSlice(relType, len(matched), len(matched))
			for j, sm := range matched {
				rv := reflect.New(relBase).Elem()
				fillStructFromMap(rv, sm, relMeta)
				if relType.Elem().Kind() == reflect.Pointer {
					slice.Index(j).Set(reflect.New(relBase))
					slice.Index(j).Elem().Set(rv)
				} else {
					slice.Index(j).Set(rv)
				}
			}
			target.Set(slice)
		} else {
			rv := reflect.New(relBase).Elem()
			fillStructFromMap(rv, matched[0], relMeta)
			if relField.Type.Kind() == reflect.Pointer {
				ptrVal := reflect.New(relBase)
				ptrVal.Elem().Set(rv)
				target.Set(ptrVal)
			} else {
				target.Set(rv)
			}
		}
	}
	return nil
}

// AllMaps 执行当前查询，返回 []map[string]any（列名→值）。
// 适合需要拿到原始行（如联表结果）再用 ScanList 在内存中组装嵌套结构的场景。
func (m *Model[T]) AllMaps() ([]map[string]any, error) {
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
	return rowsToMaps(rows)
}

// toMapSlice 把 list（[]map[string]any 或 *[]map[string]any）规范化。
func toMapSlice(list any) ([]map[string]any, error) {
	rv := reflect.ValueOf(list)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, ErrInvalidValue
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return nil, ErrInvalidValue
	}
	out := make([]map[string]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		m, ok := rv.Index(i).Interface().(map[string]any)
		if !ok {
			return nil, ErrInvalidValue
		}
		out[i] = m
	}
	return out, nil
}

// lookupColValue 在结构体元素上按列名查找字段值（兼容 tdb:"-" 字段）。
// 找不到或为零值返回 nil。
func lookupColValue(v reflect.Value, col string) any {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if columnOf(f) == col {
			fv := v.Field(i)
			if isZero(fv) {
				return nil
			}
			return fv.Interface()
		}
	}
	return nil
}

// fillStructFromMap 把 map 的列值填充进结构体（按列名匹配）。
func fillStructFromMap(v reflect.Value, m map[string]any, meta *structMeta) {
	for col, idx := range meta.colIndex {
		raw, ok := m[col]
		if !ok {
			continue
		}
		assignField(v.Field(idx), raw)
	}
}

// equalAny 比较两个关联键是否相等（处理 []byte / 数值 / 字符串边界）。
func equalAny(a, b any) bool {
	av := normalizeKey(a)
	bv := normalizeKey(b)
	return av == bv
}

func normalizeKey(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

// Value 查询单列首行值。col 指定列名（如 "name"）。
// 无结果时返回 (nil, nil)。
func (m *Model[T]) Value(col string) (any, error) {
	if col == "" {
		return nil, ErrInvalidValue
	}
	rows, err := m.Fields(col).Limit(1).AllMaps()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0][col], nil
}

// Array 查询单列全部值，返回 []any（顺序即查询结果顺序）。
func (m *Model[T]) Array(col string) ([]any, error) {
	if col == "" {
		return nil, ErrInvalidValue
	}
	rows, err := m.Fields(col).AllMaps()
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, r[col])
	}
	return out, nil
}

// Chunk 分批游标：每批取 size 条，调用 f 处理。处理中发生错误立即中断并返回该错误。
// 适用于大数据集遍历，避免一次性加载内存。
func (m *Model[T]) Chunk(size int, f func(records []T) error) error {
	if size <= 0 {
		size = 100
	}
	page := 0
	for {
		batch, err := m.Offset(page * size).Limit(size).All()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		if err := f(batch); err != nil {
			return err
		}
		if len(batch) < size {
			return nil
		}
		page++
	}
}

// toSet 把字符串切片转为 set。
func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it] = true
	}
	return s
}
