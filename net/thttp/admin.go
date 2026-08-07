// Package thttp 的内置管理端点（Admin / pprof）。
package thttp

import (
	"fmt"
	"net/http"
	httppprof "net/http/pprof"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/os/tcfg"
)

// AdminConfig 管理端点配置。
type AdminConfig struct {
	// EnablePprof 是否启用 pprof（默认 true）。
	EnablePprof bool
	// EnableStatus 是否启用状态页（默认 true）。
	EnableStatus bool
	// BasicAuth 可选的 HTTP Basic Auth 认证。空 map 表示不需要认证。
	BasicAuth map[string]string
	// PathPrefix 管理端点路径前缀（默认 "/__admin"）。
	PathPrefix string
}

// DefaultAdminConfig 返回默认管理端点配置。
func DefaultAdminConfig() *AdminConfig {
	return &AdminConfig{
		// Status 状态页（含 admin 首页）与 pprof 默认开启，EnableAdmin() 即代表启用完整管理端点。
		// 注意：pprof 会暴露运行时 profile 数据，生产环境建议通过 AdminFromTree 显式关闭。
		EnablePprof: true,
		EnableStatus: true,
		PathPrefix:   "/__admin",
	}
}

// AdminOption 是 AdminConfig 的建造选项。
type AdminOption func(*AdminConfig)

// AdminEnablePprof 启用 pprof。
func AdminEnablePprof(enable bool) AdminOption {
	return func(c *AdminConfig) { c.EnablePprof = enable }
}

// AdminEnableStatus 启用状态页。
func AdminEnableStatus(enable bool) AdminOption {
	return func(c *AdminConfig) { c.EnableStatus = enable }
}

// AdminPathPrefix 设置路径前缀。
func AdminPathPrefix(prefix string) AdminOption {
	return func(c *AdminConfig) { c.PathPrefix = prefix }
}

// AdminBasicAuth 设置 BasicAuth 用户/密码。
func AdminBasicAuth(users map[string]string) AdminOption {
	return func(c *AdminConfig) { c.BasicAuth = users }
}

// AdminConfigFromTree 从配置树读取管理端点配置。
// 约定路径：admin.enabled / admin.path_prefix / admin.enable_pprof /
// admin.enable_status / admin.basic_auth。
func AdminConfigFromTree(tree tcfg.Reader) *AdminConfig {
	cfg := DefaultAdminConfig()
	if tree.Has("admin.path_prefix") {
		cfg.PathPrefix = tree.String("admin.path_prefix")
	}
	if tree.Has("admin.enable_pprof") {
		cfg.EnablePprof = tree.Bool("admin.enable_pprof")
	}
	if tree.Has("admin.enable_status") {
		cfg.EnableStatus = tree.Bool("admin.enable_status")
	}
	if tree.Has("admin.basic_auth") {
		if auth, ok := tree.Lookup("admin.basic_auth"); ok {
			if authMap, ok := auth.(map[string]any); ok {
				cfg.BasicAuth = make(map[string]string, len(authMap))
				for k, v := range authMap {
					cfg.BasicAuth[k] = fmt.Sprint(v)
				}
			}
		}
	}
	return cfg
}

// ConfigureAdminFromTree 是框架内部的 ConfigureAtBoot 回调：
// 仅当配置文件中 admin.enabled = true 时才启用管理端点。
func (e *Engine) ConfigureAdminFromTree(tree tcfg.Reader) error {
	if !tree.Bool("admin.enabled") {
		return nil
	}
	cfg := AdminConfigFromTree(tree)
	opts := []AdminOption{
		AdminPathPrefix(cfg.PathPrefix),
		AdminEnablePprof(cfg.EnablePprof),
		AdminEnableStatus(cfg.EnableStatus),
	}
	if len(cfg.BasicAuth) > 0 {
		opts = append(opts, AdminBasicAuth(cfg.BasicAuth))
	}
	e.EnableAdmin(opts...)
	return nil
}

