package tapp

import (
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
)

/* ------------------------------------------------------------------ */
/* 全局异常处理器                                                        */
/* ------------------------------------------------------------------ */

// Reporter 是异常上报契约。
type Reporter interface {
	// Report 记录一个未被忽略的异常。
	Report(c *core.Ctx, err error)
}

// ReporterFunc 让普通函数可作为 Reporter 使用。
type ReporterFunc func(c *core.Ctx, err error)

// Report 实现 Reporter 接口。
func (f ReporterFunc) Report(c *core.Ctx, err error) { f(c, err) }

// ExceptionHandle 是全局异常处理器。
//
// 职责：
//   - IgnoreReport：不需要记录日志的异常类型清单；
//   - Report：记录异常；
//   - Render：把异常渲染为响应。
//
// 渲染策略：
//   - Ajax / JSON 请求 -> JSON 响应；
//   - Debug 开启 -> HTML 错误详情页；
//   - 否则 -> 简洁的 HTML 错误页。
type ExceptionHandle struct {
	// Debug 为 true 时输出异常详情。
	Debug bool

	// Reporter 负责记录异常，为 nil 时不记录。
	Reporter Reporter

	// IgnoreReport 是无需记录日志的业务错误码清单。
	IgnoreReport []string

	// RenderJSON 强制所有异常都以 JSON 渲染，适用于纯 API 项目。
	RenderJSON bool
}

// NewExceptionHandle 创建一个默认的异常处理器。
func NewExceptionHandle() *ExceptionHandle {
	return &ExceptionHandle{
		// 默认忽略校验失败、鉴权类异常，不写错误日志。
		IgnoreReport: []string{
			errors.CodeValidation,
			errors.CodeUnauthorized,
			errors.CodeForbidden,
			errors.CodeNotFound,
			errors.CodeBadRequest,
			errors.CodeMethodNotAllowed,
		},
	}
}

// shouldReport 判断异常是否需要记录。
func (h *ExceptionHandle) shouldReport(err error) bool {
	code := errors.CodeOf(err)
	for _, ignored := range h.IgnoreReport {
		if ignored == code {
			return false
		}
	}
	return true
}

// Report 记录异常。
func (h *ExceptionHandle) Report(c *core.Ctx, err error) {
	if err == nil || h.Reporter == nil || !h.shouldReport(err) {
		return
	}
	h.Reporter.Report(c, err)
}

// Render 将异常渲染为 HTTP 响应。
func (h *ExceptionHandle) Render(c *core.Ctx, err error) {
	if err == nil {
		return
	}
	e := errors.From(err)
	status := e.Status
	if status <= 0 {
		status = http.StatusInternalServerError
	}

	if h.RenderJSON || wantsJSON(c) {
		h.renderJSON(c, e, status)
		return
	}
	h.renderHTML(c, e, status)
}

// Handle 是 Report + Render 的组合，供中间件直接调用。
func (h *ExceptionHandle) Handle(c *core.Ctx, err error) {
	h.Report(c, err)
	h.Render(c, err)
}

/* ------------------------------------------------------------------ */
/* 接入框架统一响应协议                                                   */
/* ------------------------------------------------------------------ */

// 确保异常处理器可直接充当框架的 Responder，
// 这样所有 handler 返回的 error 都会汇入集中异常处理。
var _ core.Responder = (*ExceptionHandle)(nil)

// Reply 实现 core.Responder：成功时输出统一的 Result 结构。
func (h *ExceptionHandle) Reply(c *core.Ctx, data any) {
	c.JSONStatus(http.StatusOK, &Result{Code: CodeSuccess, Msg: "success", Data: data})
}

// Fail 实现 core.Responder：失败时走 report + render 全流程。
func (h *ExceptionHandle) Fail(c *core.Ctx, err error) { h.Handle(c, err) }

// Register 将该异常处理器设为框架全局响应协议。
func (h *ExceptionHandle) Register() { core.SetResponder(h) }

/* ------------------------------------------------------------------ */
/* 渲染实现                                                              */
/* ------------------------------------------------------------------ */

