// Package sha256 提供 SHA-256 摘要计算。
// 设计要点：
//   - 基于标准库 crypto/sha256，零外部依赖。
//   - 提供 Encrypt（不返回 error，sha256.Sum256 不会出错）、MustEncrypt、EncryptFile/MustEncryptFile。
//   - 返回值统一为十六进制小写字符串。
package sha256

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Encrypt 计算任意类型数据的 SHA-256 摘要，返回十六进制小写字符串。
func Encrypt(data any) string {
	bs, _ := toBytes(data)
	return EncryptBytes(bs)
}

// EncryptBytes 计算字节数据的 SHA-256 摘要。
func EncryptBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// EncryptString 计算字符串的 SHA-256 摘要。
func EncryptString(data string) string {
	return EncryptBytes([]byte(data))
}

// EncryptFile 计算文件的 SHA-256 摘要。
func EncryptFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MustEncryptFile 计算文件的 SHA-256 摘要，失败时 panic。
func MustEncryptFile(path string) string {
	s, err := EncryptFile(path)
	if err != nil {
		panic(fmt.Sprintf("sha256.MustEncryptFile: %v", err))
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
