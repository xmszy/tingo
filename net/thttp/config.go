package thttp

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/os/tenv"
)

// Config 是 HTTP 服务配置。
type Config struct {
	// Addr 是监听地址，如 :8080。
	Addr string

	// ReadTimeout 是读取整个请求的超时。
	ReadTimeout time.Duration
	// ReadHeaderTimeout 是读取请求头的超时。
	ReadHeaderTimeout time.Duration
	// WriteTimeout 是写响应的超时。
	WriteTimeout time.Duration
	// IdleTimeout 是 keep-alive 空闲超时。
	IdleTimeout time.Duration
	// ShutdownTimeout 是优雅关闭的最长等待时间。
	ShutdownTimeout time.Duration

	// MaxHeaderBytes 是请求头的最大字节数。
	MaxHeaderBytes int
	// MaxMultipartMemory 是 multipart 表单在内存中的最大字节数。
	MaxMultipartMemory int64
	// MaxBody 限制请求体大小（字节），0 表示不限制。
	// 防止恶意大 POST 打满连接/内存——超限时 c.Request.Body 读取返回
	// http.MaxBytesReader 对应的 ErrStatusRequestEntityTooLarge，由 Recover 中间件兜底。
	MaxBody int64
	// Version 是 API 版本前缀（如 "v1"）。非空时所有路由自动挂载于 /{version} 下，
	// 例如 Version="v1" 时 /user/list 变为 /v1/user/list。便于无侵入地做 API 版本化。
	Version string

	// CertFile 与 KeyFile 非空时启用 HTTPS。
	CertFile string
	KeyFile  string

	// TrustedProxies 是可信代理列表，影响 ClientIP 的解析。
	TrustedProxies []string

	// RedirectTrailingSlash 允许自动补全/去除结尾斜杠后重定向。
	RedirectTrailingSlash bool
	// RedirectFixedPath 允许自动修正路径大小写后重定向。
	RedirectFixedPath bool
	// HandleMethodNotAllowed 开启后未匹配方法返回 405 而非 404。
	HandleMethodNotAllowed bool

	// BindRouteMeta 决定是否在上下文中绑定应用/控制器/方法名。
	// 关闭可减少一层闭包调用，用于极致性能场景。
	BindRouteMeta bool

	// PrintRoutes 启动时打印完整路由表。
	PrintRoutes bool
}

// defaultConfig 返回默认配置。
func defaultConfig() Config {
	return Config{
		Addr:                   ":8080",
		ReadTimeout:            60 * time.Second,
		ReadHeaderTimeout:      20 * time.Second,
		WriteTimeout:           60 * time.Second,
		IdleTimeout:            120 * time.Second,
		ShutdownTimeout:        10 * time.Second,
		MaxHeaderBytes:         1 << 20,  // 1 MiB
		MaxMultipartMemory:     32 << 20, // 32 MiB
		MaxBody:                0,        // 0 = 不限制
		RedirectTrailingSlash:  true,
		HandleMethodNotAllowed: true,
		BindRouteMeta:          true,
	}
}

// ginModeFromEnv 由 APP_DEBUG 决定 gin 运行模式：
// true 对应 gin 的调试模式，否则（含未设置）对应生产模式。
func ginModeFromEnv() string {
	if tenv.Get("APP_DEBUG", false) {
		return gin.DebugMode
	}
	return gin.ReleaseMode
}

/* ------------------------------------------------------------------ */
/* Options 函数式配置                                                   */
/* ------------------------------------------------------------------ */

// Option 是配置修改函数。
type Option func(*Config)

// Addr 设置监听地址。
func Addr(addr string) Option { return func(c *Config) { c.Addr = addr } }

// ReadTimeout 设置读超时。
func ReadTimeout(d time.Duration) Option { return func(c *Config) { c.ReadTimeout = d } }

// WriteTimeout 设置写超时。
func WriteTimeout(d time.Duration) Option { return func(c *Config) { c.WriteTimeout = d } }

// IdleTimeout 设置空闲超时。
func IdleTimeout(d time.Duration) Option { return func(c *Config) { c.IdleTimeout = d } }

// ShutdownTimeout 设置优雅关闭等待时间。
func ShutdownTimeout(d time.Duration) Option {
	return func(c *Config) { c.ShutdownTimeout = d }
}

// TLS 启用 HTTPS。
func TLS(certFile, keyFile string) Option {
	return func(c *Config) { c.CertFile, c.KeyFile = certFile, keyFile }
}

// TrustedProxies 设置可信代理。
func TrustedProxies(proxies ...string) Option {
	return func(c *Config) { c.TrustedProxies = proxies }
}

// MaxMultipartMemory 设置 multipart 内存上限。
func MaxMultipartMemory(n int64) Option {
	return func(c *Config) { c.MaxMultipartMemory = n }
}

// PrintRoutes 启动时打印路由表。
func PrintRoutes() Option { return func(c *Config) { c.PrintRoutes = true } }

// DisableRouteMeta 关闭路由元信息绑定以换取极致性能。
// 关闭后 Ctx.App()/Controller()/Action() 将返回空串。
func DisableRouteMeta() Option { return func(c *Config) { c.BindRouteMeta = false } }

// WithConfig 直接使用完整配置。
func WithConfig(cfg Config) Option { return func(c *Config) { *c = cfg } }
