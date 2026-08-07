package tview

import (
	"html/template"
	"reflect"
	"strconv"
	"strings"
)

// withContent 在 merged 数据中注入 content 字段，供布局模板使用。
func withContent(data any, content string) any {
	if m, ok := data.(map[string]any); ok {
		m["content"] = template.HTML(content)
		return m
	}
	return map[string]any{"content": template.HTML(content), "data": data}
}

// toStr 将任意值转为字符串。
func toStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		return strings.TrimSpace(sprintAny(x))
	}
}

// sprintAny 兜底字符串化。
func sprintAny(v any) string {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	b, ok := rv.Interface().(interface{ String() string })
	if ok {
		return b.String()
	}
	return ""
}

// titleCase 将字符串中每个单词的首字母大写（零依赖替代 strings.Title）。
func titleCase(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	upper := true
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '-' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
		upper = false
	}
	return string(b)
}

// isEmpty 判断值是否为空（用于 default 函数）。
func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Pointer, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}
