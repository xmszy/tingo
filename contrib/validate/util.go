package validate

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// splitRules 按 | 切分规则，但允许正则规则内的 |（regex:...）。
func splitRules(ruleStr string) []string {
	var out []string
	var cur strings.Builder
	inRegex := false
	for i := 0; i < len(ruleStr); i++ {
		ch := ruleStr[i]
		switch {
		case ch == '|' && !inRegex:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			if ch == ':' && strings.HasPrefix(cur.String(), "regex") {
				inRegex = true
			}
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

// parseRule 拆分规则名与参数。max:25 -> ("max", "25")。
func parseRule(rule string) (name, param string) {
	if i := strings.Index(rule, ":"); i >= 0 {
		return rule[:i], rule[i+1:]
	}
	return rule, ""
}

// toString 将任意值转为字符串用于规则比较。
func toString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case fmt.Stringer:
		return x.String()
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.FormatInt(rv.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return strconv.FormatUint(rv.Uint(), 10)
		case reflect.Float32, reflect.Float64:
			return strconv.FormatFloat(rv.Float(), 'f', -1, 64)
		case reflect.Bool:
			return strconv.FormatBool(rv.Bool())
		}
		return fmt.Sprintf("%v", v)
	}
}
