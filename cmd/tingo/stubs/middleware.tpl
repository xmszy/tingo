// Package middleware 是{{if .App}} {{.App}} 应用的{{end}}中间件（由 tingo make 生成）。
package middleware

import "github.com/xmszy/tingo/core"

// {{.Name}} 是一个示例中间件。
func {{.Name}}() core.Handler {
	return func(c *core.Ctx) {
		// 在请求前做点什么…
		c.Next()
		// 在响应后做点什么…
	}
}
