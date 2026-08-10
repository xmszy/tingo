package tconv

import (
	"encoding/json"
	"reflect"
)

// ──────────────── 切片类型转换 ────────────────
//
// 将任意值（切片 / 数组 / JSON 字符串 / 单值）转换为各类切片，供配置、参数解析复用。

// sliceReflect 将切片/数组/单值反射展开为 []any。
func sliceReflect(v any) []any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out
	default:
		return []any{v}
	}
}

// Interfaces 转为 []any。单值被包装为单元素切片。
func Interfaces(v any) []any { return sliceReflect(v) }

// SliceAny 等价于 Interfaces。
func SliceAny(v any) []any { return sliceReflect(v) }

// SliceInt 转为 []int。
func SliceInt(v any) []int {
	if s, ok := v.([]int); ok {
		return s
	}
	raw := sliceReflect(v)
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		out = append(out, Int(item))
	}
	return out
}

// SliceInt8 转为 []int8。
func SliceInt8(v any) []int8 {
	raw := sliceReflect(v)
	out := make([]int8, 0, len(raw))
	for _, item := range raw {
		out = append(out, Int8(item))
	}
	return out
}

// SliceInt32 转为 []int32。
func SliceInt32(v any) []int32 {
	raw := sliceReflect(v)
	out := make([]int32, 0, len(raw))
	for _, item := range raw {
		out = append(out, Int32(item))
	}
	return out
}

// SliceInt64 转为 []int64。
func SliceInt64(v any) []int64 {
	raw := sliceReflect(v)
	out := make([]int64, 0, len(raw))
	for _, item := range raw {
		out = append(out, Int64(item))
	}
	return out
}

// SliceUint 转为 []uint。
func SliceUint(v any) []uint {
	raw := sliceReflect(v)
	out := make([]uint, 0, len(raw))
	for _, item := range raw {
		out = append(out, Uint(item))
	}
	return out
}

// SliceUint64 转为 []uint64。
func SliceUint64(v any) []uint64 {
	raw := sliceReflect(v)
	out := make([]uint64, 0, len(raw))
	for _, item := range raw {
		out = append(out, Uint64(item))
	}
	return out
}

// SliceFloat32 转为 []float32。
func SliceFloat32(v any) []float32 {
	raw := sliceReflect(v)
	out := make([]float32, 0, len(raw))
	for _, item := range raw {
		out = append(out, Float32(item))
	}
	return out
}

// SliceFloat64 转为 []float64。
func SliceFloat64(v any) []float64 {
	raw := sliceReflect(v)
	out := make([]float64, 0, len(raw))
	for _, item := range raw {
		out = append(out, Float64(item))
	}
	return out
}

// SliceBool 转为 []bool。
func SliceBool(v any) []bool {
	if s, ok := v.([]bool); ok {
		return s
	}
	raw := sliceReflect(v)
	out := make([]bool, 0, len(raw))
	for _, item := range raw {
		out = append(out, Bool(item))
	}
	return out
}

// SliceStr 转为 []string。
func SliceStr(v any) []string { return Strings(v) }

// SliceMap 转为 []map[string]any。入参可为 []map / [][]any / JSON 字符串 / []struct。
func SliceMap(v any) []map[string]any {
	if v == nil {
		return nil
	}
	// JSON 字符串
	if s, ok := v.(string); ok && len(s) > 0 && (s[0] == '[') {
		var out []map[string]any
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out
		}
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil
	}
	out := make([]map[string]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		if m := Map(rv.Index(i).Interface()); m != nil {
			out = append(out, m)
		}
	}
	return out
}
