package tapp

import (
	"strings"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
)

/* ------------------------------------------------------------------ */
/* 助手函数                                                             */
/* ------------------------------------------------------------------ */

// Abort 中断请求并抛出指定状态码的异常。
//
// 通过 panic 交由 Recover 中间件统一渲染，
// 因此可以在任意调用深度使用，无需层层返回 error。
func Abort(status int, msg ...string) {
	e := errors.NewError(status, statusCode(status), statusText(status))
	if len(msg) > 0 && msg[0] != "" {
		e = e.WithMessage(msg[0])
	}
	panic(e)
}

// AbortIf 在条件成立时中断请求。
func AbortIf(cond bool, status int, msg ...string) {
	if cond {
		Abort(status, msg...)
	}
}

// statusCode 把 HTTP 状态码映射为框架内置业务码。
func statusCode(status int) string {
	switch status {
	case 400:
		return errors.CodeBadRequest
	case 401:
		return errors.CodeUnauthorized
	case 403:
		return errors.CodeForbidden
	case 404:
		return errors.CodeNotFound
	case 405:
		return errors.CodeMethodNotAllowed
	case 409:
		return errors.CodeConflict
	case 422:
		return errors.CodeValidation
	case 429:
		return errors.CodeTooManyRequests
	case 503:
		return errors.CodeUnavailable
	case 504:
		return errors.CodeTimeout
	default:
		return errors.CodeInternal
	}
}

func statusText(status int) string {
	switch status {
	case 400:
		return "请求参数错误"
	case 401:
		return "未认证或登录已过期"
	case 403:
		return "无访问权限"
	case 404:
		return "资源不存在"
	case 405:
		return "请求方法不允许"
	case 409:
		return "资源冲突"
	case 422:
		return "参数校验失败"
	case 429:
		return "请求过于频繁"
	case 503:
		return "服务暂不可用"
	case 504:
		return "请求超时"
	default:
		return "服务内部错误"
	}
}

/* ------------------------------------------------------------------ */
/* 命名转换                                                             */
/* ------------------------------------------------------------------ */

// Snake 把驼峰转为下划线命名。
//
//	Snake("UserProfile") == "user_profile"
//	Snake("APIKey")      == "api_key"
func Snake(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			// 连续大写视为缩写，仅在缩写结束处断词。
			if i > 0 && (runes[i-1] < 'A' || runes[i-1] > 'Z' ||
				(i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z')) {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Camel 把下划线转为大驼峰。
//
//	Camel("user_profile") == "UserProfile"
func Camel(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	upper := true
	for _, r := range s {
		if r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			if r >= 'a' && r <= 'z' {
				r = r - 'a' + 'A'
			}
			upper = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// LowerCamel 把下划线转为小驼峰。
//
//	LowerCamel("user_profile") == "userProfile"
func LowerCamel(s string) string {
	c := Camel(s)
	if c == "" {
		return c
	}
	r := []rune(c)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] = r[0] - 'A' + 'a'
	}
	return string(r)
}

/* ------------------------------------------------------------------ */
/* 响应助手                                                             */
/* ------------------------------------------------------------------ */

// JSON 输出统一格式的成功响应，供非控制器场景（如中间件）使用。
func JSON(c *core.Ctx, data any, msg ...string) {
	m := "success"
	if len(msg) > 0 {
		m = msg[0]
	}
	c.JSONStatus(200, &Result{Code: CodeSuccess, Msg: m, Data: data})
}

// Fail 输出统一格式的失败响应。
func Fail(c *core.Ctx, msg string, code ...int) {
	bizCode := CodeFailure
	if len(code) > 0 {
		bizCode = code[0]
	}
	c.JSONStatus(200, &Result{Code: bizCode, Msg: msg})
}
