package trace

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestTraceInjectsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(Config{})})...)
	e.GET("/", func(c *gin.Context) {
		ctx := core.FromGin(c)
		c.String(200, ID(ctx))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	tid := w.Body.String()
	if tid == "" {
		t.Fatal("missing trace id in body")
	}
	if w.Header().Get(headerTraceID) != tid {
		t.Fatalf("trace header mismatch: %q vs %q", w.Header().Get(headerTraceID), tid)
	}
}

func TestTraceReusesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(Config{})})...)
	e.GET("/", func(c *gin.Context) {
		c.String(200, ID(core.FromGin(c)))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerRequest, "fixed-id-123")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "fixed-id-123") {
		t.Fatalf("should reuse request id, got %q", w.Body.String())
	}
}

func TestTraceSlowLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	slow := make(chan string, 1)
	e.Use(core.GinChain([]core.Handler{Middleware(Config{
		SlowLog: 1 * time.Millisecond,
		OnSlow: func(id, method, path string, d time.Duration) {
			slow <- id
		},
	})})...)
	e.GET("/", func(c *gin.Context) {
		time.Sleep(5 * time.Millisecond)
		c.String(200, "ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)
	select {
	case <-slow:
	case <-time.After(time.Second):
		t.Fatal("slow log not triggered")
	}
}