// errorBody 是 JSON 错误响应体，与 Result 结构保持一致的字段习惯。
type errorBody struct {
	Code    string         `json:"code"`
	Msg     string         `json:"msg"`
	Meta    map[string]any `json:"meta,omitempty"`
	Detail  string         `json:"detail,omitempty"`
	Request string         `json:"request_id,omitempty"`
}

func (h *ExceptionHandle) renderJSON(c *core.Ctx, e *errors.Error, status int) {
	body := &errorBody{
		Code:    e.Code,
		Msg:     e.Message,
		Meta:    e.Meta,
		Request: c.RequestID(),
	}
	// 仅在 debug 下暴露底层错误链，避免生产环境泄露内部细节。
	if h.Debug {
		if cause := e.Unwrap(); cause != nil {
			body.Detail = cause.Error()
		}
	}
	c.JSONStatus(status, body)
}

func (h *ExceptionHandle) renderHTML(c *core.Ctx, e *errors.Error, status int) {
	var b strings.Builder
	b.Grow(512)
	b.WriteString(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	b.WriteString(`<title>`)
	b.WriteString(strconv.Itoa(status))
	b.WriteString(` </title><style>`)
	b.WriteString(`body{margin:0;padding:0;background:#f7f8fa;color:#24292f;` +
		`font:14px/1.7 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif}`)
	b.WriteString(`.wrap{max-width:820px;margin:12vh auto;padding:0 24px}`)
	b.WriteString(`.card{background:#fff;border:1px solid #e5e7eb;border-radius:12px;padding:32px 36px;` +
		`box-shadow:0 1px 3px rgba(0,0,0,.06)}`)
	b.WriteString(`.status{font-size:56px;font-weight:700;color:#d1242f;line-height:1;margin:0 0 12px}`)
	b.WriteString(`.msg{font-size:18px;margin:0 0 20px}`)
	b.WriteString(`.code{display:inline-block;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;` +
		`font-size:12px;background:#f6f8fa;border:1px solid #e5e7eb;border-radius:6px;padding:2px 8px;color:#57606a}`)
	b.WriteString(`pre{background:#f6f8fa;border:1px solid #e5e7eb;border-radius:8px;padding:16px;` +
		`overflow:auto;font-size:12.5px;color:#57606a;margin:20px 0 0}`)
	b.WriteString(`.foot{margin-top:24px;color:#8b949e;font-size:12px}`)
	b.WriteString(`</style></head><body><div class="wrap"><div class="card">`)

	b.WriteString(`<p class="status">`)
	b.WriteString(strconv.Itoa(status))
	b.WriteString(`</p>`)

	b.WriteString(`<p class="msg">`)
	b.WriteString(html.EscapeString(e.Message))
	b.WriteString(`</p>`)

	if e.Code != "" {
		b.WriteString(`<span class="code">`)
		b.WriteString(html.EscapeString(e.Code))
		b.WriteString(`</span>`)
	}

	if h.Debug {
		if cause := e.Unwrap(); cause != nil {
			b.WriteString(`<pre>`)
			b.WriteString(html.EscapeString(cause.Error()))
			b.WriteString(`</pre>`)
		}
		b.WriteString(`<p class="foot">`)
		b.WriteString(html.EscapeString(c.Method()))
		b.WriteString(` `)
		b.WriteString(html.EscapeString(c.Path()))
		b.WriteString(`</p>`)
	}

	b.WriteString(`</div></div></body></html>`)
	c.Data(status, "text/html; charset=utf-8", []byte(b.String()))
}

// wantsJSON 判断客户端是否期望 JSON 响应。
func wantsJSON(c *core.Ctx) bool {
	if c.IsAjax() {
		return true
	}
	if strings.Contains(c.ContentType(), "json") {
		return true
	}
	accept := c.Header("Accept")
	// Accept 中 json 出现在 html 之前即视为偏好 JSON。
	ji := strings.Index(accept, "json")
	if ji < 0 {
		return false
	}
	hi := strings.Index(accept, "html")
	return hi < 0 || ji < hi
}
