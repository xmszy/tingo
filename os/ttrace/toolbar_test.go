// Package ttrace 工具栏测试。
package ttrace_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xmszy/tingo/os/ttrace"
)

// TestToolbarHTMLInjection 验证 HTML 响应被注入 TP 风格调试面板。
func TestToolbarHTMLInjection(t *testing.T) {
	tb := ttrace.Default()
	tb.Config.Panels = ttrace.Panels{
		Base:  true,
		File:  true,
		Info:  true,
		Error: true,
		SQL:   true,
		Log:   true,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "<!DOCTYPE html><html><head></head><body>Hello</body></html>")
	})

	handler := tb.Middleware()(mux)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Result().Body)
	bodyStr := string(body)

	for _, want := range []string{
		"#tingo_page_trace",                 // TP 面板根节点
		"tingo_page_trace_open",             // 右下角常驻耗时条
		"tingo_page_trace_tab_tit",          // 分区标题
		"基本", "文件", "流程", "错误", "SQL", "调试", // 固定分区
		"Hello",   // 原始内容保留
		"</body>", // body 标签保留
	} {
		if !strings.Contains(bodyStr, want) {
			t.Fatalf("响应缺少 %q\nbody=%s", want, bodyStr)
		}
	}
}

// TestToolbarSessionInfo 验证「基本」分区包含会话 cookie 信息。
func TestToolbarSessionInfo(t *testing.T) {
	tb := ttrace.Default()
	tb.Config.Panels = ttrace.Panels{Base: true}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "<html><body>Test</body></html>")
	})

	handler := tb.Middleware()(mux)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "tingo_session", Value: "abc123_def456"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), "tingo_session") {
		t.Error("基本分区未包含 tingo_session cookie 信息")
	}
}

// TestToolbarSkipAjax 验证 AJAX 请求不注入面板。
func TestToolbarSkipAjax(t *testing.T) {
	tb := ttrace.Default()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"ok":true}`)
	})

	handler := tb.Middleware()(mux)

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Result().Body)
	if strings.Contains(string(body), "tingo_page_trace") {
		t.Error("AJAX 请求不应注入工具栏")
	}
}

// TestToolbarTraceChannel 验证业务 Trace 写入「调试」分区。
func TestToolbarTraceChannel(t *testing.T) {
	ttrace.Trace("用户登录 uid=1", "info")
	ttrace.Trace("自定义调试信息", "debug")

	all := ttrace.AllTrace()
	if len(all["info"]) == 0 {
		t.Error("info 通道未记录")
	}
	if len(all["debug"]) == 0 {
		t.Error("debug 通道未记录")
	}
}

// TestToolbarNonHTMLInjection 验证非 HTML 响应（如 JSON）也注入右下角浮层，
// 与 ThinkPHP 行为一致：数据保留且 Content-Type 改写为 text/html 使浮层可渲染。
func TestToolbarNonHTMLInjection(t *testing.T) {
	tb := ttrace.Default()
	ttrace.LogSQL("SELECT 1", 0)

	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"ok":true}`)
	})

	handler := tb.Middleware()(mux)
	req := httptest.NewRequest("GET", "/api", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "tingo_page_trace") {
		t.Error("JSON 响应未注入浮层（与 TP 行为不符）")
	}
	if !strings.Contains(body, `{"ok":true}`) {
		t.Error("原 JSON 数据丢失")
	}
	ct := rec.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("JSON 响应 Content-Type 未改写为 text/html: %q", ct)
	}
}
