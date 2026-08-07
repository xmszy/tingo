// Package yaml 提供 YAML 编解码。
// 设计要点：
//   - 基于 gopkg.in/yaml.v3（gin 已依赖），无新增外部依赖。
package yaml

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// Marshal 将值序列化为 YAML 字节数组。
func Marshal(v any) ([]byte, error) { return yaml.Marshal(v) }

// MustMarshal 序列化，失败时 panic。
func MustMarshal(v any) []byte { b, err := Marshal(v); if err != nil { panic(err) }; return b }

// MarshalString 序列化为 YAML 字符串。
func MarshalString(v any) (string, error) { b, err := Marshal(v); return string(b), err }

// MustMarshalString 序列化，失败时 panic。
func MustMarshalString(v any) string { return string(MustMarshal(v)) }

// Unmarshal 反序列化 YAML 字节数组。
func Unmarshal(data []byte, v any) error { return yaml.Unmarshal(data, v) }

// UnmarshalString 反序列化 YAML 字符串。
func UnmarshalString(data string, v any) error { return Unmarshal([]byte(data), v) }

// ToJSON 将 YAML 字节数组转为 JSON 字节数组。
func ToJSON(yamlData []byte) ([]byte, error) {
	var v any
	if err := Unmarshal(yamlData, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
