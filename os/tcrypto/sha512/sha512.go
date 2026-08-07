// Package sha512 提供 SHA-512 摘要计算。
// 设计要点：
//   - 基于标准库 crypto/sha512，零外部依赖。
//   - 提供 Encrypt（不返回 error）、EncryptFile/MustEncryptFile。
//   - 额外提供 SHA-384 和 SHA-512/256 变体。
//   - 返回值统一为十六进制小写字符串。
package sha512

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Encrypt 计算任意类型数据的 SHA-512 摘要，返回十六进制小写字符串。
func Encrypt(data any) string {
	bs, _ := toBytes(data)
	return EncryptBytes(bs)
}

// EncryptBytes 计算字节数据的 SHA-512 摘要。
func EncryptBytes(data []byte) string {
	sum := sha512.Sum512(data)
	return hex.EncodeToString(sum[:])
}

// EncryptString 计算字符串的 SHA-512 摘要。
func EncryptString(data string) string {
	return EncryptBytes([]byte(data))
}

// EncryptFile 计算文件的 SHA-512 摘要。
func EncryptFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncryptFile 计算文件的 SHA-512 摘要，失败时 panic。
func MustEncryptFile(path string) string {
	s, err := EncryptFile(path)
	if err != nil {
		panic(fmt.Sprintf("sha512.MustEncryptFile: %v", err))
	}
	return s
}

// Encrypt384 计算任意类型数据的 SHA-384 摘要。
func Encrypt384(data any) string {
	bs, _ := toBytes(data)
	sum := sha512.Sum384(bs)
	return hex.EncodeToString(sum[:])
}

// Encrypt384Bytes 计算字节数据的 SHA-384 摘要。
func Encrypt384Bytes(data []byte) string {
	sum := sha512.Sum384(data)
	return hex.EncodeToString(sum[:])
}

// Encrypt384String 计算字符串的 SHA-384 摘要。
func Encrypt384String(data string) string {
	return Encrypt384Bytes([]byte(data))
}

// Encrypt384File 计算文件的 SHA-384 摘要。
func Encrypt384File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha512.New384()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncrypt384File 计算文件的 SHA-384 摘要，失败时 panic。
func MustEncrypt384File(path string) string {
	s, err := Encrypt384File(path)
	if err != nil {
		panic(fmt.Sprintf("sha512.MustEncrypt384File: %v", err))
	}
	return s
}

// Encrypt512_256 计算任意类型数据的 SHA-512/256 摘要。
func Encrypt512_256(data any) string {
	bs, _ := toBytes(data)
	sum := sha512.Sum512_256(bs)
	return hex.EncodeToString(sum[:])
}

// Encrypt512_256Bytes 计算字节数据的 SHA-512/256 摘要。
func Encrypt512_256Bytes(data []byte) string {
	sum := sha512.Sum512_256(data)
	return hex.EncodeToString(sum[:])
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
