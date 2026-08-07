// Package core 提供 tingo 的运行时内核。
//
// 设计原则：
//  1. 零成本抽象 —— 类型定义零拷贝，指针转换是编译期 no-op。
//  2. 零额外分配 —— 不在请求路径上包装、不逃逸、不反射。
//  3. 注册期展开 —— 多应用/中间件在启动时全部展开进路由树。
package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrNotHijackable 底层 ResponseWriter 不支持连接接管。
var ErrNotHijackable = errors.New("tingo: response writer does not support hijacking")

// Ctx 是 tingo 的请求上下文。
//
// 通过类型定义（而非结构体嵌入）实现零拷贝，指针转换不产生任何堆分配，
// 也不增加一层间接寻址。
//
// 注意：Ctx 只能通过转换获得，不要直接 new(Ctx)。
type Ctx gin.Context

// G 返回底层上下文对象，零成本。
//
// 当需要使用 tingo 尚未封装的原生能力时使用。
//
//go:inline
func (c *Ctx) G() *gin.Context { return (*gin.Context)(c) }

// FromGin 将 *gin.Context 转为 *Ctx。零成本。
//
//go:inline
func FromGin(c *gin.Context) *Ctx { return (*Ctx)(c) }

// ToGin 将 *Ctx 转为 *gin.Context。零成本。
//
//go:inline
func ToGin(c *Ctx) *gin.Context { return (*gin.Context)(c) }

/* ------------------------------------------------------------------ */
/* 请求基础信息                                                          */
/* ------------------------------------------------------------------ */

// Req 返回原始 *http.Request。
func (c *Ctx) Req() *http.Request { return c.Request }

// Res 返回响应写入器。
func (c *Ctx) Res() gin.ResponseWriter { return c.Writer }

// Method 返回请求方法，如 GET、POST。
func (c *Ctx) Method() string { return c.Request.Method }

// Path 返回请求路径（不含 query）。
func (c *Ctx) Path() string { return c.Request.URL.Path }

// FullPath 返回匹配到的路由模式，如 /user/:id。未匹配时返回空串。
func (c *Ctx) FullPath() string { return c.G().FullPath() }

// Host 返回请求 Host。
func (c *Ctx) Host() string { return c.Request.Host }

// URL 返回请求 URL。
func (c *Ctx) URL() *url.URL { return c.Request.URL }

// Scheme 返回请求协议 http 或 https。
func (c *Ctx) Scheme() string {
	if c.Request.TLS != nil {
		return "https"
	}
	if s := c.Request.Header.Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	return "http"
}

