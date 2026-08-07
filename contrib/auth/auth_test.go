package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestBasicRejectsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Basic("test", map[string]string{"user": "pass"})})...)
	engine.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBasicAcceptsValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Basic("test", map[string]string{"user": "pass"})})...)
	engine.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestBearerRejectsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Bearer("good-token")})...)
	engine.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
