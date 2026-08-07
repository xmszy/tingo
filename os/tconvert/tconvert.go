package tconvert

import (
	"reflect"
	"strconv"
	"time"
)

// ToString 将 any 转为字符串。
func ToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.String {
			return rv.String()
		}
		return ""
	}
}

// ToInt 将 any 转为 int。
func ToInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

// ToFloat 将 any 转为 float64。
func ToFloat(v any) float64 {
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
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// ToBool 将 any 转为 bool。
func ToBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		b, _ := strconv.ParseBool(val)
		return b
	case int:
		return val != 0
	default:
		return v != nil
	}
}

// ToInt64 将 any 转为 int64。
func ToInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}

// ToTime 将 any 转为 time.Time。
func ToTime(v any) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	case string:
		t, _ := time.Parse(time.RFC3339, val)
		return t
	default:
		return time.Time{}
	}
}
