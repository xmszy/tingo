// Package cors 提供 CORS（跨域资源共享）中间件。
//
// 设计：零外部依赖，纯标准库实现；默认开启常用跨域策略，亦可通过
// Config 细粒度控制。
package cors

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xmszy/tingo/core"
)

// Config 是 CORS 配置。
type Config struct {
	// AllowOrigins 允许的来源，* 表示全部。默认 ["*"]。
	AllowOrigins []string
	// AllowMethods 允许的方法，默认 GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS。
	AllowMethods []string
	// AllowHeaders 允许的请求头，默认常用集。
	AllowHeaders []string
	// ExposeHeaders 暴露给浏览器的响应头。
	ExposeHeaders []string
	// AllowCredentials 是否允许携带凭据（cookie 等）。
	AllowCredentials bool
	// MaxAge 预检结果缓存时长。
	MaxAge time.Duration
}

// Default 返回常用默认配置。
func Default() Config {
	return Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

// Middleware 返回 CORS 中间件。
func Middleware(cfg Config) core.Handler {
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = []string{"*"}
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = Default().AllowMethods
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = Default().AllowHeaders
	}
	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := strconv.FormatInt(int64(cfg.MaxAge.Seconds()), 10)

	return func(c *core.Ctx) {
		h := c.G().Writer.Header()
		if cfg.AllowOrigins[0] == "*" {
			if cfg.AllowCredentials {
				// 凭据模式下不能用 *，回退为请求来源。
				h.Set("Access-Control-Allow-Origin", c.Header("Origin"))
			} else {
				h.Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			origin := c.Header("Origin")
			if origin == "" || contains(cfg.AllowOrigins, origin) {
				h.Set("Access-Control-Allow-Origin", origin)
			}
		}
		if cfg.AllowCredentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}
		if allowMethods != "" {
			h.Set("Access-Control-Allow-Methods", allowMethods)
		}
		if allowHeaders != "" {
			h.Set("Access-Control-Allow-Headers", allowHeaders)
		}
		if exposeHeaders != "" {
			h.Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if maxAge != "0" {
			h.Set("Access-Control-Max-Age", maxAge)
		}
		// 预检请求直接终止。
		if c.Method() == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
