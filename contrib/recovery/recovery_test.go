package recovery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func TestRecoversPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Middleware()})...)
	engine.GET("/", func(c *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
