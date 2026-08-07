package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestMiddlewareSetsHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := Middleware(Default())

	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{h})...)
	engine.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Allow-Origin *, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected Allow-Methods header")
	}
}

func TestOptionsShortCircuit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := Middleware(Default())
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{h})...)
	engine.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
}
