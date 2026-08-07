package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xmszy/tingo/core"
)

// CORSConfig 是跨域中间件的配置。
type CORSConfig struct {
	// AllowOrigins 是允许的来源列表。含 "*" 表示允许全部。
	AllowOrigins []string
	// AllowMethods 是允许的方法列表。
	AllowMethods []string
	// AllowHeaders 是允许的请求头列表。
	AllowHeaders []string
	// ExposeHeaders 是允许客户端读取的响应头列表。
	ExposeHeaders []string
	// AllowCredentials 决定是否允许携带凭证。
	// 注意：为 true 时 AllowOrigins 不能是 "*"，否则浏览器会拒绝。
	AllowCredentials bool
	// MaxAge 是预检请求的缓存时长。
	MaxAge time.Duration
}

// DefaultCORSConfig 返回宽松的默认跨域配置，适用于开发环境。
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions,
		},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		MaxAge:       12 * time.Hour,
	}
}

// CORS 返回跨域中间件。
//
// 所有静态响应头在注册期预先拼接完成，运行时仅做写头操作。
func CORS(opts ...func(*CORSConfig)) core.Handler {
	cfg := DefaultCORSConfig()
	for _, o := range opts {
		o(&cfg)
	}

	// 注册期预计算，避免每请求重复 Join。
	var (
		allowAll      = false
		allowedOrigin = make(map[string]struct{}, len(cfg.AllowOrigins))
		methods       = strings.Join(cfg.AllowMethods, ", ")
		headers       = strings.Join(cfg.AllowHeaders, ", ")
		exposed       = strings.Join(cfg.ExposeHeaders, ", ")
		maxAge        = strconv.Itoa(int(cfg.MaxAge.Seconds()))
		credentials   = cfg.AllowCredentials
	)
	for _, o := range cfg.AllowOrigins {
		if o == "*" {
			allowAll = true
			break
		}
		allowedOrigin[o] = struct{}{}
	}

	return func(c *core.Ctx) {
		origin := c.Header("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// 决定回显的 Origin。启用凭证时必须回显具体来源而非 "*"。
		switch {
		case allowAll && !credentials:
			c.SetHeader("Access-Control-Allow-Origin", "*")
		case allowAll:
			c.SetHeader("Access-Control-Allow-Origin", origin)
			c.SetHeader("Vary", "Origin")
		default:
			if _, ok := allowedOrigin[origin]; !ok {
				// 来源不被允许：不加 CORS 头，由浏览器拦截。
				c.Next()
				return
			}
			c.SetHeader("Access-Control-Allow-Origin", origin)
			c.SetHeader("Vary", "Origin")
		}

		if credentials {
			c.SetHeader("Access-Control-Allow-Credentials", "true")
		}
		if exposed != "" {
			c.SetHeader("Access-Control-Expose-Headers", exposed)
		}

		// 预检请求直接结束，不进入业务 handler。
		if c.Method() == http.MethodOptions {
			c.SetHeader("Access-Control-Allow-Methods", methods)
			c.SetHeader("Access-Control-Allow-Headers", headers)
			c.SetHeader("Access-Control-Max-Age", maxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
