// Package tcookie 提供 HTTP Cookie 便捷操作。
//
// 基于 net/http.Cookie 包装，提供 Set/Get/Forget/Flash 等便捷的
// Cookie 帮助函数。
package tcookie

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xmszy/tingo/core"
)

// Options Cookie 设置选项。
type Options struct {
	// Path 作用路径（默认 "/"）。
	Path string
	// Domain 作用域。
	Domain string
	// MaxAge 最大存活秒数（0=会话级，-1=删除）。
	MaxAge int
	// Expire 过期时间（与 MaxAge 二选一，Expire 优先级更高）。
	Expire time.Time
	// Secure 仅 HTTPS。
	Secure bool
	// HTTPOnly 禁止 JS 访问。
	HTTPOnly bool
	// SameSite 同站策略。
	SameSite http.SameSite
	// Encode 写入前编码（nil 即不使用）。
	Encode func(string) string
	// Decode 读取后解码（nil 即不使用）。
	Decode func(string) string
}

// New 创建 Cookie 选项实例。
func New(opts ...func(*Options)) *Options {
	o := &Options{Path: "/"}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// ---- 构建选项 ---

// WithPath 设置 Cookie 路径。
func WithPath(p string) func(*Options) { return func(o *Options) { o.Path = p } }

// WithDomain 设置 Cookie 域。
func WithDomain(d string) func(*Options) { return func(o *Options) { o.Domain = d } }

// WithMaxAge 设置 Cookie 最大存活秒数。
func WithMaxAge(s int) func(*Options) { return func(o *Options) { o.MaxAge = s } }

// WithExpire 设置 Cookie 过期时间。
func WithExpire(t time.Time) func(*Options) { return func(o *Options) { o.Expire = t } }

// WithSecure 设置 Secure 标志。
func WithSecure(b bool) func(*Options) { return func(o *Options) { o.Secure = b } }

// WithHTTPOnly 设置 HTTPOnly 标志。
func WithHTTPOnly(b bool) func(*Options) { return func(o *Options) { o.HTTPOnly = b } }

// WithSameSite 设置 SameSite。
func WithSameSite(s http.SameSite) func(*Options) { return func(o *Options) { o.SameSite = s } }

// WithEncode 设置编码器。
func WithEncode(f func(string) string) func(*Options) { return func(o *Options) { o.Encode = f } }

// ---- Cookie 操作 ----

// Set 写入 Cookie。
func Set(c *core.Ctx, name, value string, opts ...func(*Options)) {
	o := New(opts...)
	if o.Encode != nil {
		value = o.Encode(value)
	}
	ck := &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		Path:     o.Path,
		Domain:   o.Domain,
		MaxAge:   o.MaxAge,
		Secure:   o.Secure,
		HttpOnly: o.HTTPOnly,
		SameSite: o.SameSite,
	}
	if !o.Expire.IsZero() {
		ck.Expires = o.Expire
	}
	http.SetCookie(c.Writer, ck)
}

// Get 读取 Cookie 值。
func Get(c *core.Ctx, name string, opts ...func(*Options)) string {
	ck, err := c.Request.Cookie(name)
	if err != nil || ck.Value == "" {
		return ""
	}
	val, err := url.QueryUnescape(ck.Value)
	if err != nil {
		return ck.Value
	}
	o := New(opts...)
	if o.Decode != nil {
		val = o.Decode(val)
	}
	return val
}

// Has 判断 Cookie 是否存在。
func Has(c *core.Ctx, name string) bool {
	_, err := c.Request.Cookie(name)
	return err == nil
}

// Forget 删除 Cookie（MaxAge=-1）。
func Forget(c *core.Ctx, name string, opts ...func(*Options)) {
	o := New(opts...)
	o.MaxAge = -1
	Set(c, name, "", func(opt *Options) {
		*opt = *o
	})
}

// Flash 一次性闪存：读取后立即删除。
func Flash(c *core.Ctx, name string, opts ...func(*Options)) string {
	val := Get(c, name, opts...)
	if val != "" {
		Forget(c, name, opts...)
	}
	return val
}

// DebugInfo 返回 Cookie 调试信息。
func DebugInfo(c *core.Ctx) string {
	info := "Cookies:\n"
	for _, ck := range c.Request.Cookies() {
		val := ck.Value
		if len(val) > 32 {
			val = val[:32] + "..."
		}
		info += fmt.Sprintf("  %s = %s\n", ck.Name, val)
	}
	return info
}
