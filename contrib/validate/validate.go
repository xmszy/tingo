// Package validate 提供校验器。
//
// 设计目标：便捷的数据校验体验。
//   - 规则写法：require|max:25|email|in:1,2,3|regex:^...
//   - 同时支持结构体 tag（binding 风格），字段级注解零侵入；
//   - 校验对象可是结构体、map[string]any（表单），或任意值；
//   - 出错即返回首个字段的首个错误（默认语义），并支持批量收集；
//   - 内置常用规则 + 可扩展的自定义规则函数；
//   - 零外部依赖，纯标准库实现，不进入请求热路径的性能门禁。
package validate

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Error 是单条校验错误信息。
type Error struct {
	// Field 出错的字段名（结构体字段名或表单 key）。
	Field string
	// Rule 触发的规则名。
	Rule string
	// Message 可读的错误信息。
	Message string
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("field %q failed rule %q", e.Field, e.Rule)
}

// Errors 是多条校验错误（批量模式）。
type Errors []*Error

// Error 实现 error 接口，聚合所有错误。
func (es Errors) Error() string {
	if len(es) == 0 {
		return "validation failed"
	}
	var b strings.Builder
	for i, e := range es {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e.Error())
	}
	return b.String()
}

// First 返回首条错误。
func (es Errors) First() *Error {
	if len(es) == 0 {
		return nil
	}
	return es[0]
}

// Messages 返回字段到消息的映射，便于 JSON 响应。
func (es Errors) Messages() map[string]string {
	m := make(map[string]string, len(es))
	for _, e := range es {
		if _, ok := m[e.Field]; !ok {
			m[e.Field] = e.Message
		}
	}
	return m
}

/* ------------------------------------------------------------------ */
/* 规则函数                                                            */
/* ------------------------------------------------------------------ */

// RuleFunc 是自定义规则函数。
// 入参 value 为字段当前值（已转为字符串便于比较），param 为规则冒号后的参数。
// 返回 true 表示通过。
type RuleFunc func(value string, param string) bool

// Validator 是校验器实例。
type Validator struct {
	rules  map[string]RuleFunc
	msgFmt map[string]string // 规则 -> 默认消息模板（含 %s 字段名）
}

var (
	defaultValidator = New()
)

// New 创建独立校验器（含全部内置规则）。
func New() *Validator {
	v := &Validator{
		rules:  map[string]RuleFunc{},
		msgFmt: map[string]string{},
	}
	v.registerBuiltin()
	return v
}

// 全局实例的便捷函数。

// AddRule 注册自定义规则到全局校验器。
func AddRule(name string, fn RuleFunc) { defaultValidator.AddRule(name, fn) }

// SetMsg 设置规则默认消息模板到全局校验器。
func SetMsg(rule, tmpl string) { defaultValidator.SetMsg(rule, tmpl) }

// Check 校验任意数据；rules 为 "field: rule1|rule2" 形式的映射。
// data 可为结构体或 map[string]any。返回 nil 表示通过。
func Check(data any, rules map[string]string, msgs ...map[string]string) error {
	return defaultValidator.Check(data, rules, msgs...)
}

// CheckBatch 同 Check，但收集全部错误而非首条即止。
func CheckBatch(data any, rules map[string]string, msgs ...map[string]string) error {
	return defaultValidator.CheckBatch(data, rules, msgs...)
}

/* ------------------------------------------------------------------ */
/* 注册内置规则                                                         */
/* ------------------------------------------------------------------ */

