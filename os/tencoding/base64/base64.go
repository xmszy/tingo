// Package base64 提供 Base64 编解码。
// 设计要点：
//   - 基于标准库 encoding/base64，零外部依赖。
//   - 默认使用 StdEncoding，同时提供 RawStdEncoding / URLEncoding / RawURLEncoding 便捷函数。
package base64

import (
	"encoding/base64"
)

// Encode 使用标准 Base64 编码。
func Encode(src []byte) string { return base64.StdEncoding.EncodeToString(src) }

// Decode 使用标准 Base64 解码。
func Decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

// MustDecode 使用标准 Base64 解码，失败时返回空串。
func MustDecode(s string) []byte { b, _ := Decode(s); return b }

// EncodeString 编码字符串。
func EncodeString(src string) string { return Encode([]byte(src)) }

// DecodeString 解码为字符串。
func DecodeString(s string) (string, error) { b, err := Decode(s); return string(b), err }

// MustDecodeString 解码为字符串，失败时返回空串。
func MustDecodeString(s string) string { return string(MustDecode(s)) }

// EncodeRaw 使用 RawStdEncoding（无填充）编码。
func EncodeRaw(src []byte) string { return base64.RawStdEncoding.EncodeToString(src) }

// DecodeRaw 使用 RawStdEncoding 解码。
func DecodeRaw(s string) ([]byte, error) { return base64.RawStdEncoding.DecodeString(s) }

// EncodeURL 使用 URL 安全的 Base64 编码（RFC 4648 §5）。
func EncodeURL(src []byte) string { return base64.URLEncoding.EncodeToString(src) }

// DecodeURL 使用 URL 安全的 Base64 解码。
func DecodeURL(s string) ([]byte, error) { return base64.URLEncoding.DecodeString(s) }

// EncodeRawURL 使用 RawURLEncoding 编码。
func EncodeRawURL(src []byte) string { return base64.RawURLEncoding.EncodeToString(src) }

// DecodeRawURL 使用 RawURLEncoding 解码。
func DecodeRawURL(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
