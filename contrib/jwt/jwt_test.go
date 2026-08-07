package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestMiddlewareRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := Middleware(Config{Secret: "s3cret"})

	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{h})...)
	engine.GET("/protected", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMiddlewareAcceptsValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := Middleware(Config{Secret: "s3cret", Exclude: []string{"/login"}})

	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{h})...)
	engine.GET("/protected", func(c *gin.Context) {
		claims := FromContext(core.FromGin(c))
		if claims == nil {
			c.String(500, "no claims")
			return
		}
		c.String(200, "ok")
	})

	tok, err := NewToken("s3cret", "user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestExcludeSkipsAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := Middleware(Config{Secret: "s3cret", Exclude: []string{"/login"}})

	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{h})...)
	engine.GET("/login", func(c *gin.Context) { c.String(200, "login") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for excluded path, got %d", w.Code)
	}
}
