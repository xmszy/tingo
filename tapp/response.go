package tapp

import (
	"net/http"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
)

/* ------------------------------------------------------------------ */
/* 统一响应结构                                                          */
/* ------------------------------------------------------------------ */

// Result 是统一的 JSON 响应结构。
//
// 字段顺序：code、msg、data。
type Result struct {
	// Code 是业务状态码，0 表示成功。
	Code int `json:"code"`
	// Msg 是提示消息。
	Msg string `json:"msg"`
	// Data 是业务数据，为空时不输出。
	Data any `json:"data,omitempty"`
	// URL 是跳转地址，为空时不输出。
	URL string `json:"url,omitempty"`
}

// 约定的业务状态码：0 表示成功，1 表示失败。
const (
	// CodeSuccess 是成功的业务码。
	CodeSuccess = 0
	// CodeFailure 是失败的默认业务码。
	CodeFailure = 1
)

// Success 输出成功响应。
// 返回 error 恒为 nil，使控制器可统一写成 `return ctrl.Success(c, data)`。
func (ctrl *Controller) Success(c *core.Ctx, data any, msg ...string) error {
	m := "success"
	if len(msg) > 0 {
		m = msg[0]
	}
	c.JSONStatus(http.StatusOK, &Result{Code: CodeSuccess, Msg: m, Data: data})
	return nil
}

// Error 输出失败响应。
// HTTP 状态码仍为 200，失败信息通过业务码表达。
func (ctrl *Controller) Error(c *core.Ctx, msg string, code ...int) error {
	bizCode := CodeFailure
	if len(code) > 0 {
		bizCode = code[0]
	}
	c.JSONStatus(http.StatusOK, &Result{Code: bizCode, Msg: msg})
	return nil
}

// Result 输出自定义的完整响应体。
func (ctrl *Controller) Result(c *core.Ctx, data any, code int, msg string) error {
	c.JSONStatus(http.StatusOK, &Result{Code: code, Msg: msg, Data: data})
	return nil
}

// Redirect 输出重定向。
func (ctrl *Controller) Redirect(c *core.Ctx, url string, code ...int) error {
	status := http.StatusFound
	if len(code) > 0 {
		status = code[0]
	}
	c.Redirect(status, url)
	return nil
}

/* ------------------------------------------------------------------ */
/* 错误构造助手                                                          */
/* ------------------------------------------------------------------ */

// BindError 将绑定失败包装为统一的参数错误。
func BindError(err error) error {
	if err == nil {
		return nil
	}
	return errors.ErrBadRequest.Wrap(err)
}

// ValidationError 构造带字段明细的校验错误。
func ValidationError(fields map[string]string, msg ...string) error {
	e := errors.ErrValidation
	if len(msg) > 0 && msg[0] != "" {
		e = e.WithMessage(msg[0])
	} else if len(fields) > 0 {
		// 以首个字段错误作为主消息。
		for _, v := range fields {
			e = e.WithMessage(v)
			break
		}
	}
	if len(fields) > 0 {
		e = e.WithMeta("fields", fields)
	}
	return e
}
