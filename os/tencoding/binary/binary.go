// Package binary 提供二进制数据与数值之间的编解码。
// 设计要点：
//   - 基于标准库 encoding/binary，零外部依赖。
//   - 默认大端序（BigEndian），同时提供小端序便捷函数。
package binary

import (
	"encoding/binary"
)

// Encode 以大端序将数值编码为字节数组。
func Encode(v any) []byte {
	switch val := v.(type) {
	case uint16:
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, val)
		return b
	case uint32:
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, val)
		return b
	case uint64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, val)
		return b
	case int16:
		return Encode(uint16(val))
	case int32:
		return Encode(uint32(val))
	case int64:
		return Encode(uint64(val))
	case int:
		return Encode(uint64(val))
	case float32:
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, Float32ToUint32(val))
		return b
	case float64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, Float64ToUint64(val))
		return b
	default:
		return nil
	}
}

// Decode 以大端序将字节数组解码为数值。
func Decode(b []byte) (uint64, error) {
	switch len(b) {
	case 1:
		return uint64(b[0]), nil
	case 2:
		return uint64(binary.BigEndian.Uint16(b)), nil
	case 4:
		return uint64(binary.BigEndian.Uint32(b)), nil
	case 8:
		return binary.BigEndian.Uint64(b), nil
	default:
		return 0, nil
	}
}

// EncodeLE 以小端序将数值编码为字节数组。
func EncodeLE(v any) []byte {
	switch val := v.(type) {
	case uint16:
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, val)
		return b
	case uint32:
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, val)
		return b
	case uint64:
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, val)
		return b
	default:
		return Encode(v) // 回退
	}
}

// DecodeLE 以小端序将字节数组解码为数值。
func DecodeLE(b []byte) (uint64, error) {
	switch len(b) {
	case 1:
		return uint64(b[0]), nil
	case 2:
		return uint64(binary.LittleEndian.Uint16(b)), nil
	case 4:
		return uint64(binary.LittleEndian.Uint32(b)), nil
	case 8:
		return binary.LittleEndian.Uint64(b), nil
	default:
		return 0, nil
	}
}

// Float32ToUint32 浮点转 uint32（保留 IEEE 754 位模式）。
func Float32ToUint32(f float32) uint32 { return binary.BigEndian.Uint32(Float32ToBytes(f)) }

// Float64ToUint64 浮点转 uint64。
func Float64ToUint64(f float64) uint64 { return binary.BigEndian.Uint64(Float64ToBytes(f)) }

// Float32ToBytes float32 → []byte。
func Float32ToBytes(f float32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, Float32ToUint32(f))
	return b
}

// Float64ToBytes float64 → []byte。
func Float64ToBytes(f float64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, Float64ToUint64(f))
	return b
}
