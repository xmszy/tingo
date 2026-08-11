package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tcfg"
)

func TestAccessLoggerSkipPathKeepsNotFoundResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := NewAccessLogger()
	logger.Configure(LoggerConfig{Output: &output, SkipPaths: []string{"/detect/version"}, SkipNotFound: true})
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{logger.Handler()})...)

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/detect/version", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if output.Len() != 0 {
		t.Fatalf("skipped request was logged: %s", output.String())
	}

	// 默认 SkipNotFound=true：普通 404 也不应被记录。
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ordinary", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("ordinary status = %d", response.Code)
	}
	if output.Len() != 0 {
		t.Fatalf("404 was logged although SkipNotFound defaults to true: %s", output.String())
	}
}

// TestAccessLoggerSkipNotFoundFalseLogs404 验证显式关闭 SkipNotFound 后 404 仍被记录（审计场景）。
func TestAccessLoggerSkipNotFoundFalseLogs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := NewAccessLogger()
	logger.Configure(LoggerConfig{Output: &output, SkipNotFound: false})
	engine := gin.New()
	engine.Use(core.GinChain([]core.Handler{logger.Handler()})...)

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ordinary", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(output.String(), "/ordinary") {
		t.Fatalf("404 was not logged with SkipNotFound=false: %s", output.String())
	}
}

func TestAccessLoggerConfiguresFromTree(t *testing.T) {
	logger := NewAccessLogger()
	if err := logger.ConfigureFromTree(tcfg.NewFromTree(tcfg.Tree{
		"log": map[string]any{
			"level": "debug",
			"access": map[string]any{
				"enabled":    true,
				"skip_paths": []any{"/health"},
			},
		},
	})); err != nil {
		t.Fatal(err)
	}
	compiled := logger.current.Load()
	if compiled == nil {
		t.Fatal("access logger was not configured")
	}
	if _, ok := compiled.skip["/health"]; !ok {
		t.Fatalf("skip paths = %#v", compiled.skip)
	}
	// 未配置 skip_not_found 时默认 true：404/405 不记录。
	if !compiled.skipNotFound {
		t.Fatalf("skipNotFound should default to true, got %v", compiled.skipNotFound)
	}
}

// TestAccessLoggerFromTreeSkipNotFoundFalse 验证显式 false 时记录 404。
func TestAccessLoggerFromTreeSkipNotFoundFalse(t *testing.T) {
	logger := NewAccessLogger()
	if err := logger.ConfigureFromTree(tcfg.NewFromTree(tcfg.Tree{
		"log": map[string]any{
			"access": map[string]any{
				"enabled":        true,
				"skip_not_found": false,
			},
		},
	})); err != nil {
		t.Fatal(err)
	}
	if logger.current.Load().skipNotFound {
		t.Fatal("skipNotFound should be false when explicitly configured")
	}
}
