package tdb

import (
	"database/sql"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// fieldMeta 缓存结构体的列映射元信息（仅反射一次）。
type fieldMeta struct {
	index  int    // 结构体字段索引
	column string // 对应数据库列名
}

// structMeta 预计算的完整结构体元信息（线程安全缓存）。
type structMeta struct {
	fields       []fieldMeta    // 可扫描字段列表
	colIndex     map[string]int // 列名 → 字段索引，O(1) 查找
	fieldIndex   map[string]int // 字段名 → 字段索引，供 findField 查表
	softDeleteCol string        // 软删除列名（空=无）
}

// allColumns 返回模型包含的全部列名（用于 FieldsEx 排除场景）。
func (sm *structMeta) allColumns() []string {
	cols := make([]string, 0, len(sm.fields))
	for _, f := range sm.fields {
		cols = append(cols, f.column)
	}
	return cols
}

var (
	metaMu      sync.RWMutex
	metaCache   = map[reflect.Type]*structMeta{}
)

// columnOf 解析字段的数据库列名（优先 tdb:"col" / db:"col" / json:"col"，否则 snake_case）。
func columnOf(f reflect.StructField) string {
	for _, tag := range []string{"tdb", "db", "json"} {
		if v := f.Tag.Get(tag); v != "" {
			name := v
			if i := strings.IndexByte(name, ','); i >= 0 {
				name = name[:i]
			}
			if name != "" && name != "-" {
				return name
			}
			if name == "-" {
				return ""
			}
		}
	}
	return toSnake(f.Name)
}

// toSnake 把 CamelCase 转为 snake_case。
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// metaFor 获取（并缓存）T 的结构体元信息，线程安全。
func metaFor(t reflect.Type) *structMeta {
	metaMu.RLock()
	m, ok := metaCache[t]
	metaMu.RUnlock()
	if ok {
		return m
	}

	metaMu.Lock()
	defer metaMu.Unlock()
	// 双重检查
	if m, ok = metaCache[t]; ok {
		return m
	}

	fields := make([]fieldMeta, 0, t.NumField())
	colIdx := make(map[string]int, t.NumField())
	fieldIdx := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // 非导出字段跳过
			continue
		}
		col := columnOf(f)
		if col == "" {
			continue
		}
		fields = append(fields, fieldMeta{index: i, column: col})
		colIdx[col] = i
		fieldIdx[f.Name] = i
	}

	sdCol, _ := softDeleteField(t)
	m = &structMeta{fields: fields, colIndex: colIdx, fieldIndex: fieldIdx, softDeleteCol: sdCol}
	metaCache[t] = m
	return m
}

// rowsToModels 把多行扫描进 []T。
func rowsToModels[T any](rows *sql.Rows) ([]T, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	meta := metaFor(reflect.TypeFor[T]())
	var out []T
	for rows.Next() {
		ptrs := make([]any, len(cols))
		for i := range cols {
			ptrs[i] = new(any)
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		var v T
		vv := reflect.ValueOf(&v).Elem()
		for i, c := range cols {
			if idx, ok := meta.colIndex[c]; ok {
				assignField(vv.Field(idx), *(ptrs[i].(*any)))
			}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// rowsToMaps 扫描所有行，返回 []map[string]any（列名→值）。用于 ScanList 等
// 需要在内存中按关联键重新组装的场景。
func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		ptrs := make([]any, len(cols))
		for i := range cols {
			ptrs[i] = new(any)
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = *(ptrs[i].(*any))
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// rowToModel 扫描单行进 *T（无行返回 false）。
func rowToModel[T any](rows *sql.Rows, dst *T) (bool, error) {
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	meta := metaFor(reflect.TypeOf(dst).Elem())
	ptrs := make([]any, len(cols))
	for i := range cols {
		ptrs[i] = new(any)
	}
	if err := rows.Scan(ptrs...); err != nil {
		return false, err
	}
	vv := reflect.ValueOf(dst).Elem()
	for i, c := range cols {
		if idx, ok := meta.colIndex[c]; ok {
			assignField(vv.Field(idx), *(ptrs[i].(*any)))
		}
	}
	return true, nil
}

// assignField 把数据库返回值（通常是 []byte / string / 数值 / nil）赋给结构体字段，
// 处理 *sql.NullX 与目标类型的兼容。
func assignField(field reflect.Value, raw any) {
	if !field.CanSet() {
		return
	}
	if raw == nil {
		return
	}
	rv := reflect.ValueOf(raw)
	// 数据库常见返回：[]byte（mysql driver）、string、int64、float64、bool、time.Time。
	switch field.Kind() {
	case reflect.Pointer:
		// 解引用一层指针：若 raw 已是同类型指针则优先直接采用，避免对 **T 无限递归。
		if rv.Type().AssignableTo(field.Type()) {
			field.Set(rv)
			return
		}
		if rv.Type().ConvertibleTo(field.Type()) {
			field.Set(rv.Convert(field.Type()))
			return
		}
		elem := reflect.New(field.Type().Elem())
		if rv.Kind() == reflect.Pointer {
			// raw 是指针：把其指向的值赋给目标元素
			assignField(elem.Elem(), rv.Elem().Interface())
		} else {
			assignField(elem.Elem(), raw)
		}
		field.Set(elem)
		return
	case reflect.String:
		field.SetString(asString(raw))
		return
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(asInt(raw))
		return
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(uint64(asInt(raw)))
		return
	case reflect.Float32, reflect.Float64:
		field.SetFloat(asFloat(raw))
		return
	case reflect.Bool:
		field.SetBool(asBool(raw))
		return
	}
	// 复杂类型（time.Time、自定义）走 reflect 直接设置。
	if rv.Type().AssignableTo(field.Type()) {
		field.Set(rv)
		return
	}
	if rv.Type().ConvertibleTo(field.Type()) {
		field.Set(rv.Convert(field.Type()))
	}
}

// asString 把数据库值转字符串。
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case nil:
		return ""
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

func asInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case []byte:
		n, _ := parseIntBytes(x)
		return n
	case string:
		n, _ := parseIntBytes([]byte(x))
		return n
	default:
		return 0
	}
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case []byte:
		f, _ := parseFloatBytes(x)
		return f
	case string:
		f, _ := parseFloatBytes([]byte(x))
		return f
	default:
		return 0
	}
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case []byte:
		return string(x) == "1" || string(x) == "true"
	case string:
		return x == "1" || x == "true"
	default:
		return false
	}
}

func parseIntBytes(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, errInvalidInt
	}
	// 处理可选正负号
	i := 0
	neg := false
	switch b[0] {
	case '-':
		neg = true
		i = 1
	case '+':
		i = 1
	}
	var n int64
	for ; i < len(b); i++ {
		c := b[i]
		if c < '0' || c > '9' {
			return 0, errInvalidInt
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

func parseFloatBytes(b []byte) (float64, error) {
	f, err := parseFloat(string(b))
	return f, err
}
