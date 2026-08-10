package tvalid

import (
	"fmt"
	"strconv"
	"time"
)

// ──────────────── 跨字段比较规则（联合规则，需 data） ────────────────

func init() {
	registerCrossFieldRules()
}

func registerCrossFieldRules() {
	// same:field — 当前字段值必须与 field 字段相等（如确认密码）。
	ruleFuncCtx["same"] = func(value any, args []string, data map[string]any) error {
		if len(args) == 0 {
			return nil
		}
		other, ok := data[args[0]]
		if !ok {
			return nil
		}
		if !equalValues(value, other) {
			return fmt.Errorf("must be same as %s", args[0])
		}
		return nil
	}
	// different:field — 当前字段值必须与 field 字段不同。
	ruleFuncCtx["different"] = func(value any, args []string, data map[string]any) error {
		if len(args) == 0 {
			return nil
		}
		other, ok := data[args[0]]
		if !ok {
			return nil
		}
		if equalValues(value, other) {
			return fmt.Errorf("must be different from %s", args[0])
		}
		return nil
	}
	// gt:field / gte:field / lt:field / lte:field — 与其他字段数值比较。
	ruleFuncCtx["gt"] = crossNumberCmp(func(a, b float64) bool { return a > b }, "greater than")
	ruleFuncCtx["gte"] = crossNumberCmp(func(a, b float64) bool { return a >= b }, "greater than or equal")
	ruleFuncCtx["lt"] = crossNumberCmp(func(a, b float64) bool { return a < b }, "less than")
	ruleFuncCtx["lte"] = crossNumberCmp(func(a, b float64) bool { return a <= b }, "less than or equal")
}

func crossNumberCmp(cmp func(a, b float64) bool, verb string) RuleFuncCtx {
	return func(value any, args []string, data map[string]any) error {
		if len(args) == 0 {
			return nil
		}
		other, ok := data[args[0]]
		if !ok {
			return nil
		}
		a := toFloat(value)
		b := toFloat(other)
		if !cmp(a, b) {
			return fmt.Errorf("must be %s %s", verb, args[0])
		}
		return nil
	}
}

func equalValues(a, b any) bool {
	af, ok1 := isNumber(a)
	bf, ok2 := isNumber(b)
	if ok1 && ok2 {
		return af == bf
	}
	return equalValue(a, b)
}

// isNumber 尝试将值解析为 float64（用于跨字段数值比较）。
func isNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// ──────────────── 普通规则 ────────────────

func init() {
	// date-range:start,end — 值（日期/时间）必须落在 [start, end] 区间内。
	defaultValidator.Register("date-range", func(value any, args []string) error {
		if len(args) < 2 {
			return nil
		}
		t, ok := toTime(value)
		if !ok {
			return fmt.Errorf("date format invalid")
		}
		start, ok1 := toTime(parseTimeArg(args[0]))
		end, ok2 := toTime(parseTimeArg(args[1]))
		if !ok1 || !ok2 {
			return fmt.Errorf("date-range args invalid")
		}
		if t.Before(start) || t.After(end) {
			return fmt.Errorf("must be between %s and %s", args[0], args[1])
		}
		return nil
	}, "{field} 必须落在 {args} 区间内")
}

// parseTimeArg 支持 "2006-01-02" 与 "2006-01-02 15:04:05" 两种简写。
func parseTimeArg(s string) string {
	if len(s) == 10 {
		return s + " 00:00:00"
	}
	return s
}

func toTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		for _, l := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
			if t, err := time.Parse(l, x); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}
