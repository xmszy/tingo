package tdb

import (
	"reflect"
	"sort"
	"time"
)

// decomposeStruct 把 struct（或指针）拆成列名与值（跳过零值字段，便于部分更新）。
// 返回值已按列名字典序排序，保证同输入生成稳定 SQL（利于缓存/可读）。
func decomposeStruct(value any) ([]string, []any, error) {
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, nil, ErrInvalidTable
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, nil, ErrInvalidTable
	}
	t := v.Type()
	type kv struct {
		col string
		val any
	}
	var kvs []kv
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		col := columnOf(f)
		if col == "" {
			continue
		}
		fv := v.Field(i)
		if isZero(fv) {
			continue
		}
		kvs = append(kvs, kv{col: col, val: fv.Interface()})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].col < kvs[j].col })
	cols := make([]string, len(kvs))
	vals := make([]any, len(kvs))
	for i, kv := range kvs {
		cols[i] = kv.col
		vals[i] = kv.val
	}
	return cols, vals, nil
}

// injectAutoTimestamp 在 value 中设置自动时间戳列。
// value 必须是 map[string]any 或可寻址结构体指针。
func injectAutoTimestamp(value any, col string) bool {
	if m, ok := value.(map[string]any); ok {
		m[col] = time.Now()
		return true
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		elem := rv.Elem()
		if elem.Kind() == reflect.Struct {
			return setFieldByCol(elem, col, time.Now())
		}
	}
	return false
}

// setFieldByCol 根据 tdb 列名在结构体上设置 time.Time 字段值。
func setFieldByCol(v reflect.Value, col string, now time.Time) bool {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if columnOf(f) == col {
			fv := v.Field(i)
			if !fv.CanSet() {
				return false
			}
			if fv.Kind() == reflect.Pointer {
				// *time.Time 字段：分配并设置
				fv.Set(reflect.ValueOf(&now))
			} else {
				fv.Set(reflect.ValueOf(now))
			}
			return true
		}
	}
	return false
}

// isZero 判断 reflect.Value 是否为零值（用于部分更新跳过）。
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface:
		return v.IsNil()
	}
	return false
}
