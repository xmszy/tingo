package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/xmszy/tingo/os/tcodegen"
)

//go:embed stubs/*.tpl
var embeddedStubs embed.FS

// stubFuncs 提供模板内可用的自定义函数。
var stubFuncs = template.FuncMap{
	"snake":      tcodegen.Snake,
	"lowerCamel": lowerCamelIdentifier,
	"plural":     pluralize,
}

// loadStub 按 kind + variant 加载模板内容。
//
// 查找优先级：
//  1. 项目根 stubs/ 目录（用户自定义）
//  2. 嵌入式 stubs/ 目录（内置默认）
//
// variant 为空时加载 kind.tpl；否则加载 kind.variant.tpl。
// 未找到时返回空字符串。
func loadStub(kind, variant string) string {
	filename := kind + ".tpl"
	if variant != "" {
		filename = kind + "." + variant + ".tpl"
	}

	// 1. 项目 stubs/ 目录（用户自定义模板）
	if data, err := os.ReadFile(filepath.Join("stubs", filename)); err == nil {
		return string(data)
	}

	// 2. 嵌入式内置模板
	if data, err := embeddedStubs.ReadFile("stubs/" + filename); err == nil {
		return string(data)
	}

	return ""
}

// ── 模板辅助函数 ──

// lowerCamelIdentifier 将 snake_case 转为 lowerCamelCase。
// 如 "user_order" → "userOrder"。
func lowerCamelIdentifier(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		if i == 0 {
			runes[0] = unicode.ToLower(runes[0])
		} else {
			runes[0] = unicode.ToUpper(runes[0])
		}
		b.WriteString(string(runes))
	}
	return b.String()
}

// snakeIdentifier 将 PascalCase 转为 snake_case。
// 如 "UserOrder" → "user_order"。
func snakeIdentifier(s string) string {
	return tcodegen.Snake(s)
}

// kebabIdentifier 将 PascalCase 转为 kebab-case。
// 如 "ExportReport" → "export-report"。
func kebabIdentifier(s string) string {
	return strings.ReplaceAll(tcodegen.Snake(s), "_", "-")
}

// pluralize 将英文单词变为复数（简单规则：末尾 +s）。
func pluralize(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") || strings.HasSuffix(s, "z") ||
		strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	if strings.HasSuffix(s, "y") {
		if l := len(s); l > 1 {
			last := rune(s[l-2])
			if last != 'a' && last != 'e' && last != 'i' && last != 'o' && last != 'u' {
				return s[:l-1] + "ies"
			}
		}
	}
	return s + "s"
}
