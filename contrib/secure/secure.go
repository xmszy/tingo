// Package secure 提供安全相关的响应头中间件。
//
// 设计：零外部依赖，纯标准库实现。
package secure

import (
	"github.com/xmszy/tingo/core"
)

// Config 是安全头配置。
type Config struct {
	// SSLRedirect 强制 HTTPS 跳转。
	SSLRedirect bool
	// SSLHost 跳转目标主机。
	SSLHost string
	// STSSeconds HSTS 最大存活（秒）。
	STSSeconds int
	// STSIncludeSubdomains 是否包含子域。
	STSIncludeSubdomains bool
	// FrameDeny 禁止被 iframe 嵌套（X-Frame-Options: DENY）。
	FrameDeny bool
	// ContentTypeNosniff 禁用 MIME sniffing。
	ContentTypeNosniff bool
	// BrowserXSSFilter 启用浏览器 XSS 防护。
	BrowserXSSFilter bool
	// ReferrerPolicy Referrer 策略。
	ReferrerPolicy string
}

// Default 返回推荐默认配置。
func Default() Config {
	return Config{
		SSLRedirect:           false,
		STSSeconds:            31536000,
		STSIncludeSubdomains:  true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXSSFilter:      true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}
}

// Middleware 返回安全头中间件。
func Middleware(cfg Config) core.Handler {
	return func(c *core.Ctx) {
		h := c.G().Writer.Header()
		if cfg.SSLRedirect && c.Scheme() == "http" {
			host := cfg.SSLHost
			if host == "" {
				host = c.Host()
			}
			c.G().Redirect(301, "https://"+host+c.Path()+queryOf(c))
			c.Abort()
			return
		}
		if cfg.STSSeconds > 0 {
			v := "max-age=" + itoa(cfg.STSSeconds)
			if cfg.STSIncludeSubdomains {
				v += "; includeSubDomains"
			}
			h.Set("Strict-Transport-Security", v)
		}
		if cfg.FrameDeny {
			h.Set("X-Frame-Options", "DENY")
		}
		if cfg.ContentTypeNosniff {
			h.Set("X-Content-Type-Options", "nosniff")
		}
		if cfg.BrowserXSSFilter {
			h.Set("X-XSS-Protection", "1; mode=block")
		}
		if cfg.ReferrerPolicy != "" {
			h.Set("Referrer-Policy", cfg.ReferrerPolicy)
		}
		c.Next()
	}
}

func queryOf(c *core.Ctx) string {
	q := c.RawQuery()
	if q == "" {
		return ""
	}
	return "?" + q
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