// IsAjax 判断是否为 XMLHttpRequest 请求。
func (c *Ctx) IsAjax() bool {
	return c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// IP 返回客户端 IP（经可信代理链解析）。
func (c *Ctx) IP() string { return c.G().ClientIP() }

// RemoteIP 返回直连对端 IP。
func (c *Ctx) RemoteIP() string { return c.G().RemoteIP() }

// UserAgent 返回 User-Agent 头。
func (c *Ctx) UserAgent() string { return c.Request.UserAgent() }

// ContentType 返回请求的 Content-Type（已剥离参数）。
func (c *Ctx) ContentType() string { return c.G().ContentType() }

// Referer 返回 Referer 头。
func (c *Ctx) Referer() string { return c.Request.Referer() }

/* ------------------------------------------------------------------ */
/* 路由参数 / 查询参数 / 表单                                             */
/* ------------------------------------------------------------------ */

// Param 返回路由参数，如 /user/:id 中的 id。
//
//go:inline
func (c *Ctx) Param(key string) string { return c.Params.ByName(key) }

// Query 返回 URL query 参数。
//
//go:inline
func (c *Ctx) Query(key string) string { return c.G().Query(key) }

// DefaultQuery 返回 URL query 参数，不存在时返回默认值。
func (c *Ctx) DefaultQuery(key, def string) string { return c.G().DefaultQuery(key, def) }

// QueryArray 返回同名的多个 query 参数。
func (c *Ctx) QueryArray(key string) []string { return c.G().QueryArray(key) }

// QueryMap 返回形如 key[sub]=v 的 query 参数集合。
func (c *Ctx) QueryMap(key string) map[string]string { return c.G().QueryMap(key) }

// Post 返回 POST 表单字段。
func (c *Ctx) Post(key string) string { return c.G().PostForm(key) }

// DefaultPost 返回 POST 表单字段，不存在时返回默认值。
func (c *Ctx) DefaultPost(key, def string) string { return c.G().DefaultPostForm(key, def) }

// PostArray 返回同名的多个 POST 表单字段。
func (c *Ctx) PostArray(key string) []string { return c.G().PostFormArray(key) }

// PostMap 返回形如 key[sub]=v 的 POST 表单集合。
func (c *Ctx) PostMap(key string) map[string]string { return c.G().PostFormMap(key) }

// Header 返回请求头。
func (c *Ctx) Header(key string) string { return c.Request.Header.Get(key) }

// Cookie 返回 cookie 值，不存在时返回空串。
func (c *Ctx) Cookie(name string) string {
	v, err := c.Request.Cookie(name)
	if err != nil {
		return ""
	}
	s, _ := url.QueryUnescape(v.Value)
	return s
}

// File 返回上传的单个文件。
func (c *Ctx) File(name string) (*multipart.FileHeader, error) {
	return c.G().FormFile(name)
}

// Files 返回上传的多个同名文件。
func (c *Ctx) Files(name string) ([]*multipart.FileHeader, error) {
	form, err := c.G().MultipartForm()
	if err != nil {
		return nil, err
	}
	return form.File[name], nil
}

// SaveFile 保存上传文件到指定路径。
func (c *Ctx) SaveFile(f *multipart.FileHeader, dst string) error {
	return c.G().SaveUploadedFile(f, dst)
}

// Body 读取并返回请求体原始字节。可重复调用：首次读取后会将内容
// 还原回 Request.Body，因此后续读取（包括内部的 Bind/ShouldBind）
// 都能拿到完整数据，不会返回空。
func (c *Ctx) Body() ([]byte, error) {
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}

// RawQuery 返回原始 query 字符串。
func (c *Ctx) RawQuery() string { return c.Request.URL.RawQuery }

/* ------------------------------------------------------------------ */
/* 流程控制                                                             */
/* ------------------------------------------------------------------ */

// Next 执行链中后续的 handler。仅在中间件中使用。
//
//go:inline
func (c *Ctx) Next() { c.G().Next() }

// Abort 终止后续 handler 的执行。
//
//go:inline
func (c *Ctx) Abort() { c.G().Abort() }

// IsAborted 返回当前链是否已被终止。
func (c *Ctx) IsAborted() bool { return c.G().IsAborted() }

// AbortWithStatus 终止链并写出状态码。
func (c *Ctx) AbortWithStatus(code int) { c.G().AbortWithStatus(code) }

/* ------------------------------------------------------------------ */
/* context.Context 实现                                                 */
/* ------------------------------------------------------------------ */

// Deadline 实现 context.Context。
func (c *Ctx) Deadline() (time.Time, bool) { return c.G().Deadline() }

// Done 实现 context.Context。
func (c *Ctx) Done() <-chan struct{} { return c.G().Done() }

// Err 实现 context.Context。
func (c *Ctx) Err() error { return c.G().Err() }

// Value 实现 context.Context。
func (c *Ctx) Value(key any) any { return c.G().Value(key) }

var _ context.Context = (*Ctx)(nil)

/* ------------------------------------------------------------------ */
/* 网络工具                                                             */
/* ------------------------------------------------------------------ */

// Hijack 接管底层连接，用于 WebSocket 等场景。
func (c *Ctx) Hijack() (net.Conn, error) {
	h, ok := c.Writer.(http.Hijacker)
	if !ok {
		return nil, ErrNotHijackable
	}
	conn, _, err := h.Hijack()
	return conn, err
}
