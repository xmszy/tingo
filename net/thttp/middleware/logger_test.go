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
	logger.Configure(LoggerConfig{Output: &output, SkipPaths: []string{"/detect/version"}})
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

	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ordinary", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("ordinary status = %d", response.Code)
	}
	if !strings.Contains(output.String(), "/ordinary") {
		t.Fatalf("ordinary request was not logged: %s", output.String())
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
}
