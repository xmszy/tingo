// Package lang 提供国际化中间件。
//
// 核心功能：
//  1. 自动检测用户语言（Cookie > Query > Accept-Language > 默认值）
//  2. 将 locale 注入请求 Context
//  3. 提供 T() 快捷函数翻译消息
//
// 用法：
//
//	lang.SetDefault(translator)        // 注册翻译器
//	engine.Use(lang.Middleware(lang.Config{
//	    DefaultLang: "zh",
//	}))
//	// 在控制器中：
//	msg := lang.T(c, "welcome", params...)
package lang

import (
	"net/http"
	"strings"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tlang"
)

// defaultTranslator 全局默认翻译器（SetDefault 注册）。
var defaultTranslator *tlang.Translator

// SetDefault 注册全局默认翻译器。
// 此函数应在应用初始化时调用一次。
func SetDefault(tr *tlang.Translator) {
	defaultTranslator = tr
}

// Translator 返回全局翻译器。
func Translator() *tlang.Translator {
	return defaultTranslator
}

// Config 语言检测中间件的配置。
type Config struct {
	// CookieName 存储语言偏好的 Cookie 名（默认 "lang"）。
	CookieName string
	// QueryParam URL 查询参数名（默认 "lang"）。
	QueryParam string
	// DefaultLang 当所有检测手段都失败时使用的回退语言。
	DefaultLang string
}

// Middleware 返回自动语言检测中间件。
//
// 检测优先级：Cookie > Query > Accept-Language 头 > DefaultLang
func Middleware(cfg Config) core.Handler {
	if cfg.CookieName == "" {
		cfg.CookieName = "lang"
	}
	if cfg.QueryParam == "" {
		cfg.QueryParam = "lang"
	}
	if cfg.DefaultLang == "" {
		cfg.DefaultLang = "zh"
	}

	return func(c *core.Ctx) {
		locale := detectLocale(c, cfg)
		// 将 locale 注入 request context，tlang.TranslateCtx 可读取
		ctx := tlang.SetLocaleCtx(c.Request.Context(), locale)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// T 翻译消息（使用默认翻译器）。
// c 提供 locale 上下文，key 是消息键，params 用于占位符替换。
func T(c *core.Ctx, key string, params ...any) string {
	if defaultTranslator == nil {
		return key
	}
	return defaultTranslator.TranslateCtx(c.Request.Context(), key, params...)
}

// Locale 从请求上下文获取当前检测到的语言。
func Locale(c *core.Ctx) string {
	if l, ok := tlang.LocaleFromCtx(c.Request.Context()); ok {
		return l
	}
	return ""
}

// SetLocale 动态设置当前请求的语言。
func SetLocale(c *core.Ctx, locale string) {
	ctx := tlang.SetLocaleCtx(c.Request.Context(), locale)
	c.Request = c.Request.WithContext(ctx)
}

// detectLocale 按优先级检测语言：Cookie > Query > Accept-Language > DefaultLang。
func detectLocale(c *core.Ctx, cfg Config) string {
	// 1. Cookie
	if cookie, err := c.Request.Cookie(cfg.CookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// 2. Query 参数
	if val := c.Request.URL.Query().Get(cfg.QueryParam); val != "" {
		return val
	}

	// 3. Accept-Language 头（取首选语言）
	if al := c.Request.Header.Get("Accept-Language"); al != "" {
		return parseAcceptLanguage(al)
	}

	return cfg.DefaultLang
}

// parseAcceptLanguage 从 Accept-Language 头解析首选语言。
// 例如 "zh-CN,zh;q=0.9,en;q=0.8" → "zh"
func parseAcceptLanguage(al string) string {
	// 取第一个逗号前的部分
	if idx := strings.IndexByte(al, ','); idx > 0 {
		al = al[:idx]
	}
	// 去掉权重 ;q=...
	if idx := strings.IndexByte(al, ';'); idx > 0 {
		al = al[:idx]
	}
	al = strings.TrimSpace(al)
	// "zh-CN" → "zh"
	if idx := strings.IndexByte(al, '-'); idx > 0 {
		al = al[:idx]
	}
	return al
}

// Ensure Request 方法可用，兼容 net/http 语义。
var _ http.CookieJar = nil
