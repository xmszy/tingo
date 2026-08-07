package core

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/os/tvalid"
)

/* ------------------------------------------------------------------ */
/* 响应输出                                                             */
/* ------------------------------------------------------------------ */

// Status 设置响应状态码。
func (c *Ctx) Status(code int) *Ctx {
	c.G().Status(code)
	return c
}

// SetHeader 设置响应头。
func (c *Ctx) SetHeader(key, val string) *Ctx {
	c.G().Header(key, val)
	return c
}

// SetCookie 设置 cookie。
func (c *Ctx) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) *Ctx {
	c.G().SetCookie(name, value, maxAge, path, domain, secure, httpOnly)
	return c
}

// JSON 输出 JSON，状态码 200。
func (c *Ctx) JSON(v any) { c.G().JSON(http.StatusOK, v) }

// JSONStatus 以指定状态码输出 JSON。
func (c *Ctx) JSONStatus(code int, v any) { c.G().JSON(code, v) }

// String 输出纯文本，状态码 200。
func (c *Ctx) String(format string, values ...any) {
	c.G().String(http.StatusOK, format, values...)
}

// StringStatus 以指定状态码输出纯文本。
func (c *Ctx) StringStatus(code int, format string, values ...any) {
	c.G().String(code, format, values...)
}

// XML 输出 XML，状态码 200。
func (c *Ctx) XML(v any) { c.G().XML(http.StatusOK, v) }

// YAML 输出 YAML，状态码 200。
func (c *Ctx) YAML(v any) { c.G().YAML(http.StatusOK, v) }

// Data 输出原始字节。
func (c *Ctx) Data(code int, contentType string, data []byte) {
	c.G().Data(code, contentType, data)
}

// HTML 渲染模板。
func (c *Ctx) HTML(name string, data any) {
	c.G().HTML(http.StatusOK, name, data)
}

// Redirect 重定向。
func (c *Ctx) Redirect(code int, location string) { c.G().Redirect(code, location) }

// Download 触发文件下载。
func (c *Ctx) Download(filepath, filename string) {
	c.G().FileAttachment(filepath, filename)
}

// ServeFile 直接输出文件内容。
func (c *Ctx) ServeFile(filepath string) { c.G().File(filepath) }

// NoContent 输出 204。
func (c *Ctx) NoContent() { c.G().Status(http.StatusNoContent) }

// Stream 以流式方式写出响应。step 返回 false 时结束。
func (c *Ctx) Stream(step func(w io.Writer) bool) bool {
	return c.G().Stream(step)
}

// SSE 推送一条 Server-Sent Event。
func (c *Ctx) SSE(event string, data any) {
	c.G().SSEvent(event, data)
}

/* ------------------------------------------------------------------ */
/* 参数绑定                                                             */
/* ------------------------------------------------------------------ */

// Bind 根据 Content-Type 自动绑定并校验请求数据。
func (c *Ctx) Bind(obj any) error { return c.G().ShouldBind(obj) }

// BindJSON 绑定 JSON 请求体。
func (c *Ctx) BindJSON(obj any) error { return c.G().ShouldBindJSON(obj) }

// BindQuery 绑定 URL query。
func (c *Ctx) BindQuery(obj any) error { return c.G().ShouldBindQuery(obj) }

// BindURI 绑定路由参数。
func (c *Ctx) BindURI(obj any) error { return c.G().ShouldBindUri(obj) }

// BindHeader 绑定请求头。
func (c *Ctx) BindHeader(obj any) error { return c.G().ShouldBindHeader(obj) }

// BindAllAndValidate 依次绑定 uri、query、body，然后自动校验 valid tag。
//
// 绑定完成后会自动调用 tvalid.CheckStruct(obj) 对 valid tag 进行校验。
// 如果绑定失败返回绑定错误，如果校验失败返回 tvalid.Errors。
//
// 用法：
//
//	type LoginReq struct {
//	    Name string `form:"name" valid:"required|len:3,20" label:"用户名"`
//	    Age  int    `form:"age"  valid:"min:1|max:120"`
//	}
//
//	var req LoginReq
//	if err := c.BindAllAndValidate(&req); err != nil {
//	    // 绑定或校验失败
//	    c.JSON(400, gin.H{"error": err.Error()})
//	    return
//	}
func (c *Ctx) BindAllAndValidate(obj any) error {
	if err := c.BindAll(obj); err != nil {
		return err
	}
	// 自动调用 tvalid 校验 valid tag
	return ValidateStruct(obj)
}

// BindAll 依次绑定 uri、query、body，后者覆盖前者。
//
// 绑定顺序与语义：
//  1. uri   —— 仅在存在路由参数时执行；
//  2. query —— 始终执行。即便 query 为空也不能跳过，
//     因为 `form:"page,default=1"` 这类默认值依赖此步生效；
//  3. body  —— 仅在确有请求体时执行，空体不视为错误。
//
// 校验（binding tag）在最后一步统一触发，
// 以保证前面步骤填入的默认值也参与校验。
func (c *Ctx) BindAll(obj any) error {
	g := (*gin.Context)(c)

	if len(c.Params) > 0 {
		if err := g.ShouldBindUri(obj); err != nil {
			return err
		}
	}

	// 始终绑定 query：这是 default 标签生效的地方。
	if err := g.ShouldBindQuery(obj); err != nil {
		return err
	}

	if !c.hasBody() {
		return nil
	}
	if err := g.ShouldBind(obj); err != nil {
		// 空请求体不算错误，其余错误照常返回。
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

// hasBody 判断请求是否携带可解析的请求体。
func (c *Ctx) hasBody() bool {
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return false
	}
	// ContentLength 为 -1 表示长度未知（如 chunked），仍需尝试解析。
	return c.Request.ContentLength != 0
}

// ──────────────── 自动校验 ────────────────

// ValidateStruct 使用默认 tvalid 校验器校验结构体的 valid tag。
// 如果结构体没有任何 valid tag，则直接返回 nil（跳过校验，零开销）。
// 这是 BindAllAndValidate 的底层实现，也可在自定义绑定场景中独立调用。
func ValidateStruct(obj any) error {
	err := tvalid.CheckStruct(obj)
	if err != nil {
		return err
	}
	return nil
}
