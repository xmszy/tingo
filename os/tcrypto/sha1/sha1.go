// Package sha1 提供 SHA-1 摘要计算。
// 设计要点：
//   - 基于标准库 crypto/sha1，零外部依赖。
//   - 提供 Encrypt/MustEncrypt 双版本，以及 Bytes/String/File 便捷变体。
//   - 返回值统一为十六进制小写字符串。
//   - 注意：SHA-1 已不再适合安全场景，本包仅提供兼容性。
package sha1

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Encrypt 计算任意类型数据的 SHA-1 摘要，返回十六进制小写字符串。
func Encrypt(data any) (string, error) {
	bs, err := toBytes(data)
	if err != nil {
		return "", err
	}
	return EncryptBytes(bs)
}

// MustEncrypt 计算 SHA-1 摘要，失败时 panic。
func MustEncrypt(data any) string {
	s, err := Encrypt(data)
	if err != nil {
		panic(fmt.Sprintf("sha1.MustEncrypt: %v", err))
	}
	return s
}

// EncryptBytes 计算字节数据的 SHA-1 摘要。
func EncryptBytes(data []byte) (string, error) {
	h := sha1.New()
	if _, err := h.Write(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncryptBytes 计算字节数据的 SHA-1 摘要，失败时 panic。
func MustEncryptBytes(data []byte) string {
	s, err := EncryptBytes(data)
	if err != nil {
		panic(fmt.Sprintf("sha1.MustEncryptBytes: %v", err))
	}
	return s
}

// EncryptString 计算字符串的 SHA-1 摘要。
func EncryptString(data string) (string, error) {
	return EncryptBytes([]byte(data))
}

// MustEncryptString 计算字符串的 SHA-1 摘要，失败时 panic。
func MustEncryptString(data string) string {
	return MustEncryptBytes([]byte(data))
}

// EncryptFile 计算文件的 SHA-1 摘要。
func EncryptFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncryptFile 计算文件的 SHA-1 摘要，失败时 panic。
func MustEncryptFile(path string) string {
	s, err := EncryptFile(path)
	if err != nil {
		panic(fmt.Sprintf("sha1.MustEncryptFile: %v", err))
	}
	return s
}

// toBytes 将任意类型转为 []byte。
func toBytes(data any) ([]byte, error) {
	switch v := data.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case fmt.Stringer:
		return []byte(v.String()), nil
	default:
		return fmt.Append(nil, v), nil
	}
}
