package tapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
)

// TestJSONHelper 验证 JSON 辅助函数写入 JSON 响应。
func TestJSONHelper(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/h", func(c *core.Ctx) {
		JSON(c, map[string]any{"a": 1})
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/h", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"a":1`) {
		t.Fatalf("body = %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
}
