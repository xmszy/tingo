// Package html 提供 HTML 实体编解码。
// 设计要点：
//   - 基于标准库 html，零外部依赖。
//   - 提供 Escape/Unescape 和 StripTags 等便捷函数。
package html

import (
	"html"
	"regexp"
	"strings"
)

var tagRe = regexp.MustCompile(`<[^>]*>`)

// Escape 将特殊字符转义为 HTML 实体。
func Escape(s string) string { return html.EscapeString(s) }

// Unescape 将 HTML 实体还原为特殊字符。
func Unescape(s string) string { return html.UnescapeString(s) }

// StripTags 移除字符串中的 HTML 标签。
func StripTags(s string) string { return tagRe.ReplaceAllString(s, "") }

// SpecialChars 将特殊字符转为 HTML 实体（& " ' < >）。
func SpecialChars(s string) string { return html.EscapeString(s) }

// SpecialCharsDecode 将 HTML 实体还原。
func SpecialCharsDecode(s string) string { return html.UnescapeString(s) }

// Entities 编码所有适用的字符为 HTML 实体（别名）。
func Entities(s string) string { return html.EscapeString(s) }

// EntitiesDecode 解码所有 HTML 实体（别名）。
func EntitiesDecode(s string) string { return html.UnescapeString(s) }

// NL2BR 将换行符替换为 <br>。
func NL2BR(s string) string { return strings.ReplaceAll(s, "\n", "<br>") }