func (v *Validator) registerBuiltin() {
	// 类型/非空。
	v.AddRule("require", func(val, _ string) bool { return val != "" })
	v.SetMsg("require", "%s 不能为空")

	v.AddRule("alpha", func(val, _ string) bool { return regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(val) })
	v.SetMsg("alpha", "%s 只能是字母")

	v.AddRule("alphaNum", func(val, _ string) bool { return regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(val) })
	v.SetMsg("alphaNum", "%s 只能是字母和数字")

	v.AddRule("alphaDash", func(val, _ string) bool { return regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(val) })
	v.SetMsg("alphaDash", "%s 只能是字母、数字、下划线或短横线")

	v.AddRule("number", func(val, _ string) bool { _, e := strconv.ParseFloat(val, 64); return e == nil })
	v.SetMsg("number", "%s 必须是数字")

	v.AddRule("integer", func(val, _ string) bool { _, e := strconv.Atoi(val); return e == nil })
	v.SetMsg("integer", "%s 必须是整数")

	v.AddRule("email", func(val, _ string) bool {
		return regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`).MatchString(val)
	})
	v.SetMsg("email", "%s 格式不正确")

	v.AddRule("url", func(val, _ string) bool {
		return regexp.MustCompile(`^https?://[^\s]+$`).MatchString(val)
	})
	v.SetMsg("url", "%s 不是合法的 URL")

	v.AddRule("ip", func(val, _ string) bool {
		return regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`).MatchString(val)
	})
	v.SetMsg("ip", "%s 不是合法的 IP")

	// 长度/区间。
	v.AddRule("max", func(val, param string) bool {
		n, err := strconv.Atoi(param)
		if err != nil {
			return false
		}
		r := []rune(val)
		return len(r) <= n
	})
	v.SetMsg("max", "%s 长度不能超过 %s")

	v.AddRule("min", func(val, param string) bool {
		n, err := strconv.Atoi(param)
		if err != nil {
			return false
		}
		return len([]rune(val)) >= n
	})
	v.SetMsg("min", "%s 长度不能小于 %s")

	v.AddRule("length", func(val, param string) bool {
		// length:3 或 length:3,8
		parts := strings.Split(param, ",")
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return false
		}
		if len(parts) == 2 {
			m, err := strconv.Atoi(parts[1])
			if err != nil {
				return false
			}
			l := len([]rune(val))
			return l >= n && l <= m
		}
		return len([]rune(val)) == n
	})
	v.SetMsg("length", "%s 长度必须为 %s")

	// 比较。
	v.AddRule("in", func(val, param string) bool {
		for _, p := range strings.Split(param, ",") {
			if strings.TrimSpace(p) == val {
				return true
			}
		}
		return false
	})
	v.SetMsg("in", "%s 必须在 %s 范围内")

	v.AddRule("notIn", func(val, param string) bool {
		for _, p := range strings.Split(param, ",") {
			if strings.TrimSpace(p) == val {
				return false
			}
		}
		return true
	})
	v.SetMsg("notIn", "%s 不能在 %s 范围内")

	v.AddRule("gt", func(val, param string) bool {
		a, e1 := strconv.ParseFloat(val, 64)
		b, e2 := strconv.ParseFloat(param, 64)
		return e1 == nil && e2 == nil && a > b
	})
	v.SetMsg("gt", "%s 必须大于 %s")

	v.AddRule("lt", func(val, param string) bool {
		a, e1 := strconv.ParseFloat(val, 64)
		b, e2 := strconv.ParseFloat(param, 64)
		return e1 == nil && e2 == nil && a < b
	})
	v.SetMsg("lt", "%s 必须小于 %s")

	v.AddRule("between", func(val, param string) bool {
		parts := strings.Split(param, ",")
		if len(parts) != 2 {
			return false
		}
		a, e1 := strconv.ParseFloat(val, 64)
		lo, e2 := strconv.ParseFloat(parts[0], 64)
		hi, e3 := strconv.ParseFloat(parts[1], 64)
		return e1 == nil && e2 == nil && e3 == nil && a >= lo && a <= hi
	})
	v.SetMsg("between", "%s 必须在 %s 之间")

	// 正则。
	v.AddRule("regex", func(val, param string) bool {
		re, err := regexp.Compile(param)
		if err != nil {
			return false
		}
		return re.MatchString(val)
	})
	v.SetMsg("regex", "%s 格式不正确")

	// 确认（与另一字段相等，如 password 与 password_confirm）。
	v.AddRule("confirm", func(val, param string) bool {
		// 该规则需要跨字段比较，由校验主流程特殊处理。
		return true
	})
}

// AddRule 注册自定义规则。
func (v *Validator) AddRule(name string, fn RuleFunc) {
	v.rules[name] = fn
}

// SetMsg 设置默认消息模板。模板中 %s 会被字段名替换，第二个 %s 为规则参数。
func (v *Validator) SetMsg(rule, tmpl string) {
	v.msgFmt[rule] = tmpl
}

/* ------------------------------------------------------------------ */
/* 校验主流程                                                          */
/* ------------------------------------------------------------------ */

// Check 校验单条即止（首错即返回）。
func (v *Validator) Check(data any, rules map[string]string, msgs ...map[string]string) error {
	es := v.check(data, rules, msgs...)
	if len(es) > 0 {
		return es[0]
	}
	return nil
}

// CheckBatch 收集全部错误。
func (v *Validator) CheckBatch(data any, rules map[string]string, msgs ...map[string]string) error {
	es := v.check(data, rules, msgs...)
	if len(es) > 0 {
		return es
	}
	return nil
}

func (v *Validator) check(data any, rules map[string]string, msgs ...map[string]string) Errors {
	msgMap := map[string]string{}
	if len(msgs) > 0 {
		for k, val := range msgs[0] {
			msgMap[k] = val
		}
	}
	// 提取所有字段的当前值。
	values := extractValues(data)

	var errs Errors
	for field, ruleStr := range rules {
		val := values[field]
		strVal := toString(val)
		for _, rule := range splitRules(ruleStr) {
			name, param := parseRule(rule)
			fn, ok := v.rules[name]
			if !ok {
				continue
			}
			// confirm 需要跨字段比较。
			if name == "confirm" {
				other := param
				if other == "" {
					other = field + "_confirm"
				}
				if toString(values[other]) != strVal {
					errs = append(errs, v.makeError(field, name, param, msgMap))
				}
				continue
			}
			// require 之外的规则在空值时跳过。
			if name != "require" && strVal == "" {
				continue
			}
			if !fn(strVal, param) {
				errs = append(errs, v.makeError(field, name, param, msgMap))
				if len(msgs) == 0 {
					// 单条模式：首个失败即停（Check 已截断首条，这里仍收集全部由调用方决定）。
				}
			}
		}
	}
	return errs
}

func (v *Validator) makeError(field, rule, param string, msgMap map[string]string) *Error {
	// 优先使用用户在 msgs 中针对 "field.rule" 或 "field" 的自定义消息。
	if m, ok := msgMap[field+"."+rule]; ok {
		return &Error{Field: field, Rule: rule, Message: m}
	}
	if m, ok := msgMap[field]; ok {
		return &Error{Field: field, Rule: rule, Message: m}
	}
	tmpl, ok := v.msgFmt[rule]
	if !ok {
		tmpl = "%s 校验失败"
	}
	msg := fmt.Sprintf(tmpl, field, param)
	return &Error{Field: field, Rule: rule, Message: msg}
}

/* ------------------------------------------------------------------ */
/* 工具函数                                                            */
/* ------------------------------------------------------------------ */

// extractValues 从结构体或 map 中提取字段名 -> 值（string 化）。
// 同时解析结构体字段的 validate tag 作为附加规则。
func extractValues(data any) map[string]any {
	out := map[string]any{}
	rv := reflect.ValueOf(data)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return out
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Struct:
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.PkgPath != "" { // 未导出
				continue
			}
			name := f.Name
			if tag := f.Tag.Get("json"); tag != "" {
				if n := strings.Split(tag, ",")[0]; n != "" && n != "-" {
					name = n
				}
			}
			if tag := f.Tag.Get("form"); tag != "" {
				if n := strings.Split(tag, ",")[0]; n != "" && n != "-" {
					name = n
				}
			}
			out[name] = rv.Field(i).Interface()
		}
	case reflect.Map:
		for _, k := range rv.MapKeys() {
			out[toString(k.Interface())] = rv.MapIndex(k).Interface()
		}
	}
	return out
}

// tagRulesCache 按 reflect.Type 缓存 struct tag 解析结果，避免重复反射。
var tagRulesCache sync.Map // map[reflect.Type]map[string]string

// TagRules 从结构体类型中提取 "validate" tag 规则，返回 field -> ruleStr。
// 用于支持结构体字段注解式校验。结果按类型缓存，线程安全。
func TagRules(data any) map[string]string {
	rt := reflect.TypeOf(data)
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return map[string]string{}
	}

	if v, ok := tagRulesCache.Load(rt); ok {
		return v.(map[string]string)
	}

	out := map[string]string{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("validate")
		if tag == "" {
			continue
		}
		name := f.Name
		if j := f.Tag.Get("json"); j != "" {
			if n := strings.Split(j, ",")[0]; n != "" && n != "-" {
				name = n
			}
		}
		if form := f.Tag.Get("form"); form != "" {
			if n := strings.Split(form, ",")[0]; n != "" && n != "-" {
				name = n
			}
		}
		out[name] = tag
	}

	tagRulesCache.Store(rt, out)
	return out
}

// Struct 校验带 validate tag 的结构体。msgMap 可选，自定义消息。
func Struct(data any, msgs ...map[string]string) error {
	return defaultValidator.Check(data, TagRules(data), msgs...)
}

// StructBatch 同 Struct，但收集全部错误。
func StructBatch(data any, msgs ...map[string]string) error {
	return defaultValidator.CheckBatch(data, TagRules(data), msgs...)
}
