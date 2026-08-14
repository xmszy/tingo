package tapp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
)

// TestControllerSuccess 验证 Success 写入标准成功响应。
func TestControllerSuccess(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/s", func(c *core.Ctx) {
		(&Controller{}).Success(c, map[string]any{"id": 1})
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
}

// TestControllerResult 验证 Result 写入自定义业务码响应。
func TestControllerResult(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/r", func(c *core.Ctx) {
		(&Controller{}).Result(c, nil, 40001, "bad")
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/r", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("empty body")
	}
}
