package tingo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
)

type isolatedApplication struct {
	framework **core.App
}

func (isolatedApplication) Config() core.AppConfig { return core.AppConfig{Default: true} }
func (a isolatedApplication) Routes(router core.Router) {
	router.GET("/", func(ctx *core.Ctx) {
		*a.framework = ctx.Framework()
		ctx.String("ok")
	})
}

func TestNewAppOwnsRuntimeState(t *testing.T) {
	var requestApp *core.App
	framework := NewApp(thttp.WithMode(thttp.ModeTest)).
		App("index", isolatedApplication{framework: &requestApp})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	framework.Engine().ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if requestApp != framework.Core() {
		t.Fatal("request was not bound to the framework app")
	}
	if requestApp == core.DefaultApp() {
		t.Fatal("explicit framework unexpectedly used the default app")
	}
}
