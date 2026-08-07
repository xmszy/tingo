// Package crc32 提供 CRC-32 校验值计算。
// 设计要点：
//   - 基于标准库 hash/crc32，零外部依赖。
//   - 默认使用 IEEE 多项式，支持切换为 Castagnoli 或 Koopman。
//   - 返回 uint32，并提供 Hex 格式的便捷方法。
package crc32

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// 默认表 = IEEE。
var table = crc32.MakeTable(crc32.IEEE)

// SetTable 设置 CRC 多项式表。默认为 IEEE。
// 可切换为 crc32.Castagnoli 或 crc32.Koopman。
func SetTable(t *crc32.Table) { table = t }

// Encrypt 计算任意类型数据的 CRC-32 校验值。
func Encrypt(data any) (uint32, error) {
	bs, err := toBytes(data)
	if err != nil {
		return 0, err
	}
	return EncryptBytes(bs), nil
}

// MustEncrypt 计算 CRC-32 校验值，失败时 panic。
func MustEncrypt(data any) uint32 {
	v, err := Encrypt(data)
	if err != nil {
		panic(fmt.Sprintf("crc32.MustEncrypt: %v", err))
	}
	return v
}

// EncryptBytes 计算字节数据的 CRC-32 校验值。
func EncryptBytes(data []byte) uint32 {
	return crc32.Checksum(data, table)
}

// EncryptString 计算字符串的 CRC-32 校验值。
func EncryptString(data string) uint32 {
	return crc32.Checksum([]byte(data), table)
}

// EncryptFile 计算文件的 CRC-32 校验值。
func EncryptFile(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	h := crc32.New(table)
	if _, err := io.Copy(h, f); err != nil {
		return 0, err
	}
	return h.Sum32(), nil
}

// MustEncryptFile 计算文件的 CRC-32 校验值，失败时 panic。
func MustEncryptFile(path string) uint32 {
	v, err := EncryptFile(path)
	if err != nil {
		panic(fmt.Sprintf("crc32.MustEncryptFile: %v", err))
	}
	return v
}

// EncryptStringHex 计算字符串的 CRC-32，返回十六进制字符串。
func EncryptStringHex(data string) string {
	return fmt.Sprintf("%08x", crc32.Checksum([]byte(data), table))
}

// EncryptBytesHex 计算字节数据的 CRC-32，返回十六进制字符串。
func EncryptBytesHex(data []byte) string {
	return fmt.Sprintf("%08x", crc32.Checksum(data, table))
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
