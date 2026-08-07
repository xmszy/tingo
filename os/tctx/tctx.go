// Package tctx 提供 context 工具函数。
//
// 设计要点：
//   - 基于标准库 context，零外部依赖。
//   - 泛型 Value 读取，免除类型断言。
//   - WithValue 链式设置多个值。
package tctx

import "context"

// ctxKey 用于避免 key 冲突的内部类型。
type ctxKey struct{ name string }

// WithValue 在 context 中设置带类型的键值对。
func WithValue(ctx context.Context, key string, value any) context.Context {
	return context.WithValue(ctx, ctxKey{key}, value)
}

// Value 从 context 中读取泛型值。
func Value[T any](ctx context.Context, key string) (T, bool) {
	v := ctx.Value(ctxKey{key})
	if v == nil {
		return *new(T), false
	}
	tv, ok := v.(T)
	return tv, ok
}

// MustValue 从 context 中读取值，失败返回零值。
func MustValue[T any](ctx context.Context, key string) T {
	v, _ := Value[T](ctx, key)
	return v
}

// WithValues 批量设置多个键值对，返回新的 context。
func WithValues(ctx context.Context, kv map[string]any) context.Context {
	for k, v := range kv {
		ctx = WithValue(ctx, k, v)
	}
	return ctx
}