// EnableAdmin 在 Engine 上启用管理端点。
//
// 用法：
//
//	engine.EnableAdmin(thttp.AdminPathPrefix("/__admin"), thttp.AdminEnablePprof(true))
func (e *Engine) EnableAdmin(opts ...AdminOption) {
	if e.adminConfigured {
		return
	}
	e.adminConfigured = true
	cfg := DefaultAdminConfig()
	for _, o := range opts {
		o(cfg)
	}

	prefix := cfg.PathPrefix
	if prefix == "" {
		prefix = "/__admin"
	}

	// 使用独立 gin.Engine 承载 admin 路由，避免在主引擎的 radix tree 上与自动路由产生路径节点冲突。
	adminEngine := gin.New()

	// BasicAuth（全局应用于 adminEngine）
	if len(cfg.BasicAuth) > 0 {
		adminEngine.Use(func(c *gin.Context) {
			user, pass, ok := c.Request.BasicAuth()
			if !ok {
				c.Header("WWW-Authenticate", `Basic realm="Admin"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			expectedPass, exists := cfg.BasicAuth[user]
			if !exists || expectedPass != pass {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		})
	}

	// Admin 首页
	if cfg.EnableStatus {
		adminEngine.GET("/", func(c *gin.Context) {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, adminStatusHTML)
		})
	}

	// 状态端点
	if cfg.EnableStatus {
		adminEngine.GET("/status", statusHandler)
		adminEngine.GET("/health", healthHandler)
		adminEngine.GET("/info", infoHandler)
		adminEngine.GET("/routes", routesHandler)
	}

	// pprof 端点
	if cfg.EnablePprof {
		adminEngine.GET("/debug/pprof/", gin.WrapH(http.HandlerFunc(httppprof.Index)))
		adminEngine.GET("/debug/pprof/cmdline", gin.WrapH(http.HandlerFunc(httppprof.Cmdline)))
		adminEngine.GET("/debug/pprof/profile", gin.WrapH(http.HandlerFunc(httppprof.Profile)))
		adminEngine.POST("/debug/pprof/symbol", gin.WrapH(http.HandlerFunc(httppprof.Symbol)))
		adminEngine.GET("/debug/pprof/symbol", gin.WrapH(http.HandlerFunc(httppprof.Symbol)))
		adminEngine.GET("/debug/pprof/trace", gin.WrapH(http.HandlerFunc(httppprof.Trace)))
		adminEngine.GET("/debug/pprof/allocs", gin.WrapH(httppprof.Handler("allocs")))
		adminEngine.GET("/debug/pprof/block", gin.WrapH(httppprof.Handler("block")))
		adminEngine.GET("/debug/pprof/goroutine", gin.WrapH(httppprof.Handler("goroutine")))
		adminEngine.GET("/debug/pprof/heap", gin.WrapH(httppprof.Handler("heap")))
		adminEngine.GET("/debug/pprof/mutex", gin.WrapH(httppprof.Handler("mutex")))
		adminEngine.GET("/debug/pprof/threadcreate", gin.WrapH(httppprof.Handler("threadcreate")))
	}

	// 在引擎顶部插入拦截中间件：匹配 admin 前缀的请求直接委托给 adminEngine 处理，
	// 绕过主引擎的 radix tree，从而与自动路由完全隔离。
	e.gin.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, prefix) {
			c.Next()
			return
		}
		// 剥离前缀，让 adminEngine 的路由表精确匹配
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, prefix)
		if c.Request.URL.Path == "" {
			c.Request.URL.Path = "/"
		}
		// 在委托前将状态码重置为 200。
		// 因为 admin 请求在主引擎中通常无匹配路由，经由 serveError 时
		// c.writermem.status 已被标记为 404。而某些 handler（如 pprof）
		// 不显式调用 WriteHeader，依赖默认 200。若不重置，这些 handler
		// 写 body 时 WriteHeaderNow 会把 404 写到 wire，导致状态码错误。
		c.Status(http.StatusOK)
		adminEngine.HandleContext(c)
		c.Abort()
	})
}

func statusHandler(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, adminStatusHTML)
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func infoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"framework": "tingo",
	})
}

func routesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "listed via GET /__admin/routes",
	})
}

const adminStatusHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Tingo Admin</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 40px; background: #f5f5f5; }
h1 { color: #333; border-bottom: 2px solid #409eff; padding-bottom: 10px; }
.card { background: white; border-radius: 8px; padding: 20px; margin: 10px 0; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
a { color: #409eff; text-decoration: none; }
a:hover { text-decoration: underline; }
</style>
</head>
<body>
<h1>Tingo Admin</h1>
<div class="card">
  <h3>Server Status</h3>
  <p><strong>Status:</strong> Running</p>
  <p><strong>Framework:</strong> tingo</p>
</div>
<div class="card">
  <h3>Debug</h3>
  <p><a href="/__admin/debug/pprof/">pprof</a></p>
  <p><a href="/__admin/health">Health Check</a></p>
  <p><a href="/__admin/routes">Routes</a></p>
</div>
<div class="card">
  <h3>pprof Profiles</h3>
  <p><a href="/__admin/debug/pprof/goroutine?debug=1">Goroutines</a></p>
  <p><a href="/__admin/debug/pprof/heap?debug=1">Heap</a></p>
  <p><a href="/__admin/debug/pprof/allocs?debug=1">Allocations</a></p>
  <p><a href="/__admin/debug/pprof/profile?seconds=30">CPU Profile (30s)</a></p>
  <p><a href="/__admin/debug/pprof/trace?seconds=5">Trace (5s)</a></p>
</div>
</body>
</html>`
