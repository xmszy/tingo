// Package xml 提供 XML 编解码。
// 设计要点：
//   - 基于标准库 encoding/xml，零外部依赖。
//   - 提供 Marshal/Unmarshal + Must 版本。
package xml

import (
	"encoding/xml"
)

// Marshal 将值序列化为 XML 字节数组。
func Marshal(v any) ([]byte, error) { return xml.Marshal(v) }

// MustMarshal 序列化，失败时 panic。
func MustMarshal(v any) []byte { b, err := Marshal(v); if err != nil { panic(err) }; return b }

// MarshalString 序列化为 XML 字符串。
func MarshalString(v any) (string, error) { b, err := Marshal(v); return string(b), err }

// MustMarshalString 序列化为 XML 字符串，失败时 panic。
func MustMarshalString(v any) string { return string(MustMarshal(v)) }

// MarshalIndent 带缩进的序列化。
func MarshalIndent(v any, prefix, indent string) ([]byte, error) { return xml.MarshalIndent(v, prefix, indent) }

// Unmarshal 反序列化 XML 字节数组。
func Unmarshal(data []byte, v any) error { return xml.Unmarshal(data, v) }

// UnmarshalString 反序列化 XML 字符串。
func UnmarshalString(data string, v any) error { return Unmarshal([]byte(data), v) }

// Encode 字符串 XML 转义（防止注入）。
func Encode(s string) string { return xmlEscape(s) }

// Decode 字符串 XML 反转义。
func Decode(s string) string { b, _ := xmlUnescape(s); return b }

func xmlEscape(s string) string {
	b, _ := xml.Marshal(struct{ S string }{S: s})
	// 去掉 <xml> 外层
	out := string(b)
	if len(out) > 19 && out[:8] == "<xml><S>" {
		out = out[8 : len(out)-12]
	}
	return out
}

func xmlUnescape(s string) (string, error) {
	var v struct{ S string }
	err := xml.Unmarshal([]byte("<xml><S>"+s+"</S></xml>"), &v)
	return v.S, err
}
