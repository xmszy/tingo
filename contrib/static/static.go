// Package static 提供静态文件服务中间件。
//
// 设计：零外部依赖，使用标准库 http.FileServer。当请求路径命中静态文件时
// 直接响应并终止链路；否则放行到后续路由。
package static

import (
	"net/http"
	"strings"

	"github.com/xmszy/tingo/core"
)

// Middleware 在 root 目录下提供静态文件服务，仅对以 prefix 开头的请求生效。
//
//	t.Use(static.Middleware("/static", "./public"))
//
// 访问 /static/foo.png 将读取 ./public/foo.png。
func Middleware(prefix, root string) core.Handler {
	fileServer := http.FileServer(http.Dir(root))
	strip := strings.TrimSuffix(prefix, "/")
	return func(c *core.Ctx) {
		p := c.Path()
		if !strings.HasPrefix(p, prefix) {
			c.Next()
			return
		}
		// 去掉前缀后得到文件相对路径。
		rel := strings.TrimPrefix(p, strip)
		if rel == "" || rel == "/" {
			rel = "/index.html"
		}
		c.G().Request.URL.Path = rel
		fileServer.ServeHTTP(c.Res(), c.Req())
		c.Abort()
	}
}
