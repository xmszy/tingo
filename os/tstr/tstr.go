// Package tstr 提供字符串处理工具。
// 设计要点：
//   - 基于标准库 strings/strconv/unicode，零外部依赖。
//   - 补充常见命名辅助函数。
//   - 与 tlang.Snake/Camel 整合。
package tstr

import (
	"math/rand"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ──────────────── 判断 ────────────────

// IsEmpty 判断字符串是否为空（去除空白后）。
func IsEmpty(s string) bool { return strings.TrimSpace(s) == "" }

// IsNumeric 判断字符串是否全为数字。
func IsNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// IsLetter 判断字符串是否全为字母。
func IsLetter(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// HasPrefix 判断是否以 prefix 开头（标准库别名）。
func HasPrefix(s, prefix string) bool { return strings.HasPrefix(s, prefix) }

// HasSuffix 判断是否以 suffix 结尾（标准库别名）。
func HasSuffix(s, suffix string) bool { return strings.HasSuffix(s, suffix) }

// Contains 判断是否包含子串。
func Contains(s, substr string) bool { return strings.Contains(s, substr) }

// ContainsAny 判断是否包含任意一个字符。
func ContainsAny(s, chars string) bool { return strings.ContainsAny(s, chars) }

// EqualFold 忽略大小写比较。
func EqualFold(s, t string) bool { return strings.EqualFold(s, t) }

// ──────────────── 截取 ────────────────

// SubStr 截取子串（支持中文）。start 可为负（从末尾计数）。
func SubStr(s string, start int, length ...int) string {
	runes := []rune(s)
	l := len(runes)
	if start < 0 {
		start = l + start
		if start < 0 {
			start = 0
		}
	}
	if start >= l {
		return ""
	}
	if len(length) > 0 && length[0] >= 0 {
		end := start + length[0]
		if end > l {
			end = l
		}
		return string(runes[start:end])
	}
	return string(runes[start:])
}

// Limit 截取指定长度（按 rune 计数，UTF-8 安全），超出追加省略号。
func Limit(s string, limit int, ellipsis ...string) string {
	ell := "..."
	if len(ellipsis) > 0 {
		ell = ellipsis[0]
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + ell
}

// StrLimitRune 按 rune 数量截断字符串，超出追加省略号（UTF-8 安全）。
// 与 Limit 等价，显式命名强调 UTF-8 安全语义。
func StrLimitRune(s string, limit int, ellipsis ...string) string {
	return Limit(s, limit, ellipsis...)
}

// ──────────────── 大小写 ────────────────

// ToLower 转小写。
func ToLower(s string) string { return strings.ToLower(s) }

// ToUpper 转大写。
func ToUpper(s string) string { return strings.ToUpper(s) }

// UcFirst 首字母大写。
func UcFirst(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// LcFirst 首字母小写。
func LcFirst(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

// Title 单词首字母大写（以 unicode 分词）。
func Title(s string) string { return strings.ToTitle(s) }

// ──────────────── 命名风格 ────────────────

// Snake 驼峰转蛇形：HelloWorld → hello_world。
func Snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Camel 蛇形转驼峰：hello_world → HelloWorld。
func Camel(s string) string {
	var b strings.Builder
	upper := true
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(rune(s[i])))
			upper = false
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// LowerCamel 蛇形转小驼峰：hello_world → helloWorld。
func LowerCamel(s string) string {
	c := Camel(s)
	if c == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(c)
	return string(unicode.ToLower(r)) + c[size:]
}

// Kebab 转短横线分隔：HelloWorld → hello-world。
func Kebab(s string) string { return strings.ReplaceAll(Snake(s), "_", "-") }

// CaseSnakeScreaming 驼峰转全大写下划线：HelloWorld → HELLO_WORLD。
func CaseSnakeScreaming(s string) string { return strings.ToUpper(Snake(s)) }

// CaseKebabScreaming 驼峰转全大写短横线：HelloWorld → HELLO-WORLD。
func CaseKebabScreaming(s string) string { return strings.ToUpper(Kebab(s)) }

// ──────────────── 修剪 ────────────────

// Trim 去除两端空白。
func Trim(s string) string { return strings.TrimSpace(s) }

// TrimStr 去除两端指定字符集。
func TrimStr(s, cutset string) string { return strings.Trim(s, cutset) }

// TrimLeft 去除左侧空白。
func TrimLeft(s string) string { return strings.TrimLeft(s, " \t\n\r") }

// TrimRight 去除右侧空白。
func TrimRight(s string) string { return strings.TrimRight(s, " \t\n\r") }

// ──────────────── 查找 ────────────────

// Pos 查找子串位置，未找到返回 -1。
func Pos(haystack, needle string) int { return strings.Index(haystack, needle) }

// PosR 反向查找子串位置。
func PosR(haystack, needle string) int { return strings.LastIndex(haystack, needle) }

// Count 统计子串出现次数。
func Count(s, substr string) int { return strings.Count(s, substr) }

// ──────────────── 替换 ────────────────

// Replace 替换子串，n 为次数（-1 为全部）。
func Replace(s, old, new string, n int) string { return strings.Replace(s, old, new, n) }

// ReplaceAll 替换所有匹配的子串。
func ReplaceAll(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

// ──────────────── 分割/拼接 ────────────────

// Split 分割字符串。
func Split(s, sep string) []string { return strings.Split(s, sep) }

// SplitN 分割字符串，限制段数。
func SplitN(s, sep string, n int) []string { return strings.SplitN(s, sep, n) }

// Fields 按空白分割。
func Fields(s string) []string { return strings.Fields(s) }

// Join 拼接字符串数组。
func Join(elems []string, sep string) string { return strings.Join(elems, sep) }

// ──────────────── 其他 ────────────────

// Len 返回字符串字符数（rune 计数，支持中文）。
func Len(s string) int { return utf8.RuneCountInString(s) }

// LenBytes 返回字符串字节数。
func LenBytes(s string) int { return len(s) }

// Repeat 重复字符串。
func Repeat(s string, count int) string { return strings.Repeat(s, count) }

// Reverse 反转字符串（支持 Unicode）。
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Shuffle 随机打乱字符串。
func Shuffle(s string) string {
	runes := []rune(s)
	rand.Shuffle(len(runes), func(i, j int) { runes[i], runes[j] = runes[j], runes[i] })
	return string(runes)
}

// Concat 拼接多个字符串。
func Concat(parts ...string) string { return strings.Join(parts, "") }

// Ord 返回字符串首字符的 Unicode 码点。
func Ord(s string) int {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	return int(r)
}

// Chr 将 Unicode 码点转为字符。
func Chr(code int) string { return string(rune(code)) }

// ──────────────── 类型转换 ────────────────

// ToInt 字符串转整数。
func ToInt(s string) int { return ToIntDefault(s, 0) }

// ToIntDefault 字符串转整数，带默认值。
func ToIntDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// ToInt64 字符串转 int64。
func ToInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// ToFloat64 字符串转 float64。
func ToFloat64(s string) float64 {
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

// ──────────────── 脱敏 ────────────────

// HideStr 隐藏字符串中间部分，用指定字符替换。
// 例: HideStr("13812345678", 3, 4, "*") → "138****5678"
func HideStr(s string, start, end int, char string) string {
	runes := []rune(s)
	l := len(runes)
	if l == 0 {
		return ""
	}
	if start < 0 {
		start = 0
	}
	if end < 0 || end > l {
		end = l
	}
	if start >= end {
		return s
	}
	hideChar := rune('*')
	if len(char) > 0 {
		hideChar, _ = utf8.DecodeRuneInString(char)
	}
	var b strings.Builder
	b.Grow(l*utf8.UTFMax + (end-start))
	b.WriteString(string(runes[:start]))
	for i := start; i < end; i++ {
		b.WriteRune(hideChar)
	}
	b.WriteString(string(runes[end:]))
	return b.String()
}

// HideEmail 隐藏邮箱地址的用户名中间部分。
// 例: HideEmail("zhangsan@example.com") → "zha***san@example.com"
func HideEmail(email string) string {
	idx := strings.IndexByte(email, '@')
	if idx < 0 {
		return HideStr(email, 1, len([]rune(email))-1, "*")
	}
	user := email[:idx]
	domain := email[idx:]
	runes := []rune(user)
	if len(runes) <= 2 {
		return user[:1] + "***" + domain
	}
	mid := len(runes) / 2
	return string(runes[:mid-1]) + "***" + string(runes[mid+1]) + domain
}

// ──────────────── 转义 ────────────────

// AddSlashes 转义单引号、双引号和反斜杠。
func AddSlashes(s string) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/10)
	for _, r := range s {
		switch r {
		case '\'', '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// StripSlashes 去除转义符：\n → 换行，\\ → \，\' → '。
func StripSlashes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case '\\', '\'', '"':
				b.WriteByte(s[i+1])
				i++
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// ──────────────── 相似度 ────────────────

// SimilarText 计算两个字符串的相似度（Levenshtein 距离比例）。
// 返回值 [0,1]，1 表示完全相同。
func SimilarText(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 && lb == 0 {
		return 1.0
	}
	if la == 0 || lb == 0 {
		return 0.0
	}
	// 滚动数组优化：O(min(la,lb)) 空间
	if la < lb {
		ra, rb, la, lb = rb, ra, lb, la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return 1.0 - float64(prev[lb])/float64(la)
}

func min3(a, b, c int) int {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}

// ──────────────── 版本比较 ────────────────

// CompareVersion 比较版本号。返回 -1: a<b, 0: a==b, 1: a>b。
// 支持 "1.2.3"、"1.2.3-beta" 等格式。
func CompareVersion(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var an, bn int
		if i < len(as) {
			an, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bn, _ = strconv.Atoi(bs[i])
		}
		if an < bn {
			return -1
		}
		if an > bn {
			return 1
		}
	}
	return 0
}

// ──────────────── 随机 ────────────────

// Random 生成指定长度的随机字符串（字母数字）。
func Random(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// RandomNum 生成指定长度的随机数字字符串。
func RandomNum(length int) string {
	const charset = "0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// RandomLetter 生成指定长度的随机字母字符串。
func RandomLetter(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
