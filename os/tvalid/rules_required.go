package tvalid

import (
	"fmt"
	"reflect"
	"strings"
)

// ──────────────── 条件必填规则（联合规则） ────────────────
//
// 普通规则 RuleFunc 只能访问单个字段值；条件必填需要访问整份数据，
// 因此引入 RuleFuncCtx，签名额外携带 data 用于联合判断。

// RuleFuncCtx 联合规则函数：可访问整份数据 data 做关联判断。
// value 为当前字段值，args 为规则参数，data 为整份校验数据（字段名 → 值）。
type RuleFuncCtx func(value any, args []string, data map[string]any) error

// ruleFuncCtx 联合规则注册表。
var ruleFuncCtx = map[string]RuleFuncCtx{}

func init() { registerRequiredCtx() }

func registerRequiredCtx() {
	ruleFuncCtx["required-if"] = func(value any, args []string, data map[string]any) error {
		// required-if:field1:value1,value2...[:field2:value...]
		pairs := parseFieldValuePairs(args)
		if !allMatch(pairs, data) {
			return nil // 条件不满足，不校验必填
		}
		if isEmpty(value) {
			return fmt.Errorf("required")
		}
		return nil
	}
	ruleFuncCtx["required-with"] = func(value any, args []string, data map[string]any) error {
		if anyKeyPresent(args, data) && isEmpty(value) {
			return fmt.Errorf("required")
		}
		return nil
	}
	ruleFuncCtx["required-with-all"] = func(value any, args []string, data map[string]any) error {
		if len(args) > 0 && allKeysPresent(args, data) && isEmpty(value) {
			return fmt.Errorf("required")
		}
		return nil
	}
	ruleFuncCtx["required-without"] = func(value any, args []string, data map[string]any) error {
		if len(args) > 0 && anyKeyAbsent(args, data) && isEmpty(value) {
			return fmt.Errorf("required")
		}
		return nil
	}
	ruleFuncCtx["required-without-all"] = func(value any, args []string, data map[string]any) error {
		if len(args) > 0 && allKeysAbsent(args, data) && isEmpty(value) {
			return fmt.Errorf("required")
		}
		return nil
	}
	ruleFuncCtx["required-unless"] = func(value any, args []string, data map[string]any) error {
		pairs := parseFieldValuePairs(args)
		if !allMatch(pairs, data) && isEmpty(value) {
			return fmt.Errorf("required")
		}
		return nil
	}
}

// parseFieldValuePairs 解析 "field1:v1,v2:field2:v3" 形式参数。
func parseFieldValuePairs(args []string) [][2]string {
	out := make([][2]string, 0)
	for i := 0; i+1 < len(args); i += 2 {
		out = append(out, [2]string{args[i], args[i+1]})
	}
	return out
}

func allMatch(pairs [][2]string, data map[string]any) bool {
	for _, p := range pairs {
		want := strings.Split(p[1], ",")
		got := fmt.Sprintf("%v", data[p[0]])
		matched := false
		for _, w := range want {
			if w == got {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return len(pairs) > 0
}

func anyKeyPresent(keys []string, data map[string]any) bool {
	for _, k := range keys {
		if _, ok := data[k]; ok && !isEmpty(data[k]) {
			return true
		}
	}
	return false
}

func allKeysPresent(keys []string, data map[string]any) bool {
	for _, k := range keys {
		if _, ok := data[k]; !ok || isEmpty(data[k]) {
			return false
		}
	}
	return true
}

func anyKeyAbsent(keys []string, data map[string]any) bool {
	for _, k := range keys {
		if _, ok := data[k]; !ok || isEmpty(data[k]) {
			return true
		}
	}
	return len(keys) > 0
}

func allKeysAbsent(keys []string, data map[string]any) bool {
	for _, k := range keys {
		if _, ok := data[k]; ok && !isEmpty(data[k]) {
			return false
		}
	}
	return len(keys) > 0
}

// isEmpty 判断值是否为空（与 ruleRequired 语义一致：nil、空串、空切片/map、nil 指针为空）。
// 数值类型（含 0）视为非空。
func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	switch val := value.(type) {
	case string:
		return val == ""
	case []byte:
		return len(val) == 0
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return false
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len() == 0
		case reflect.Ptr, reflect.Interface:
			return rv.IsNil()
		}
	}
	return false
}
