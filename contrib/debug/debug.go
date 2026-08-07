// Package debug 提供「开发调试模式」错误页中间件。
//
// 当发生 panic 或框架业务错误（*errors.Error）且 Debug=true 时，渲染一张
// 友好的 HTML 错误页，展示异常信息、堆栈、请求上下文，提升开发期排错体验；
// 生产模式（Debug=false）回退为安全的标准 JSON 500。
package debug

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
)

// Config 配置。
type Config struct {
	// Debug 是否启用详细错误页。生产环境务必置 false。
	Debug bool
	// OnPanic 可选的自定义 panic 记录（如写日志）。
	OnPanic func(err any, stack []byte)
}

// Middleware 返回调试模式恢复中间件。
func Middleware(cfg Config) core.Handler {
	return func(c *core.Ctx) {
		defer func() {
			if err := recover(); err != nil {
				stack := make([]byte, 8192)
				n := runtime.Stack(stack, false)
				stack = stack[:n]
				if cfg.OnPanic != nil {
					cfg.OnPanic(err, stack)
				}
				if !cfg.Debug {
					c.G().AbortWithStatusJSON(http.StatusInternalServerError, map[string]any{
						"code":    500,
						"message": "internal server error",
					})
					return
				}
				c.G().Header("Content-Type", "text/html; charset=utf-8")
				c.G().AbortWithStatus(http.StatusInternalServerError)
				c.G().Writer.Write([]byte(renderPage(err, stack, c)))
			}
		}()
		c.Next()
	}
}

// renderPage 生成 Whoops 风格的 HTML 错误页。
func renderPage(err any, stack []byte, c *core.Ctx) string {
	var errType, errMsg string
	switch e := err.(type) {
	case *errors.Error:
		errType = fmt.Sprintf("Error#%s (HTTP %d)", e.Code, e.Status)
		errMsg = e.Message
	default:
		errType = fmt.Sprintf("%T", err)
		errMsg = fmt.Sprintf("%v", err)
	}
	method := c.Method()
	path := c.Path()
	query := c.Req().URL.RawQuery
	stackStr := strings.ReplaceAll(string(stack), "\n", "<br>")
	stackStr = strings.ReplaceAll(stackStr, " ", "&nbsp;")

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title> tingo 调试面板 - %s </title>
<style>
  body{font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;margin:0;background:#0d1117;color:#e6edf3}
  .wrap{max-width:960px;margin:40px auto;padding:0 20px}
  h1{color:#ff7b72;font-size:22px;margin-bottom:4px}
  .meta{color:#8b949e;font-size:13px;margin-bottom:20px}
  .box{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px 20px;margin-bottom:16px}
  .box h2{font-size:14px;color:#58a6ff;margin:0 0 10px}
  .k{color:#8b949e}
  .v{color:#e6edf3;word-break:break-all}
  pre{white-space:pre-wrap;word-break:break-all;color:#d2a8ff;font-size:12px;line-height:1.5}
  .badge{display:inline-block;background:#21262d;border:1px solid #30363d;border-radius:4px;padding:2px 8px;font-size:12px;color:#7ee787}
</style>
</head>
<body>
<div class="wrap">
  <h1>⚠ %s</h1>
  <div class="meta">tingo debug panel · 请在生产环境关闭 Debug 模式</div>
  <div class="box">
    <h2>异常信息</h2>
    <div><span class="k">类型：</span><span class="v">%s</span></div>
    <div><span class="k">消息：</span><span class="v">%s</span></div>
  </div>
  <div class="box">
    <h2>请求上下文</h2>
    <div><span class="k">方法：</span><span class="v">%s</span> &nbsp; <span class="k">路径：</span><span class="v">%s</span></div>
    <div><span class="k">查询：</span><span class="v">%s</span></div>
    <div><span class="k">客户端：</span><span class="v">%s</span></div>
  </div>
  <div class="box">
    <h2>堆栈跟踪</h2>
    <pre>%s</pre>
  </div>
  <div><span class="badge">tingo</span></div>
</div>
</body>
</html>`,
		errMsg, errType, errType, errMsg,
		method, path, query, c.IP(), stackStr)
}
