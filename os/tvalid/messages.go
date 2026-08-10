package tvalid

import (
	"strings"
)

// ──────────────── 多语言错误提示 ────────────────
//
// 内置规则默认错误提示支持中/英两种语言，可通过 SetLang 切换。
// 自定义规则注册时传入的 defaultMsg 会作为兜底（在其语言模板缺失时使用）。

var (
	// lang 当前语言，默认 "zh"。
	lang = "zh"
	// defaultMessages 规则名 → {zh, en} 模板。
	defaultMessages = map[string]map[string]string{
		"required":     {"zh": "{field} 不能为空", "en": "{field} is required"},
		"ip":           {"zh": "{field} 格式不正确", "en": "{field} is not a valid IP"},
		"email":        {"zh": "{field} 邮箱格式不正确", "en": "{field} is not a valid email"},
		"url":          {"zh": "{field} URL 格式不正确", "en": "{field} is not a valid URL"},
		"len":          {"zh": "{field} 长度必须为 {args}", "en": "{field} length must be {args}"},
		"len-min":      {"zh": "{field} 长度不能小于 {args}", "en": "{field} length must be >= {args}"},
		"len-max":      {"zh": "{field} 长度不能大于 {args}", "en": "{field} length must be <= {args}"},
		"regex":        {"zh": "{field} 格式不匹配", "en": "{field} format mismatch"},
		"in":           {"zh": "{field} 值不在允许范围内", "en": "{field} is not in the allowed range"},
		"not-in":       {"zh": "{field} 值不允许", "en": "{field} is not allowed"},
		"numeric":      {"zh": "{field} 必须为数字", "en": "{field} must be numeric"},
		"alpha-num":    {"zh": "{field} 只能为字母和数字", "en": "{field} must be alphanumeric"},
		"between":      {"zh": "{field} 必须在 {args} 之间", "en": "{field} must be between {args}"},
		"min":          {"zh": "{field} 不能小于 {args}", "en": "{field} must be >= {args}"},
		"max":          {"zh": "{field} 不能大于 {args}", "en": "{field} must be <= {args}"},
		"eq":           {"zh": "{field} 值不匹配", "en": "{field} mismatch"},
		"phone":        {"zh": "{field} 手机号格式不正确", "en": "{field} is not a valid phone"},
		"date":         {"zh": "{field} 日期格式不正确", "en": "{field} is not a valid date"},
		"boolean":      {"zh": "{field} 必须为布尔值", "en": "{field} must be boolean"},
		"json":         {"zh": "{field} 不是合法 JSON", "en": "{field} is not valid JSON"},
		"uuid":         {"zh": "{field} UUID 格式不正确", "en": "{field} is not a valid UUID"},
		"enum":         {"zh": "{field} 值不在允许的枚举范围内", "en": "{field} is not in the enum"},
		"alpha":        {"zh": "{field} 只能为字母", "en": "{field} must be letters only"},
		"alpha-dash":   {"zh": "{field} 只能为字母、数字、短划线和下划线", "en": "{field} must be alpha-dash"},
		"chinese":      {"zh": "{field} 只能为中文", "en": "{field} must be Chinese characters"},
		"file-size":    {"zh": "{field} 文件大小超出限制", "en": "{field} file size exceeded"},
		"file-ext":     {"zh": "{field} 文件扩展名不允许", "en": "{field} file extension not allowed"},
		"required-if":  {"zh": "{field} 在 {args} 条件下必填", "en": "{field} is required when {args}"},
		"required-with": {"zh": "{field} 在存在 {args} 时必填", "en": "{field} is required with {args}"},
		"required-with-all": {"zh": "{field} 在同时存在 {args} 时必填", "en": "{field} is required with all {args}"},
		"required-without":  {"zh": "{field} 在缺少 {args} 时必填", "en": "{field} is required without {args}"},
		"required-without-all": {"zh": "{field} 在同时缺少 {args} 时必填", "en": "{field} is required without all {args}"},
	}
)

// SetLang 设置校验错误提示语言（"zh" / "en"）。
func SetLang(l string) {
	if l == "zh" || l == "en" {
		lang = l
	}
}

// GetLang 返回当前语言。
func GetLang() string { return lang }

// messageFor 返回规则在当前语言下的默认模板；缺失时回退英文，再回退原始提示。
func messageFor(ruleName, fallback string) string {
	if m, ok := defaultMessages[ruleName]; ok {
		if s, ok := m[lang]; ok && s != "" {
			return s
		}
		if s, ok := m["en"]; ok {
			return s
		}
	}
	if fallback != "" {
		return fallback
	}
	return ruleName
}

// replaceByMap 用 map 中的键值替换 s 中的 {key} 占位符（替代 gf gstr.ReplaceByMap）。
func replaceByMap(s string, m map[string]string) string {
	if len(m) == 0 {
		return s
	}
	repls := make([]string, 0, len(m)*2)
	for k, v := range m {
		repls = append(repls, "{"+k+"}", v)
	}
	return strings.NewReplacer(repls...).Replace(s)
}
