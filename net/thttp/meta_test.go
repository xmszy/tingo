package thttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xmszy/tingo/core"
)

// metaApp 用于验证元信息在真实请求中被正确解析。
type metaApp struct{}

func (metaApp) Config() core.AppConfig { return core.AppConfig{Prefix: "/shop"} }
func (metaApp) Routes(r core.Router) {
	r.Controller("/goods", goodsCtrl{})
}

type goodsCtrl struct{}

func (goodsCtrl) Index(c *core.Ctx) {
	m := c.Route()
	c.String(m.App + "|" + m.Controller + "|" + m.Action)
}

// TestRouteMetaResolved 验证应用/控制器/动作名可被正确读出。
func TestRouteMetaResolved(t *testing.T) {
	core.ResetApps()
	core.RegisterApp("shop", metaApp{})
	e := New()

	w := do(e, http.MethodGet, "/shop/goods", "")
	if got, want := w.Body.String(), "shop|goods_ctrl|Index"; got != want {
		t.Fatalf("route meta = %q, want %q", got, want)
	}
}

// TestRouteMetaDisabled 验证关闭后元信息为空且不报错。
func TestRouteMetaDisabled(t *testing.T) {
	core.ResetApps()
	core.RegisterApp("shop", metaApp{})
	e := New(DisableRouteMeta())

	w := do(e, http.MethodGet, "/shop/goods", "")
	if got, want := w.Body.String(), "||"; got != want {
		t.Fatalf("disabled meta = %q, want %q", got, want)
	}
}

// TestRouteMetaZeroAlloc 是性能契约的回归防线。
//
// 路由元信息是 tingo 相对裸 gin 增加的能力，必须零分配，
// 否则「性能不低于 gin」的承诺就会被破坏。
func TestRouteMetaZeroAlloc(t *testing.T) {
	core.ResetApps()
	core.RegisterApp("shop", metaApp{})
	e := New()
	if err := e.Boot(); err != nil {
		t.Fatal(err)
	}

	// 捕获一个真实请求的 Ctx，其 FullPath 已由 gin 填好。
	var captured *core.Ctx
	e.Router().GET("/probe", func(c *core.Ctx) { captured = c })
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil))
	if captured == nil {
		t.Fatal("failed to capture context")
	}

	got := testing.AllocsPerRun(500, func() {
		sink = captured.Route()
	})
	if got != 0 {
		t.Fatalf("Ctx.Route() must be alloc-free, got %.1f allocs/op", got)
	}
}

// sink 防止编译器优化掉被测调用。
var sink *core.RouteMeta
