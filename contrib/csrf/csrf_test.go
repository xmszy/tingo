package csrf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func engineWithCSRF(exclude ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(core.GinChain([]core.Handler{Middleware(Config{Exclude: exclude})})...)
	e.POST("/form", func(c *gin.Context) {
		core.FromGin(c).G().JSON(http.StatusOK, gin.H{"ok": true})
	})
	e.GET("/token", func(c *gin.Context) {
		ctx := core.FromGin(c)
		ctx.G().JSON(http.StatusOK, gin.H{"token": Token(ctx)})
	})
	return e
}

func TestCSRFRejectedWithoutToken(t *testing.T) {
	e := engineWithCSRF()
	req := httptest.NewRequest(http.MethodPost, "/form", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCSRFAcceptedWithToken(t *testing.T) {
	e := engineWithCSRF()
	// 先取令牌（cookie 由响应回写）。
	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("token fetch failed: %d", w.Code)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil || body.Token == "" {
		t.Fatalf("empty token: %v", err)
	}
	cookies := w.Result().Cookies()

	// 带令牌提交。
	req2 := httptest.NewRequest(http.MethodPost, "/form", nil)
	req2.Header.Set(headerName, body.Token)
	for _, ck := range cookies {
		req2.AddCookie(ck)
	}
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}

func TestCSRFExclude(t *testing.T) {
	e := engineWithCSRF("/form")
	req := httptest.NewRequest(http.MethodPost, "/form", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("excluded path should pass, got %d", w.Code)
	}
}

func TestCSRFRotates(t *testing.T) {
	e := engineWithCSRF()
	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	var body struct {
		Token string `json:"token"`
	}
	json.NewDecoder(w.Body).Decode(&body)
	old := body.Token
	cookies := w.Result().Cookies()

	req2 := httptest.NewRequest(http.MethodPost, "/form", nil)
	req2.Header.Set(headerName, old)
	for _, ck := range cookies {
		req2.AddCookie(ck)
	}
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	// 旋转后的 cookie 应与旧令牌不同。
	for _, ck := range w2.Result().Cookies() {
		if ck.Name == cookieName && ck.Value == old {
			t.Fatal("token should rotate after successful post")
		}
	}
}
