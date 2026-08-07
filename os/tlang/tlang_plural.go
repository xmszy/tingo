// Package tlang 提供零外部依赖的国际化（i18n）支持。
//
// 复数规则（Plural Rules）
//
// 参考 CLDR 标准（http://cldr.unicode.org/index/cldr-spec/plural-rules），
// 定义六种复数形式：Zero、One、Two、Few、Many、Other。
// 不同语言使用其中不同的子集：
//
//   - 中文（zh）：仅 Other（所有数量使用同一形式）
//   - 英文（en）：One（count==1）+ Other（其余）
//   - 法文（fr）：One（0或1）+ Other（其余）
//   - 俄文（ru）：One+ Few+ Many+ Other（四形式，依赖复杂余数规则）
//   - 阿拉伯文（ar）：全部六种形式
//
// 译文消息使用 | 分隔多个复数形式，按 CLDR 顺序排列：
//
//	"没有消息 | 1条消息 | {count} 条消息"         // en: zero|one|other
//	"{count} сообщение | {count} сообщения | {count} сообщений"  // ru: one|few|many
//
// 缺失的形式自动回退到 Other，若 Other 也为空则取最后一个非空形式。
package tlang

import "strings"

// PluralForm 表示 CLDR 复数形式。
type PluralForm string

const (
	PluralInvalid PluralForm = ""
	PluralZero    PluralForm = "zero"
	PluralOne     PluralForm = "one"
	PluralTwo     PluralForm = "two"
	PluralFew     PluralForm = "few"
	PluralMany    PluralForm = "many"
	PluralOther   PluralForm = "other"
)

// pluralFormOrder 按 CLDR 标准顺序排列，供 | 分隔消息索引使用。
var pluralFormOrder = []PluralForm{PluralZero, PluralOne, PluralTwo, PluralFew, PluralMany, PluralOther}

// PluralRule 根据 count 返回对应的复数形式。
// 如未设置（nil），则始终使用 PluralOther。
type PluralRule func(count int) PluralForm

// Built-in plural rules for common languages.
// These are simple approximations that cover the most common cases.
// For full CLDR compliance (especially Russian, Arabic, etc.),
// implement your own PluralRule.

// ChinesePluralRule 中文规则：始终返回 Other。
func ChinesePluralRule(count int) PluralForm { return PluralOther }

// EnglishPluralRule 英文规则：count==1 → One，否则 → Other。
func EnglishPluralRule(count int) PluralForm {
	if count == 1 {
		return PluralOne
	}
	return PluralOther
}

// FrenchPluralRule 法文规则：count==0 或 count==1 → One，否则 → Other。
func FrenchPluralRule(count int) PluralForm {
	if count == 0 || count == 1 {
		return PluralOne
	}
	return PluralOther
}

// selectPluralForm 从 | 分隔的复数消息中选择对应形式。
// 格式：zero|one|two|few|many|other
// 缺失形式回退到 Other；Other 也为空则取最后一个非空形式。
// 若消息不含 |，直接返回原消息（未启用复数格式）。
func selectPluralForm(msg string, form PluralForm) string {
	if !strings.Contains(msg, "|") {
		return msg
	}
	parts := strings.SplitN(msg, "|", len(pluralFormOrder)+1)
	// 修剪每部分的空白
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	// 查找匹配形式
	for i, pf := range pluralFormOrder {
		if pf == form && i < len(parts) && parts[i] != "" {
			return parts[i]
		}
	}
	// 回退到 Other
	if len(parts) > int(pluralOtherIndex()) && parts[pluralOtherIndex()] != "" {
		return parts[pluralOtherIndex()]
	}
	// 最后手段：取最后一个非空部分
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return msg
}

func pluralOtherIndex() int {
	for i, pf := range pluralFormOrder {
		if pf == PluralOther {
			return i
		}
	}
	return len(pluralFormOrder) - 1
}
