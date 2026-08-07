package ttrace

import (
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/xmszy/tingo/core"
)

// ── tingo 原生中间件 ────────────────────────────────────────────────
//
// 通过 core.Ctx 获取请求上下文，把底层 gin.Writer 替换为捕获 writer 以拦截
// 响应写入；handler 返回后再决定是否注入调试面板 / 写入兜底头。
func (tb *Toolbar) Handler() core.Handler {
	return func(c *core.Ctx) {
		for _, p := range tb.skipPaths {
			if strings.HasPrefix(c.Path(), p) {
				c.Next()
				return
			}
		}
		if c.IsAjax() {
			c.Next()
			return
		}
		// 避免跨请求污染收集器。
		ClearErrors()
		ClearSQL()
		ClearTrace()

		start := time.Now()
		capture := &tingoCapture{
			ResponseWriter: c.Res(),
			req:            c.Req(),
			start:          start,
			toolbar:        tb,
		}
		c.G().Writer = capture
		c.Next()
		capture.close()
	}
}

// ── net/http 标准中间件 ─────────────────────────────────────────────

// Middleware 返回 net/http 中间件：仅 text/html 响应注入面板，其它响应写入
// X-Tingo-Trace 头（非 HTML 响应走该兜底头）。
func (tb *Toolbar) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range tb.skipPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			if isAjax(r) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rw := &responseCapture{ResponseWriter: w, req: r, start: start, toolbar: tb}
			next.ServeHTTP(rw, r)
			rw.Close()
		})
	}
}

// injectToolbar 在 HTML 的 </body> 前注入调试面板。
func injectToolbar(body []byte, tb *Toolbar, r *http.Request, start time.Time, statusCode int) []byte {
	idx := lastBodyIndex(body)
	if idx < 0 {
		return body
	}
	elapsed := time.Since(start)
	htmlPart := tb.buildHTML(r, statusCode, elapsed)
	result := make([]byte, 0, len(body)+len(htmlPart))
	result = append(result, body[:idx]...)
	result = append(result, htmlPart...)
	result = append(result, body[idx:]...)
	return result
}

func lastBodyIndex(body []byte) int {
	return bytesLastIndexFold(body, []byte("</body>"))
}

// buildHTML 装配 6 个固定分区并渲染面板。
func (tb *Toolbar) buildHTML(r *http.Request, statusCode int, elapsed time.Duration) []byte {
	p := tb.Config.Panels
	var tabs []traceTab

	if p.Base {
		tabs = append(tabs, traceTab{Title: "基本", Items: tb.baseItems(r, statusCode, elapsed)})
	}
	if p.File {
		tabs = append(tabs, traceTab{Title: "文件", Items: fileItems()})
	}
	if p.Info {
		tabs = append(tabs, traceTab{Title: "流程", Items: logItems(AllTrace(), "info")})
	}
	if p.Error {
		tabs = append(tabs, traceTab{Title: "错误", Items: errorItems()})
	}
	if p.SQL {
		tabs = append(tabs, traceTab{Title: "SQL", Items: sqlItems()})
	}
	if p.Log {
		tabs = append(tabs, traceTab{Title: "调试", Items: debugItems()})
	}

	return []byte(renderTraceHTML(formatElapsedSec(elapsed), tabs))
}

// baseItems 为「基本」分区：运行时间 / 吞吐率 / 内存 / 文件 / 缓存 / 会话。
func (tb *Toolbar) baseItems(r *http.Request, statusCode int, elapsed time.Duration) []string {
	mem := memStats()
	items := []string{
		"请求信息 : " + html.EscapeString(r.Method+" "+r.URL.RequestURI()),
		"状态码 : " + itoa(statusCode),
		"运行时间 : " + formatElapsedSec(elapsed),
		"吞吐率 : " + fmtThroughput(elapsed) + " req/s",
		"内存消耗 : " + fmtMB(mem.Alloc) + " (Alloc) / " + fmtMB(mem.Sys) + " (Sys)",
		"文件加载 : " + itoa(len(fileItems())),
		"会话信息 : " + sessionInfo(r),
		"服务器时间 : " + time.Now().Format("2006-01-02 15:04:05"),
	}
	return items
}

// fileItems 为「文件」分区：本次加载的文件列表。
func fileItems() []string {
	// Go 没有 get_included_files()，这里展示当前 goroutine 调用栈涉及的源文件
	// （运行期可观测）。
	list := runtimeVisibleSource()
	out := make([]string, 0, len(list))
	for _, f := range list {
		out = append(out, html.EscapeString(f))
	}
	return out
}

// logItems 取出某日志级别的行。
func logItems(all map[string][]string, level string) []string {
	lines := all[level]
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, html.EscapeString(l))
	}
	return out
}

// errorItems 合并 error/notice/warning 分区。
func errorItems() []string {
	errs := GetErrors()
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Trace != "" {
			out = append(out, html.EscapeString(e.Message)+" : "+html.EscapeString(e.Trace))
		} else {
			out = append(out, html.EscapeString(e.Message))
		}
	}
	return out
}

// sqlItems 为 SQL 分区。
func sqlItems() []string {
	sqls := GetSQL()
	out := make([]string, 0, len(sqls))
	for i, q := range sqls {
		ms := float64(q.Duration) / float64(time.Millisecond)
		out = append(out, html.EscapeString(itoa(i+1)+". ["+fmt.Sprintf("%.2fms", ms)+"] "+q.SQL))
	}
	return out
}

// debugItems 为「调试」分区：除 info/error/sql 外的业务日志。
func debugItems() []string {
	all := AllTrace()
	skip := map[string]bool{"info": true, "error": true, "notice": true, "warning": true, "sql": true}
	levels := make([]string, 0, len(all))
	for k := range all {
		if !skip[k] {
			levels = append(levels, k)
		}
	}
	sort.Strings(levels)
	var out []string
	for _, lv := range levels {
		for _, l := range all[lv] {
			out = append(out, html.EscapeString("["+lv+"] "+l))
		}
	}
	return out
}

// sessionInfo 返回会话摘要。
func sessionInfo(r *http.Request) string {
	for _, c := range r.Cookies() {
		if c.Name == "tingo_session" || c.Name == "PHPSESSID" {
			v := c.Value
			if len(v) > 24 {
				v = v[:24] + "…"
			}
			return html.EscapeString(c.Name + " => " + v)
		}
	}
	return "（无）"
}
