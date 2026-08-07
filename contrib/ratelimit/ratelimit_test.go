package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestMemoryLimiterBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	// 1 req/sec, burst 1：第一个过，第二个被拦。
	e.Use(core.GinChain([]core.Handler{Middleware(Config{Rate: 1, Burst: 1})})...)
	e.GET("/", func(c *gin.Context) {
		core.FromGin(c).JSON(map[string]any{"ok": true})
	})

	// 第一个请求应放行。
	w1 := httptest.NewRecorder()
	e.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", w1.Code)
	}
	// 立即第二个请求应被限流（burst 已耗尽）。
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be limited, got %d", w2.Code)
	}
}
