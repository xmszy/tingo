package ttrace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ── 辅助函数 ────────────────────────────────────────────────────────

func itoa(n int) string { return strconv.Itoa(n) }

func fmtMB(b uint64) string {
	return strconv.FormatFloat(float64(b)/1024/1024, 'f', 2, 64) + " MB"
}

func fmtThroughput(elapsed time.Duration) string {
	if elapsed <= 0 {
		return "—"
	}
	rps := float64(time.Second) / float64(elapsed)
	if rps >= 1e6 {
		return fmt.Sprintf("%.2fM", rps/1e6)
	}
	if rps >= 1e3 {
		return fmt.Sprintf("%.2fK", rps/1e3)
	}
	return strconv.FormatFloat(rps, 'f', 2, 64)
}

func bytesLastIndexFold(s, sep []byte) int {
	if len(sep) == 0 {
		return len(s)
	}
	ls := strings.ToLower(string(s))
	lsep := strings.ToLower(string(sep))
	return strings.LastIndex(ls, lsep)
}

// runtimeVisibleSource 收集当前 goroutine 调用栈中出现的 .go 源文件路径（去重、排序）。
func runtimeVisibleSource() []string {
	set := map[string]struct{}{}
	const depth = 64
	pc := make([]uintptr, depth)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	for {
		fr, more := frames.Next()
		if strings.HasSuffix(fr.File, ".go") {
			set[fr.File] = struct{}{}
		}
		if !more {
			break
		}
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// ── tingo 捕获 writer（缓冲 HTML 以便注入） ──────────────────────────

// tingoCapture 包裹 gin.ResponseWriter，缓冲所有写入，close() 时决定是否注入面板。
type tingoCapture struct {
	gin.ResponseWriter
	req     *http.Request
	start   time.Time
	toolbar *Toolbar

	buf     bytes.Buffer
	status  int
	written bool
}

func (w *tingoCapture) Header() http.Header { return w.ResponseWriter.Header() }

func (w *tingoCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	w.written = true
	return len(b), nil
}

func (w *tingoCapture) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *tingoCapture) WriteHeader(code int) { w.status = code }

// gin.ResponseWriter 需要的状态查询方法。
func (w *tingoCapture) Status() int   { return w.status }
func (w *tingoCapture) Size() int      { return w.buf.Len() }
func (w *tingoCapture) Written() bool  { return w.written }
func (w *tingoCapture) WriteHeaderNow() {}

// shouldInjectBody 判断是否把面板 HTML 拼进响应体。
// 对齐 ThinkPHP 行为：所有响应类型（HTML / 纯文本 / JSON / XML 等）都会注入浮层，
// 仅当状态码为 204（No Content）时不注入。JSON/XML 被注入后会变为「数据 + 浮层 HTML」混合，
// 与 TP 一致（debug 模式下以浏览器可直接查看调试信息为优先）。
func shouldInjectBody(ct string) bool {
	_ = ct
	return true
}

// close 把缓冲内容写出（注入面板），并写入兜底头。
func (w *tingoCapture) close() {
	body := w.buf.Bytes()
	ct := w.ResponseWriter.Header().Get("Content-Type")
	inject := shouldInjectBody(ct)

	if inject && w.status != http.StatusNoContent {
		body = injectToolbar(body, w.toolbar, w.req, w.start, w.status)
		// 非 HTML 文本响应（纯文本/字符串/无 Content-Type）改写为 text/html，
		// 使右下角浮层被浏览器正常渲染，而不是把面板 HTML 当作源码显示在页面里。
		if !strings.Contains(strings.ToLower(ct), "text/html") {
			w.ResponseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
	} else {
		w.ResponseWriter.Header().Set("X-Tingo-Trace", w.toolbar.summary(w.req, w.status, time.Since(w.start)))
	}

	if w.status == 0 {
		w.status = http.StatusOK
	}
	// 注入会改变 body 长度，故删除已声明的 Content-Length，
	// 由传输层改用分块编码，避免 ERR_CONTENT_LENGTH_MISMATCH。
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(w.status)
	if len(body) > 0 {
		w.ResponseWriter.Write(body)
	}
}

// ── net/http 捕获 writer ────────────────────────────────────────────

type responseCapture struct {
	http.ResponseWriter
	req     *http.Request
	start   time.Time
	toolbar *Toolbar

	buf     bytes.Buffer
	status  int
	written bool
}

func (w *responseCapture) Header() http.Header { return w.ResponseWriter.Header() }

func (w *responseCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	w.written = true
	return len(b), nil
}

func (w *responseCapture) WriteHeader(code int) { w.status = code }

func (w *responseCapture) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Close 关闭捕获 writer。
func (w *responseCapture) Close() {
	body := w.buf.Bytes()
	ct := w.ResponseWriter.Header().Get("Content-Type")
	inject := shouldInjectBody(ct)

	if inject && w.status != http.StatusNoContent {
		body = injectToolbar(body, w.toolbar, w.req, w.start, w.status)
		// 非 HTML 文本响应（纯文本/字符串/无 Content-Type）改写为 text/html，
		// 使右下角浮层被浏览器正常渲染，而不是把面板 HTML 当作源码显示在页面里。
		if !strings.Contains(strings.ToLower(ct), "text/html") {
			w.ResponseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
	} else {
		w.ResponseWriter.Header().Set("X-Tingo-Trace", w.toolbar.summary(w.req, w.status, time.Since(w.start)))
	}

	if w.status == 0 {
		w.status = http.StatusOK
	}
	// 注入会改变 body 长度，删除已声明的 Content-Length，避免 ERR_CONTENT_LENGTH_MISMATCH。
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(w.status)
	if len(body) > 0 {
		w.ResponseWriter.Write(body)
	}
}

// summary 生成 X-Tingo-Trace 头的紧凑 JSON（非 HTML 响应的可视化兜底）。
//
// 直接 fmt.Fprintf 进 strings.Builder，避免中间字符串临时拼接（WriteString 警告）。
func (tb *Toolbar) summary(r *http.Request, status int, elapsed time.Duration) string {
	sqls := GetSQL()
	sqlMs := 0.0
	for _, q := range sqls {
		sqlMs += float64(q.Duration) / float64(time.Millisecond)
	}
	errs := GetErrors()
	mem := memStats()
	if status == 0 {
		status = http.StatusOK
	}

	var sb strings.Builder
	sb.WriteString("{")
	fmt.Fprintf(&sb, `"method":%s`, jsonEscape(r.Method))
	fmt.Fprintf(&sb, `,"path":%s`, jsonEscape(r.URL.Path))
	fmt.Fprintf(&sb, `,"status":%d`, status)
	fmt.Fprintf(&sb, `,"elapsed_ms":%.4f`, float64(elapsed)/float64(time.Millisecond))
	fmt.Fprintf(&sb, `,"alloc_mb":%.2f`, float64(mem.Alloc)/1024/1024)
	fmt.Fprintf(&sb, `,"sql":%d`, len(sqls))
	fmt.Fprintf(&sb, `,"sql_ms":%.2f`, sqlMs)
	fmt.Fprintf(&sb, `,"errors":%d`, len(errs))
	fmt.Fprintf(&sb, `,"records":%s`, encodeRecords())
	sb.WriteString("}")
	return sb.String()
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func encodeRecords() string {
	b, err := json.Marshal(AllTrace())
	if err != nil {
		return "{}"
	}
	return string(b)
}
