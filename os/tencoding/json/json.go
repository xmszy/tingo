// Package json 提供 JSON 编解码。
// 设计要点：
//   - 基于标准库 encoding/json，零外部依赖。
//   - 提供 Marshal/Unmarshal + Must 版本 + 动态 New/Set/Get 操作。
package json

import (
	"encoding/json"
	"fmt"
)

// Marshal 将值序列化为 JSON 字节数组。
func Marshal(v any) ([]byte, error) { return json.Marshal(v) }

// MustMarshal 序列化，失败时 panic。
func MustMarshal(v any) []byte { b, err := Marshal(v); if err != nil { panic(err) }; return b }

// MarshalString 序列化为 JSON 字符串。
func MarshalString(v any) (string, error) { b, err := Marshal(v); return string(b), err }

// MustMarshalString 序列化为 JSON 字符串，失败时 panic。
func MustMarshalString(v any) string { return string(MustMarshal(v)) }

// MarshalIndent 带缩进的序列化。
func MarshalIndent(v any, prefix, indent string) ([]byte, error) { return json.MarshalIndent(v, prefix, indent) }

// Unmarshal 反序列化 JSON 字节数组到指定类型。
func Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// UnmarshalString 反序列化 JSON 字符串。
func UnmarshalString(data string, v any) error { return Unmarshal([]byte(data), v) }

// Valid 判断 JSON 是否合法。
func Valid(data []byte) bool { return json.Valid(data) }

// ──────────────── 动态 JSON 操作 ────────────────

// New 创建空 JSON 对象。
func New(data ...any) *Json {
	j := &Json{data: make(map[string]any)}
	for _, d := range data {
		switch v := d.(type) {
		case string:
			json.Unmarshal([]byte(v), &j.data)
		case []byte:
			json.Unmarshal(v, &j.data)
		case map[string]any:
			j.data = v
		}
	}
	return j
}

// Json 动态 JSON 对象。
type Json struct {
	data any
}

// Set 设置指定路径的值（支持 "a.b.c" 点号分隔）。
func (j *Json) Set(path string, value any) error {
	m, ok := j.data.(map[string]any)
	if !ok {
		return fmt.Errorf("json: root is not an object")
	}
	return setPath(m, path, value)
}

// Get 获取指定路径的值。
func (j *Json) Get(path string) *Json {
	m, ok := j.data.(map[string]any)
	if !ok {
		return &Json{}
	}
	return &Json{data: getPath(m, path)}
}

// GetString 获取字符串。
func (j *Json) GetString(path string) string {
	return j.Get(path).String()
}

// GetInt 获取整数。
func (j *Json) GetInt(path string) int {
	v := j.Get(path).Interface()
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// String 返回字符串表示。
func (j *Json) String() string {
	if s, ok := j.data.(string); ok {
		return s
	}
	b, _ := Marshal(j.data)
	return string(b)
}

// Interface 返回原始值。
func (j *Json) Interface() any { return j.data }

// Map 返回 map 表示。
func (j *Json) Map() map[string]any {
	if m, ok := j.data.(map[string]any); ok {
		return m
	}
	return nil
}

// Array 返回数组表示。
func (j *Json) Array() []any {
	if a, ok := j.data.([]any); ok {
		return a
	}
	return nil
}

func setPath(m map[string]any, path string, value any) error {
	keys := splitPath(path)
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		existing, ok := m[key]
		if !ok {
			existing = make(map[string]any)
			m[key] = existing
		}
		next, ok := existing.(map[string]any)
		if !ok {
			return fmt.Errorf("json: cannot set nested key %q on non-object", key)
		}
		m = next
	}
	m[keys[len(keys)-1]] = value
	return nil
}

func getPath(m map[string]any, path string) any {
	keys := splitPath(path)
	var current any = m
	for _, key := range keys {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = cm[key]
	}
	return current
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var keys []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			keys = append(keys, path[start:i])
			start = i + 1
		}
	}
	keys = append(keys, path[start:])
	return keys
}
