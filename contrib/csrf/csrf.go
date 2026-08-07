// Package csrf 提供表单 CSRF 防护中间件。
//
// 设计：采用「Cookie Double-Submit」方案，自包含零依赖，不强制依赖会话组件。
//   - 中间件为每个访问者签发一个随机令牌并写入 Cookie；
//   - 对「非安全方法」（POST/PUT/DELETE/PATCH）校验请求头（X-CSRF-Token）
//     或表单字段（_csrf）携带的令牌是否与 Cookie 中的令牌一致；
//   - 校验通过即旋转令牌（一次性令牌语义）。
// 业务可在渲染页面时调用 Token(c) 取出当前令牌写入表单隐藏域。
package csrf

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/xmszy/tingo/core"
)

const (
	cookieName = "tingo_csrf"
	headerName = "X-CSRF-Token"
	formField  = "_csrf"
	ginKey     = "_tingo_csrf_token"
)

// Config 是 CSRF 中间件配置。
type Config struct {
	// CookieName 令牌 Cookie 名，默认 tingo_csrf。
	CookieName string
	// Path Cookie 路径。
	Path string
	// MaxAge Cookie 有效期（秒）；默认 3600。
	MaxAge int
	// Secure 仅 HTTPS 传输。
	Secure bool
	// HttpOnly 禁止 JS 读取 Cookie（若为 true，前端需经 X-CSRF-Token 头手动传递，
	// 不能从 document.cookie 读取）。
	HttpOnly bool
	// Exclude 跳过校验的路径前缀（如 API、登录）。
	Exclude []string
	// Error 自定义校验失败响应。
	Error func(c *core.Ctx)
}

func (c *Config) norm() {
	if c.CookieName == "" {
		c.CookieName = cookieName
	}
	if c.Path == "" {
		c.Path = "/"
	}
	if c.MaxAge == 0 {
		c.MaxAge = 3600
	}
}

// genToken 生成随机会话令牌。
func genToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// currentToken 取得当前访问者的有效令牌：优先请求 Cookie，其次本请求上下文缓存，
// 都没有则生成新令牌写入 Cookie 并缓存到上下文（保证单次请求只写一次）。
func currentToken(c *core.Ctx, name string, maxAge int, path string, secure, httpOnly bool) string {
	if v, ok := c.G().Get(ginKey); ok {
		return v.(string)
	}
	if t := c.Cookie(name); t != "" {
		c.G().Set(ginKey, t)
		return t
	}
	t := genToken()
	writeCookie(c, name, t, maxAge, path, secure, httpOnly)
	c.G().Set(ginKey, t)
	return t
}

// Token 返回当前访问者的有效令牌（不存在则生成并写入 Cookie）。
// 供渲染层调用以写入表单隐藏域或页面元信息。
func Token(c *core.Ctx) string {
	return currentToken(c, cookieName, 3600, "/", false, false)
}

// Middleware 返回 CSRF 防护中间件。
func Middleware(cfg Config) core.Handler {
	cfg.norm()
	return func(c *core.Ctx) {
		// 安全方法放行，但确保已下发令牌 Cookie。
		if isSafeMethod(c.Method()) {
			currentToken(c, cfg.CookieName, cfg.MaxAge, cfg.Path, cfg.Secure, cfg.HttpOnly)
			c.Next()
			return
		}
		for _, p := range cfg.Exclude {
			if strings.HasPrefix(c.Path(), p) {
				c.Next()
				return
			}
		}
		expected := c.Cookie(cfg.CookieName)
		if expected == "" {
			fail(c, &cfg)
			return
		}
		got := c.Header(headerName)
		if got == "" {
			got = c.Post(formField)
		}
		if got == "" || !equalToken(expected, got) {
			fail(c, &cfg)
			return
		}
		// 校验通过：旋转令牌（一次性语义）。
		writeCookie(c, cfg.CookieName, genToken(), cfg.MaxAge, cfg.Path, cfg.Secure, cfg.HttpOnly)
		c.Next()
	}
}

func writeCookie(c *core.Ctx, name, value string, maxAge int, path string, secure, httpOnly bool) {
	http.SetCookie(c.Res(), &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func equalToken(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func fail(c *core.Ctx, cfg *Config) {
	if cfg.Error != nil {
		cfg.Error(c)
		return
	}
	c.G().AbortWithStatusJSON(http.StatusForbidden, map[string]any{
		"code":    http.StatusForbidden,
		"message": "CSRF token validation failed",
	})
}
