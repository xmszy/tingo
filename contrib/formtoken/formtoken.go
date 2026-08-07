// Package formtoken 提供表单令牌中间件，用于防重复提交。
//
// 设计：基于会话的表单令牌校验。
//   - GET 请求：生成一次性令牌，注入 request context，供模板表单渲染使用
//   - POST/PUT/DELETE 请求：校验请求中的令牌与会话中存储的令牌是否一致，
//     校验通过后立即使令牌失效（一次性语义，防止重复提交）
//   - 令牌存储在 request context 中（经 contrib/sessions 中间件->core.Ctx），
//     同时写入 Cache-Control 头禁止浏览器缓存令牌页面
//
// 用法：
//
//	engine.Use(formtoken.Middleware(formtoken.Config{}))
//
// 模板中：
//
//	<input type="hidden" name="__token__" value="{{ .token }}">
package formtoken

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/xmszy/tingo/core"
)

const (
	defaultField  = "__token__"
	defaultHeader = "X-Form-Token"
	ctxKey        = "_tingo_form_token"
)

// Config 表单令牌中间件配置。
type Config struct {
	// Field 表单字段名，默认 __token__。
	Field string
	// Header 请求头名，默认 X-Form-Token。
	Header string
	// Exclude 跳过校验的路径前缀。
	Exclude []string
	// Error 自定义校验失败响应。
	Error func(c *core.Ctx)
	// Store 令牌存储接口；nil 默认使用 context 内存存储。
	Store TokenStore
}

func (c *Config) norm() {
	if c.Field == "" {
		c.Field = defaultField
	}
	if c.Header == "" {
		c.Header = defaultHeader
	}
	if c.Store == nil {
		c.Store = &ctxStore{}
	}
}

// TokenStore 令牌存储接口。
type TokenStore interface {
	// Get 获取指定 key 的令牌（需返回是否有效）。
	Get(c *core.Ctx, key string) (token string, ok bool)
	// Set 存储令牌（覆盖旧值）。ttl 为过期时间。
	Set(c *core.Ctx, key string, token string, ttl time.Duration)
	// Delete 删除令牌（校验通过后失效）。
	Delete(c *core.Ctx, key string)
}

// ctxStore 默认实现：令牌存于 *core.Ctx（用 G().Set/Get）。
type ctxStore struct{}

func (s *ctxStore) Get(c *core.Ctx, key string) (string, bool) {
	v, ok := c.G().Get(key)
	if !ok {
		return "", false
	}
	t, ok := v.(string)
	return t, ok
}

func (s *ctxStore) Set(c *core.Ctx, key string, token string, _ time.Duration) {
	c.G().Set(key, token)
}

func (s *ctxStore) Delete(c *core.Ctx, key string) {
	c.G().Set(key, nil)
}

// ------------------- 中间件 -------------------

// Middleware 返回表单令牌中间件。
func Middleware(cfg Config) core.Handler {
	cfg.norm()
	return func(c *core.Ctx) {
		if isSafeMethod(c.Method()) {
			// 生成新令牌写入 context
			token := genFormToken()
			cfg.Store.Set(c, ctxKey, token, 0)
			c.G().Set("_form_token_value", token)
			c.Res().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			c.Next()
			return
		}

		// 检查排除列表
		for _, p := range cfg.Exclude {
			if len(p) > 0 && len(c.Path()) >= len(p) && c.Path()[:len(p)] == p {
				c.Next()
				return
			}
		}

		// 获取客户端提交的令牌
		got := c.Header(cfg.Header)
		if got == "" {
			got = c.Post(cfg.Field)
		}
		if got == "" {
			failFormToken(c, &cfg, "token is empty")
			return
		}

		// 获取服务端存储的令牌
		expected, ok := cfg.Store.Get(c, ctxKey)
		if !ok || expected == "" {
			failFormToken(c, &cfg, "token not found or expired")
			return
		}

		if !secureCompare(got, expected) {
			failFormToken(c, &cfg, "token mismatch")
			return
		}

		// 令牌使用后立即失效
		cfg.Store.Delete(c, ctxKey)
		c.Next()
	}
}

// Token 获取当前请求的表单令牌（供模板渲染使用）。
// 仅在 GET 请求经过 Middleware 后可用，否则返回空字符串。
func Token(c *core.Ctx) string {
	v, ok := c.G().Get("_form_token_value")
	if !ok {
		return ""
	}
	t, _ := v.(string)
	return t
}

// ------------------- helpers -------------------

func genFormToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func failFormToken(c *core.Ctx, cfg *Config, msg string) {
	if cfg.Error != nil {
		cfg.Error(c)
		return
	}
	c.G().AbortWithStatusJSON(http.StatusForbidden, map[string]any{
		"code":    http.StatusForbidden,
		"message": "Form token validation failed: " + msg,
	})
}

// 确保 errors 被引用（用于未来可能的扩展）
var _ = errors.New
