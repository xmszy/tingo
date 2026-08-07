// Package tconv 提供类型转换工具。
// 设计要点：
//   - 基于标准库 strconv + time，零外部依赖，泛型优先。
//   - 支持任意类型互转。
//   - 提供 Must 版本函数。
package tconv

import (
	"fmt"
	"strconv"
	"time"
)

// ──────────────── 整数转换 ────────────────

// Int 将任意类型转为 int。
func Int(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int8:
		return int(val)
	case int16:
		return int(val)
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint:
		return int(val)
	case uint8:
		return int(val)
	case uint16:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	case float32:
		return int(val)
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	case bool:
		if val {
			return 1
		}
		return 0
	case fmt.Stringer:
		return Int(val.String())
	default:
		return 0
	}
}

// Int64 将任意类型转为 int64。
func Int64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	case float32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Uint 将任意类型转为 uint。
func Uint(v any) uint { return uint(Int64(v)) }

// Uint64 将任意类型转为 uint64。
func Uint64(v any) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case int64:
		return uint64(val)
	case int:
		return uint64(val)
	case float64:
		return uint64(val)
	case string:
		n, _ := strconv.ParseUint(val, 10, 64)
		return n
	default:
		return uint64(Int64(v))
	}
}

// Int8 将任意类型转为 int8。
func Int8(v any) int8 { return int8(Int64(v)) }

// Int16 将任意类型转为 int16。
func Int16(v any) int16 { return int16(Int64(v)) }

// Int32 将任意类型转为 int32。
func Int32(v any) int32 { return int32(Int64(v)) }

// Uint8 将任意类型转为 uint8。
func Uint8(v any) uint8 { return uint8(Uint64(v)) }

// Uint16 将任意类型转为 uint16。
func Uint16(v any) uint16 { return uint16(Uint64(v)) }

// Uint32 将任意类型转为 uint32。
func Uint32(v any) uint32 { return uint32(Uint64(v)) }

// ──────────────── 浮点转换 ────────────────

// Float64 将任意类型转为 float64。
func Float64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		n, _ := strconv.ParseFloat(val, 64)
		return n
	default:
		return float64(Int64(v))
	}
}

// Float32 将任意类型转为 float32。
func Float32(v any) float32 { return float32(Float64(v)) }

// ──────────────── 字符串转换 ────────────────

// String 将任意类型转为字符串。
func String(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case error:
		return val.Error()
	case time.Time:
		return val.Format(time.RFC3339)
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Bool 将任意类型转为 bool。
func Bool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "1" || val == "true" || val == "yes" || val == "on"
	case int, int8, int16, int32, int64:
		return Int64(val) != 0
	case uint, uint8, uint16, uint32, uint64:
		return Uint64(val) != 0
	case float32, float64:
		return Float64(val) != 0
	case nil:
		return false
	default:
		return false
	}
}

// Bytes 将任意类型转为 []byte。
func Bytes(v any) []byte {
	switch val := v.(type) {
	case []byte:
		return val
	case string:
		return []byte(val)
	default:
		return []byte(String(v))
	}
}

// Time 将任意类型转为 time.Time。
func Time(v any) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	case string:
		layouts := []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02",
			time.RFC3339Nano,
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, val); err == nil {
				return t
			}
		}
	case int64:
		if val > 1000000000000 { // 毫秒
			return time.UnixMilli(val)
		}
		return time.Unix(val, 0)
	case int:
		return Time(int64(val))
	}
	return time.Time{}
}

// ──────────────── Slice 转换 ────────────────

// Strings 将任意类型转为 []string。
func Strings(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = String(item)
		}
		return result
	case []int:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = strconv.Itoa(item)
		}
		return result
	default:
		return nil
	}
}

// Ints 将任意类型转为 []int。
func Ints(v any) []int {
	switch val := v.(type) {
	case []int:
		return val
	case []any:
		result := make([]int, len(val))
		for i, item := range val {
			result[i] = Int(item)
		}
		return result
	case []string:
		result := make([]int, len(val))
		for i, item := range val {
			result[i] = Int(item)
		}
		return result
	default:
		return nil
	}
}

// ──────────────── Map 转换 ────────────────

// Map 将任意类型转为 map[string]any。
func Map(v any) map[string]any {
	switch val := v.(type) {
	case map[string]any:
		return val
	case map[string]string:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = v
		}
		return result
	case map[string]int:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = v
		}
		return result
	default:
		return nil
	}
}

// MapStrStr 将任意类型转为 map[string]string。
func MapStrStr(v any) map[string]string {
	switch val := v.(type) {
	case map[string]string:
		return val
	case map[string]any:
		result := make(map[string]string, len(val))
		for k, v := range val {
			result[k] = String(v)
		}
		return result
	default:
		return nil
	}
}
