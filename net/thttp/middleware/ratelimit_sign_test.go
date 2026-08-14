package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

func runRateLimit(_ *testing.T, mw core.Handler, n int) int {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{mw})...)
	engine.GET("/ping", func(c *gin.Context) { c.String(200, "ok") })

	allowed := 0
	for i := 0; i < n; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
		if w.Code == http.StatusOK {
			allowed++
		}
	}
	return allowed
}

// TestRateLimitAllowsUpToLimit 验证限流阈值内全部放行。
func TestRateLimitAllowsUpToLimit(t *testing.T) {
	mw := RateLimit(func(c *RateLimitConfig) {
		c.Limit = 3
		c.Window = time.Minute
		c.KeyFunc = func(*core.Ctx) string { return "same-ip" }
	})
	if got := runRateLimit(t, mw, 3); got != 3 {
		t.Fatalf("allowed = %d, want 3", got)
	}
}

// TestRateLimitBlocksBeyondLimit 验证超过阈值被拒绝。
func TestRateLimitBlocksBeyondLimit(t *testing.T) {
	mw := RateLimit(func(c *RateLimitConfig) {
		c.Limit = 2
		c.Window = time.Minute
		c.KeyFunc = func(*core.Ctx) string { return "same-ip" }
	})
	if got := runRateLimit(t, mw, 5); got != 2 {
		t.Fatalf("allowed = %d, want 2", got)
	}
}

// TestRateLimitPerKeyIsolated 验证不同 key 独立计数。
func TestRateLimitPerKeyIsolated(t *testing.T) {
	var ip int
	mw := RateLimit(func(c *RateLimitConfig) {
		c.Limit = 1
		c.Window = time.Minute
		c.KeyFunc = func(*core.Ctx) string {
			ip++
			return "ip-" + string(rune('a'+ip-1))
		}
	})
	// 每个 key 第一次请求独立放行。
	if got := runRateLimit(t, mw, 2); got != 2 {
		t.Fatalf("per-key allowed = %d, want 2", got)
	}
}

// TestSignValid 验证合法签名放行。
func TestSignValid(t *testing.T) {
	secret := "s3cr3t"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{"app_key": "app1", "timestamp": ts, "name": "bob"}
	nonce := "abc123"
	sign := computeSign(params, nonce, secret)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Sign(func(c *SignConfig) { c.Secret = secret })})...)
	engine.GET("/api", func(c *gin.Context) { c.String(200, "ok") })

	url := "/api?app_key=app1&timestamp=" + ts + "&name=bob&nonce=" + nonce + "&sign=" + sign
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("valid sign status = %d, body=%s", w.Code, w.Body.String())
	}
}

// TestSignRejectsBad 验证非法签名被拒绝。
func TestSignRejectsBad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Sign(func(c *SignConfig) { c.Secret = "s3cr3t" })})...)
	engine.GET("/api", func(c *gin.Context) { c.String(200, "ok") })

	url := "/api?app_key=app1&timestamp=1700000000&nonce=abc123&sign=deadbeef"
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad sign status = %d, want 401", w.Code)
	}
}

// TestSignRejectsExpiredTimestamp 验证过期时间戳被拒绝。
func TestSignRejectsExpiredTimestamp(t *testing.T) {
	secret := "s3cr3t"
	params := map[string]string{"app_key": "app1", "timestamp": "1000000000", "name": "bob"}
	sign := computeSign(params, "abc123", secret)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Sign(func(c *SignConfig) { c.Secret = secret })})...)
	engine.GET("/api", func(c *gin.Context) { c.String(200, "ok") })

	url := "/api?app_key=app1&timestamp=1000000000&name=bob&nonce=abc123&sign=" + sign
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired timestamp status = %d, want 401", w.Code)
	}
}

// TestSignIgnorePaths 验证白名单路径跳过校验。
func TestSignIgnorePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{Sign(func(c *SignConfig) {
		c.Secret = "s3cr3t"
		c.IgnorePaths = []string{"/health"}
	})})...)
	engine.GET("/health", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ignore path status = %d, want 200", w.Code)
	}
}
