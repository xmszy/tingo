package tapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
)

// TestReqJSON 验证 Request.JSON 将 JSON body 解析到结构。
func TestReqJSON(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().POST("/j", func(c *core.Ctx) {
		var m map[string]any
		if err := Req(c).JSON(&m); err != nil {
			t.Errorf("JSON parse error: %v", err)
			return
		}
		c.JSONStatus(http.StatusOK, m)
	})
	req := httptest.NewRequest(http.MethodPost, "/j", strings.NewReader(`{"k":"v"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Body.String() != `{"k":"v"}` {
		t.Fatalf("body = %q", w.Body.String())
	}
}

// TestReqParam 验证 Param 取 query / 路由参数。
func TestReqParam(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/p/:id", func(c *core.Ctx) {
		r := Req(c)
		if r.Param("id") != "7" {
			t.Errorf("route param id = %q", r.Param("id"))
		}
		if r.Get("q", "def") != "x" {
			t.Errorf("query q = %q", r.Get("q"))
		}
		c.String("ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/p/7?q=x", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)
}

// TestReqAll 验证 All 汇总全部参数。
func TestReqAll(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/a", func(c *core.Ctx) {
		all := Req(c).All()
		if all["name"] != "bob" {
			t.Errorf("All name = %q", all["name"])
		}
		c.String("ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/a?name=bob", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)
}

// TestControllerBind 验证 Controller.Bind 绑定并校验失败归类。
func TestControllerBind(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().POST("/b", func(c *core.Ctx) {
		ctrl := &Controller{}
		var r loginReq
		_ = ctrl.Bind(c, &r)
		c.String("ok")
	})
	req := httptest.NewRequest(http.MethodPost, "/b", nil)
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(httptest.NewRecorder(), req)
}

// TestReqIsJson 验证 IsJson 判定。
func TestReqIsJson(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	e.Router().GET("/ij", func(c *core.Ctx) {
		if !Req(c).IsJson() {
			t.Error("expected IsJson true")
		}
		c.String("ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/ij", nil)
	req.Header.Set("Accept", "application/json")
	e.ServeHTTP(httptest.NewRecorder(), req)
}
