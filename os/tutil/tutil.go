// Package tutil 提供杂项工具函数。
// 设计要点：
//   - 基于标准库，零外部依赖。
//   - 提供 Map/Filter/Reduce、DeepCopy、三元运算等工具函数。
package tutil

import (
	"slices"
	"encoding/gob"
	"bytes"
)

// ──────────────── 函数式工具 ────────────────

// Map 对切片每个元素应用函数。
func Map[T, U any](s []T, fn func(T) U) []U {
	result := make([]U, len(s))
	for i, v := range s {
		result[i] = fn(v)
	}
	return result
}

// Filter 过滤切片。
func Filter[T any](s []T, fn func(T) bool) []T {
	var result []T
	for _, v := range s {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce 归约操作。
func Reduce[T, U any](s []T, init U, fn func(acc U, item T) U) U {
	for _, v := range s {
		init = fn(init, v)
	}
	return init
}

// Contains 判断元素是否在切片中（要求 comparable）。
func Contains[T comparable](s []T, v T) bool {
	return slices.Contains(s, v)
}

// IndexOf 查找元素在切片中的位置，未找到返回 -1。
func IndexOf[T comparable](s []T, v T) int {
	for i, item := range s {
		if item == v {
			return i
		}
	}
	return -1
}

// Remove 按值移除切片中的元素。
func Remove[T comparable](s []T, v T) []T {
	for i, item := range s {
		if item == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// Unique 去重（保持原顺序）。
func Unique[T comparable](s []T) []T {
	seen := make(map[T]struct{}, len(s))
	result := make([]T, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// Reverse 反转切片。
func Reverse[T any](s []T) []T {
	result := make([]T, len(s))
	copy(result, s)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// ──────────────── 三元运算 ────────────────

// Ternary 泛型三元运算：if cond then a else b。
func Ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// ──────────────── 深拷贝 ────────────────

// DeepCopy 使用 gob 进行深拷贝。v 必须可 gob 编码。
func DeepCopy[T any](v T) (T, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		var zero T
		return zero, err
	}
	dec := gob.NewDecoder(&buf)
	var result T
	if err := dec.Decode(&result); err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// ──────────────── 默认值 ────────────────

// Default 如果 v 为零值则返回 def。
func Default[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}
