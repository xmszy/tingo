package errors

import "net/http"

// 内置业务错误码。业务方应定义自己的码，这些仅供框架内部与通用场景使用。
const (
	CodeBadRequest   = "BAD_REQUEST"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeConflict     = "CONFLICT"
	CodeValidation   = "VALIDATION_FAILED"
	CodeTooManyRequests = "TOO_MANY_REQUESTS"
	CodeInternal     = "INTERNAL_ERROR"
	CodeUnavailable  = "SERVICE_UNAVAILABLE"
	CodeTimeout      = "TIMEOUT"
)

// 内置错误实例。使用 Wrap / WithMessage 派生副本，不要直接修改。
var (
	// ErrBadRequest 请求参数错误。
	ErrBadRequest = NewError(http.StatusBadRequest, CodeBadRequest, "请求参数错误")
	// ErrUnauthorized 未认证。
	ErrUnauthorized = NewError(http.StatusUnauthorized, CodeUnauthorized, "未认证或登录已过期")
	// ErrForbidden 无权限。
	ErrForbidden = NewError(http.StatusForbidden, CodeForbidden, "无访问权限")
	// ErrNotFound 资源不存在。
	ErrNotFound = NewError(http.StatusNotFound, CodeNotFound, "资源不存在")
	// ErrMethodNotAllowed 方法不允许。
	ErrMethodNotAllowed = NewError(http.StatusMethodNotAllowed, CodeMethodNotAllowed, "请求方法不允许")
	// ErrConflict 资源冲突。
	ErrConflict = NewError(http.StatusConflict, CodeConflict, "资源冲突")
	// ErrValidation 参数校验失败。
	ErrValidation = NewError(http.StatusUnprocessableEntity, CodeValidation, "参数校验失败")
	// ErrTooManyRequests 请求过于频繁。
	ErrTooManyRequests = NewError(http.StatusTooManyRequests, CodeTooManyRequests, "请求过于频繁")
	// ErrInternal 服务内部错误。
	ErrInternal = NewError(http.StatusInternalServerError, CodeInternal, "服务内部错误")
	// ErrUnavailable 服务不可用。
	ErrUnavailable = NewError(http.StatusServiceUnavailable, CodeUnavailable, "服务暂不可用")
	// ErrTimeout 请求超时。
	ErrTimeout = NewError(http.StatusGatewayTimeout, CodeTimeout, "请求超时")
)
