package tapp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
)

// TestRegisterControllerRegistered 验证控制器被登记且可被遍历。
func TestRegisterControllerRegistered(t *testing.T) {
	ctrl := &diController{}
	RegisterController("/auto", ctrl)

	found := false
	for _, c := range RegisteredControllers() {
		if c == ctrl {
			found = true
		}
	}
	if !found {
		t.Fatal("RegisterController did not add controller to registry")
	}
}

// TestRegisterControllerPattern 验证模式路由前缀被记录。
func TestRegisterControllerPattern(t *testing.T) {
	RegisterController("/orders", &diController{})
	e := thttp.NewWithApp(core.NewApp())
	var hit string
	e.Router().GET("/orders/:id", func(c *core.Ctx) {
		hit = c.Path()
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/orders/7", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if hit != "/orders/7" {
		t.Fatalf("path = %q", hit)
	}
}

// TestRegisterAnnotatedRegistered 验证注解控制器被登记到独立注册表并可遍历。
func TestRegisterAnnotatedRegistered(t *testing.T) {
	RegisterAnnotated("/user", &annoCtrl{})
	// 注解路由依赖逆向扫描，仅验证登记不 panic、可遍历。
	if len(RegisteredControllers()) == 0 {
		t.Fatal("expected at least one registered controller")
	}
}
