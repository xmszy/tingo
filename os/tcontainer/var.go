// Package tcontainer Var 通用变量容器。
//
// Var 是对任意类型值的统一包装，支持：
//   - 并发安全的取值/赋值（safe 模式）。
//   - 便捷的类型转换取值（Int/String/Time/...），委托 tconv。
//   - 直接反序列化为 struct / map / slice。
//
// 典型用法：
//
//	v := tcontainer.NewVar(map[string]any{"id": 1, "name": "foo"})
//	id := v.Int("id")        // 1
//	name := v.String("name") // "foo"
//	var u User
//	v.Scan(&u)               // 填充结构体
package tcontainer

import (
	"reflect"
	"time"

	"github.com/xmszy/tingo/os/tconv"
	"github.com/xmszy/tingo/os/ttype"
)

// Var 通用变量容器。
type Var struct {
	safe bool       // 是否并发安全
	val  *ttype.Any // 始终使用并发安全的 Any 承载值
}

// NewVar 创建通用变量。
// safe 为 true（或传入任意参数）时启用并发安全（内部基于 ttype.Any，始终安全）。
func NewVar(value any, safe ...bool) *Var {
	v := &Var{safe: len(safe) > 0 && safe[0]}
	v.val = ttype.NewAny(value)
	return v
}

// Val 返回原始值。
func (v *Var) Val() any { return v.val.Val() }

// Set 设置新值。
func (v *Var) Set(value any) { v.val.Set(value) }

// Clone 克隆一份独立副本。
func (v *Var) Clone() *Var { return NewVar(v.val.Val(), v.safe) }

// IsNil 判断当前值是否为 nil。
func (v *Var) IsNil() bool { return v.val.Val() == nil }

// IsSafe 是否并发安全模式。
func (v *Var) IsSafe() bool { return v.safe }

// ──────────────── 基础类型取值 ────────────────

// Int 取值并转为 int。
func (v *Var) Int() int { return tconv.Int(v.val.Val()) }

// Int64 取值并转为 int64。
func (v *Var) Int64() int64 { return tconv.Int64(v.val.Val()) }

// Uint 取值并转为 uint。
func (v *Var) Uint() uint { return tconv.Uint(v.val.Val()) }

// Uint64 取值并转为 uint64。
func (v *Var) Uint64() uint64 { return tconv.Uint64(v.val.Val()) }

// Float32 取值并转为 float32。
func (v *Var) Float32() float32 { return tconv.Float32(v.val.Val()) }

// Float64 取值并转为 float64。
func (v *Var) Float64() float64 { return tconv.Float64(v.val.Val()) }

// String 取值并转为 string。
func (v *Var) String() string { return tconv.String(v.val.Val()) }

// Bool 取值并转为 bool。
func (v *Var) Bool() bool { return tconv.Bool(v.val.Val()) }

// Bytes 取值并转为 []byte。
func (v *Var) Bytes() []byte { return tconv.Bytes(v.val.Val()) }

// Time 取值并转为 time.Time。
func (v *Var) Time() (t time.Time) { return tconv.Time(v.val.Val()) }

// ──────────────── 复合类型取值 ────────────────

// Map 取值并转为 map[string]any。
func (v *Var) Map() map[string]any { return tconv.Map(v.val.Val()) }

// Slice 取值并转为 []any（非切片值返回 nil）。
func (v *Var) Slice() []any {
	rv := reflect.ValueOf(v.val.Val())
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil
	}
	n := rv.Len()
	out := make([]any, n)
	for i := 0; i < n; i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

// List 等价于 Slice，返回 []any。
func (v *Var) List() []any { return v.Slice() }

// Struct 将当前值反序列化到 ptr 指向的结构体。
// 委托 tconv.MapToStruct，支持 map / struct 输入。
func (v *Var) Struct(ptr any) error { return tconv.MapToStruct(v.val.Val(), ptr) }

// Scan 等同于 Struct，符合 scan 语义。
func (v *Var) Scan(dst any) error { return tconv.MapToStruct(v.val.Val(), dst) }
