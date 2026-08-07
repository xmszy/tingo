// Package tregex 提供正则表达式封装。
// 设计要点：
//   - 基于标准库 regexp，零外部依赖。
//   - 提供 Match/Replace/Split 等便捷函数。
//   - 支持全局函数和实例方法两种用法。
package tregex

import (
	"regexp"
)

// ──────────────── 全局快捷函数 ────────────────

// Match 判断字符串是否匹配正则模式。
func Match(pattern, s string) bool { return regexp.MustCompile(pattern).MatchString(s) }

// MatchAll 返回所有匹配的结果（完整匹配）。
func MatchAll(pattern, s string) []string { return regexp.MustCompile(pattern).FindAllString(s, -1) }

// MatchAllSub 返回所有匹配的分组结果。
func MatchAllSub(pattern, s string) [][]string { return regexp.MustCompile(pattern).FindAllStringSubmatch(s, -1) }

// Replace 使用正则替换。
func Replace(pattern, s, repl string) string { return regexp.MustCompile(pattern).ReplaceAllString(s, repl) }

// ReplaceFunc 使用函数替换匹配内容。
func ReplaceFunc(pattern, s string, repl func(string) string) string {
	return regexp.MustCompile(pattern).ReplaceAllStringFunc(s, repl)
}

// Split 使用正则分割。
func Split(pattern, s string) []string { return regexp.MustCompile(pattern).Split(s, -1) }

// IsMatch 判断字符串是否匹配（Match 别名）。
func IsMatch(pattern, s string) bool { return Match(pattern, s) }

// Quote 转义正则特殊字符。
func Quote(s string) string { return regexp.QuoteMeta(s) }

// ──────────────── 实例方法 ────────────────

// Regex 正则对象。
type Regex struct {
	re *regexp.Regexp
}

// New 创建正则对象。
func New(pattern string) (*Regex, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &Regex{re: re}, nil
}

// Must 创建正则对象，编译失败 panic。
func Must(pattern string) *Regex { return &Regex{re: regexp.MustCompile(pattern)} }

// Match 判断字符串是否匹配。
func (r *Regex) Match(s string) bool { return r.re.MatchString(s) }

// Find 返回第一个匹配。
func (r *Regex) Find(s string) string { return r.re.FindString(s) }

// FindAll 返回所有匹配。
func (r *Regex) FindAll(s string) []string { return r.re.FindAllString(s, -1) }

// FindSub 返回第一个匹配的分组。
func (r *Regex) FindSub(s string) []string { return r.re.FindStringSubmatch(s) }

// FindAllSub 返回所有匹配的分组。
func (r *Regex) FindAllSub(s string) [][]string { return r.re.FindAllStringSubmatch(s, -1) }

// Replace 替换匹配内容。
func (r *Regex) Replace(s, repl string) string { return r.re.ReplaceAllString(s, repl) }

// ReplaceFunc 使用函数替换。
func (r *Regex) ReplaceFunc(s string, repl func(string) string) string { return r.re.ReplaceAllStringFunc(s, repl) }

// Split 分割字符串。
func (r *Regex) Split(s string) []string { return r.re.Split(s, -1) }

// Regexp 返回底层 *regexp.Regexp。
func (r *Regex) Regexp() *regexp.Regexp { return r.re }
