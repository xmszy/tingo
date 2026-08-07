package ttrace

import (
	"fmt"
	"net/http"
	"strings"
)

// isAjax 判断是否为 AJAX 请求。
func isAjax(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest")
}

// Console 输出调试信息到命令行（用于 CLI 运行场景）。
func (tb *Toolbar) Console() string {
	var sb strings.Builder
	sb.WriteString("\n========== Tingo Page Trace ==========\n")
	type tabOut struct {
		title string
		items []string
	}
	for _, t := range []tabOut{
		{"基本", nil},
		{"SQL", sqlItems()},
		{"错误", errorItems()},
		{"调试", debugItems()},
	} {
		sb.WriteString(fmt.Sprintf("[%s]\n", t.title))
		for i, it := range t.items {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, it))
		}
	}
	sb.WriteString("=========================================\n")
	return sb.String()
}

// Bookmarklet 返回一段书签脚本，用于在无法注入 body 的响应（JSON/API）上，
// 通过读取 X-Tingo-Trace 头渲染一个浮动调试窗口。
func Bookmarklet() string {
	return `javascript:(function(){var x=document.createElement('script');x.src='https://unpkg.com/tingo-trace/bookmarklet.js';document.body.appendChild(x);})();`
}

// Enable 返回一个 core 中间件函数，便于框架门面包 t.Use(t.EnableToolbar()) 调用。
//
// 实际启用通过 Toolbar.Handler() 完成；此处仅为兼容旧 API 占位。
func (tb *Toolbar) Enable() {}
