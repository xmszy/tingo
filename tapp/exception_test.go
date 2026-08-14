package tapp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
	"github.com/xmszy/tingo/net/thttp"
)

// TestExceptionHandleReply 验证 Reply 写入成功响应协议。
func TestExceptionHandleReply(t *testing.T) {
	h := NewExceptionHandle()
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/ok", func(c *core.Ctx) {
		h.Reply(c, map[string]any{"id": 1})
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

// TestExceptionHandleFail 验证 Fail 把结构化错误渲染为统一 JSON。
func TestExceptionHandleFail(t *testing.T) {
	h := NewExceptionHandle()
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/boom", func(c *core.Ctx) {
		h.Fail(c, errors.NewError(http.StatusBadRequest, errors.CodeValidation, "bad input"))
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

// TestExceptionShouldReport 验证 IgnoreReport 命中时不再上报。
func TestExceptionShouldReport(t *testing.T) {
	h := NewExceptionHandle()
	if !h.shouldReport(errors.NewError(500, "SERVER_ERROR", "x")) {
		t.Fatal("SERVER_ERROR should be reported")
	}
	if h.shouldReport(errors.NewError(400, errors.CodeValidation, "x")) {
		t.Fatal("CodeValidation should be ignored")
	}
}
