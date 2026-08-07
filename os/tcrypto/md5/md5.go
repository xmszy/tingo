// Package md5 提供 MD5 摘要计算。
// 设计要点：
//   - 基于标准库 crypto/md5，零外部依赖。
//   - 提供 Encrypt/MustEncrypt 双版本，以及 Bytes/String/File 便捷变体。
//   - 返回值统一为十六进制小写字符串。
package md5

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Encrypt 计算任意类型数据的 MD5 摘要，返回十六进制小写字符串。
func Encrypt(data any) (string, error) {
	bs, err := toBytes(data)
	if err != nil {
		return "", err
	}
	return EncryptBytes(bs)
}

// MustEncrypt 计算 MD5 摘要，失败时 panic。
func MustEncrypt(data any) string {
	s, err := Encrypt(data)
	if err != nil {
		panic(fmt.Sprintf("md5.MustEncrypt: %v", err))
	}
	return s
}

// EncryptBytes 计算字节数据的 MD5 摘要。
func EncryptBytes(data []byte) (string, error) {
	h := md5.New()
	if _, err := h.Write(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncryptBytes 计算字节数据的 MD5 摘要，失败时 panic。
func MustEncryptBytes(data []byte) string {
	s, err := EncryptBytes(data)
	if err != nil {
		panic(fmt.Sprintf("md5.MustEncryptBytes: %v", err))
	}
	return s
}

// EncryptString 计算字符串的 MD5 摘要。
func EncryptString(data string) (string, error) {
	return EncryptBytes([]byte(data))
}

// MustEncryptString 计算字符串的 MD5 摘要，失败时 panic。
func MustEncryptString(data string) string {
	return MustEncryptBytes([]byte(data))
}

// EncryptFile 计算文件的 MD5 摘要。
func EncryptFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncryptFile 计算文件的 MD5 摘要，失败时 panic。
func MustEncryptFile(path string) string {
	s, err := EncryptFile(path)
	if err != nil {
		panic(fmt.Sprintf("md5.MustEncryptFile: %v", err))
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
		return fmt.Appendf(nil, "%v", v), nil
	}
}
