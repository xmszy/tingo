// Package trand 提供随机数生成工具。
// 设计要点：
//   - 基于标准库 math/rand/v2，零外部依赖。
//   - 提供字母/数字/字符串/切片等常用随机操作。
package trand

import (
	"math/rand/v2"
	"strings"
)

const (
	letters    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits     = "0123456789"
	lettersLow = "abcdefghijklmnopqrstuvwxyz"
)

// Int 返回 [0, max) 的随机整数。
func Int(max int) int { return rand.N(max) }

// IntRange 返回 [min, max] 的随机整数。
func IntRange(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.N(max-min+1)
}

// Float64 返回 [0.0, 1.0) 的随机浮点数。
func Float64() float64 { return rand.Float64() }

// String 返回指定长度的随机字符串（字母）。
func String(n int) string { return gen(n, letters) }

// Digits 返回指定长度的随机数字串。
func Digits(n int) string { return gen(n, digits) }

// Letters 返回指定长度的随机混合字母数字串。
func Letters(n int) string { return gen(n, letters+digits) }

// LowerLetters 返回指定长度的小写字母数字串。
func LowerLetters(n int) string { return gen(n, lettersLow+digits) }

// Symbol 返回指定长度的随机字符（含特殊符号）。
func Symbol(n int) string { return gen(n, letters+digits+"!@#$%^&*()_+-=[]{}|;:,.<>?") }

func gen(n int, charset string) string {
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte(charset[rand.N(len(charset))])
	}
	return b.String()
}

// Perm 返回 [0, n) 的随机排列。
func Perm(n int) []int { return rand.Perm(n) }

// Shuffle 随机打乱切片。
func Shuffle[T any](s []T) {
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// Pick 从切片中随机选取一个元素。
func Pick[T any](s []T) T {
	return s[rand.N(len(s))]
}

// PickN 从切片中随机选取 n 个不重复元素。
func PickN[T any](s []T, n int) []T {
	if n >= len(s) {
		result := make([]T, len(s))
		copy(result, s)
		Shuffle(result)
		return result
	}
	indices := Perm(len(s))[:n]
	result := make([]T, n)
	for i, idx := range indices {
		result[i] = s[idx]
	}
	return result
}
