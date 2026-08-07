package metric

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestMetricMiddlewareAndRender(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := New()
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(c)})...)
	e.GET("/ok", func(ctx *gin.Context) {
		core.FromGin(ctx).JSON(map[string]any{"a": 1})
	})
	e.GET("/err", func(ctx *gin.Context) {
		core.FromGin(ctx).JSONStatus(500, map[string]any{})
	})

	// 打几个请求。
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		e.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)

	out := c.Render()
	if want := "tingo_http_requests_total 4"; !contains(out, want) {
		t.Fatalf("expected total 4, got:\n%s", out)
	}
	if !contains(out, "code=\"200\"") || !contains(out, "code=\"500\"") {
		t.Fatalf("missing status code metrics:\n%s", out)
	}
	if !contains(out, "tingo_uptime_seconds") {
		t.Fatalf("missing uptime:\n%s", out)
	}
	_ = time.Second
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
