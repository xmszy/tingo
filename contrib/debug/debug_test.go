package debug

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
)

func TestDebugPageOnPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(Config{Debug: true})})...)
	e.GET("/", func(c *gin.Context) {
		panic("boom")
	})
	req := httptest.NewRequest(http.MethodGet, "/?x=1", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "boom") || !strings.Contains(body, "tingo debug panel") {
		t.Fatalf("debug page missing content:\n%s", body)
	}
}

func TestDebugFalseReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(Config{Debug: false})})...)
	e.GET("/", func(c *gin.Context) {
		panic(&errors.Error{Code: "1001", Message: "db fail", Status: 500})
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "<html") {
		t.Fatal("debug page should not render when Debug=false")
	}
}

func TestErrorsErrorShown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(Config{Debug: true})})...)
	e.GET("/", func(c *gin.Context) {
		panic(&errors.Error{Code: "2001", Message: "校验失败", Status: 500})
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "校验失败") {
		t.Fatalf("error message not shown:\n%s", w.Body.String())
	}
}
