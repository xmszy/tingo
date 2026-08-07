// Package tconv 指针辅助、泛型 Must 与零拷贝转换。
package tconv

import (
	"unsafe"
)

// ──────────────── Ptr* 辅助函数 ────────────────
// 将值转为对应类型的指针，用于数据库 nullable 字段、protobuf optional 等场景。

// PtrString 返回 *string。
func PtrString(v any) *string { s := String(v); return &s }

// PtrInt 返回 *int。
func PtrInt(v any) *int { n := Int(v); return &n }

// PtrInt8 返回 *int8。
func PtrInt8(v any) *int8 { n := Int8(v); return &n }

// PtrInt16 返回 *int16。
func PtrInt16(v any) *int16 { n := Int16(v); return &n }

// PtrInt32 返回 *int32。
func PtrInt32(v any) *int32 { n := Int32(v); return &n }

// PtrInt64 返回 *int64。
func PtrInt64(v any) *int64 { n := Int64(v); return &n }

// PtrUint 返回 *uint。
func PtrUint(v any) *uint { n := Uint(v); return &n }

// PtrUint8 返回 *uint8。
func PtrUint8(v any) *uint8 { n := Uint8(v); return &n }

// PtrUint16 返回 *uint16。
func PtrUint16(v any) *uint16 { n := Uint16(v); return &n }

// PtrUint32 返回 *uint32。
func PtrUint32(v any) *uint32 { n := Uint32(v); return &n }

// PtrUint64 返回 *uint64。
func PtrUint64(v any) *uint64 { n := Uint64(v); return &n }

// PtrFloat32 返回 *float32。
func PtrFloat32(v any) *float32 { n := Float32(v); return &n }

// PtrFloat64 返回 *float64。
func PtrFloat64(v any) *float64 { n := Float64(v); return &n }

// PtrBool 返回 *bool。
func PtrBool(v any) *bool { b := Bool(v); return &b }

// ──────────────── 泛型 Must ────────────────

// Must 在 err != nil 时 panic，否则返回 v。
// 用于初始化阶段、配置加载等不允许失败的场景。
//
//	cfg := tconv.Must(tconv.Int("42"), nil) // cfg = 42
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// ──────────────── 零拷贝转换 ────────────────

// UnsafeStrToBytes 将 string 零拷贝转为 []byte。
// 警告：返回的 []byte 不可修改，否则会触发运行时 panic。
// 用于只读场景（日志、哈希、传输等），避免内存分配。
func UnsafeStrToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// UnsafeBytesToStr 将 []byte 零拷贝转为 string。
// 警告：原 []byte 被修改时 string 内容也会变化，在并发场景下需注意。
func UnsafeBytesToStr(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
