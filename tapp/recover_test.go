package tapp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
)

// TestRecoverCatchesPanic 验证 Recover 中间件捕获 panic 并返回 500。
func TestRecoverCatchesPanic(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Use(Recover(NewExceptionHandle()))
	e.Router().GET("/panic", func(c *core.Ctx) {
		panic("boom")
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestRecoverHealthy 验证无 panic 时正常通行。
func TestRecoverHealthy(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Use(Recover(NewExceptionHandle()))
	e.Router().GET("/ok", func(c *core.Ctx) {
		c.String("ok")
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

// TestIsBrokenPipe 验证对 *net.OpError 的 broken pipe 判定。
func TestIsBrokenPipe(t *testing.T) {
	if isBrokenPipe(nil) {
		t.Fatal("nil should not be broken pipe")
	}
	if isBrokenPipe("some error") {
		t.Fatal("string should not be broken pipe")
	}
}
