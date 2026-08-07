// Package errors 提供 tingo 的结构化错误体系。
//
// 设计要点：
//   - 错误即数据：错误携带 HTTP 状态码、业务码、消息、元数据，可直接序列化给客户端。
//   - 完全兼容标准库：支持 errors.Is / errors.As / errors.Unwrap / %w。
//   - 零分配比较：Is 基于业务码比较，不做字符串拼接。
package errors

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
)

// 标准库转发，避免业务代码同时 import 两个 errors 包。
var (
	// New 创建一个普通错误（无业务码）。
	New = errors.New
	// Is 判断错误链中是否存在目标错误。
	Is = errors.Is
	// As 从错误链中提取指定类型的错误。
	As = errors.As
	// Unwrap 解包一层错误。
	Unwrap = errors.Unwrap
	// Join 合并多个错误。
	Join = errors.Join
)

// Error 是 tingo 的结构化错误。
type Error struct {
	// Status 是建议的 HTTP 状态码。
	Status int `json:"-"`
	// Code 是业务错误码，用于程序判断，全局唯一。
	Code string `json:"code"`
	// Message 是面向用户的错误消息。
	Message string `json:"message"`
	// Meta 是附加的结构化信息，如字段级校验错误。
	Meta map[string]any `json:"meta,omitempty"`

	// cause 是被包装的底层错误，不参与序列化。
	cause error
}

// 确保实现标准接口。
var (
	_ error = (*Error)(nil)
)

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e.cause != nil {
		return e.Code + ": " + e.Message + ": " + e.cause.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap 返回被包装的底层错误。
func (e *Error) Unwrap() error { return e.cause }

// Is 基于业务码比较，使 errors.Is 可用于同码不同实例的错误。
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return t.Code == e.Code
}

/* ------------------------------------------------------------------ */
/* 构造                                                                */
/* ------------------------------------------------------------------ */

// NewError 创建一个结构化错误。
//
//	var ErrUserNotFound = errors.NewError(404, "USER_NOT_FOUND", "用户不存在")
func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Newf 创建一个消息带格式化的结构化错误。
func Newf(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

/* ------------------------------------------------------------------ */
/* 链式派生：所有方法返回副本，原错误变量不被污染                          */
/* ------------------------------------------------------------------ */

// clone 返回错误的浅拷贝。
// 由于 Error 常被声明为包级变量，任何修改都必须在副本上进行。
func (e *Error) clone() *Error {
	c := &Error{
		Status:  e.Status,
		Code:    e.Code,
		Message: e.Message,
		cause:   e.cause,
	}
	if len(e.Meta) > 0 {
		c.Meta = make(map[string]any, len(e.Meta))
		maps.Copy(c.Meta, e.Meta)
	}
	return c
}

// Wrap 包装一个底层错误，返回新副本。
func (e *Error) Wrap(cause error) *Error {
	c := e.clone()
	c.cause = cause
	return c
}

// WithMessage 替换消息，返回新副本。
func (e *Error) WithMessage(msg string) *Error {
	c := e.clone()
	c.Message = msg
	return c
}

// WithMessagef 以格式化方式替换消息，返回新副本。
func (e *Error) WithMessagef(format string, args ...any) *Error {
	c := e.clone()
	c.Message = fmt.Sprintf(format, args...)
	return c
}

// WithMeta 追加元数据，返回新副本。
func (e *Error) WithMeta(key string, val any) *Error {
	c := e.clone()
	if c.Meta == nil {
		c.Meta = make(map[string]any, 4)
	}
	c.Meta[key] = val
	return c
}

// WithStatus 替换 HTTP 状态码，返回新副本。
func (e *Error) WithStatus(status int) *Error {
	c := e.clone()
	c.Status = status
	return c
}

/* ------------------------------------------------------------------ */
/* 提取                                                                */
/* ------------------------------------------------------------------ */

// From 从任意 error 中提取 *Error。
// 若错误链中不存在结构化错误，则包装为 500 内部错误。
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternal,
		Message: err.Error(),
		cause:   err,
	}
}

// StatusOf 返回错误建议的 HTTP 状态码，非结构化错误返回 500。
func StatusOf(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var e *Error
	if errors.As(err, &e) && e.Status > 0 {
		return e.Status
	}
	return http.StatusInternalServerError
}

// CodeOf 返回错误链中第一个 *Error 的业务码；无则返回 CodeInternal。
// 注意：内部使用 errors.As 递归 Unwrap，因此遍历整条错误链。
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// HasCode 检查错误链中是否存在指定业务码。
// 递归遍历 cause 链，判断错误归属。
func HasCode(err error, code string) bool {
	if err == nil || code == "" {
		return false
	}
	var e *Error
	if As(err, &e) {
		if e.Code == code {
			return true
		}
		if e.cause != nil {
			return HasCode(e.cause, code)
		}
		return false
	}
	// 非 *Error 类型也继续 Unwrap
	if u := Unwrap(err); u != nil {
		return HasCode(u, code)
	}
	return false
}
