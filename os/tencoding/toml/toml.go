// Package toml 提供 TOML 编解码。
// 设计要点：
//   - 基于 github.com/pelletier/go-toml/v2（gin 已依赖），无新增外部依赖。
package toml

import (
	toml "github.com/pelletier/go-toml/v2"
)

// Marshal 将值序列化为 TOML 字节数组。
func Marshal(v any) ([]byte, error) { return toml.Marshal(v) }

// MustMarshal 序列化，失败时 panic。
func MustMarshal(v any) []byte { b, err := Marshal(v); if err != nil { panic(err) }; return b }

// MarshalString 序列化为 TOML 字符串。
func MarshalString(v any) (string, error) { b, err := Marshal(v); return string(b), err }

// MustMarshalString 序列化，失败时 panic。
func MustMarshalString(v any) string { return string(MustMarshal(v)) }

// Unmarshal 反序列化 TOML 字节数组。
func Unmarshal(data []byte, v any) error { return toml.Unmarshal(data, v) }

// UnmarshalString 反序列化 TOML 字符串。
func UnmarshalString(data string, v any) error { return Unmarshal([]byte(data), v) }

// Encode 等同于 MarshalString。
func Encode(v any) (string, error) { return MarshalString(v) }

// Decode 等同于 UnmarshalString。
func Decode(data string, v any) error { return UnmarshalString(data, v) }
