package tapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/net/thttp"
	"github.com/xmszy/tingo/os/tvalid"
)

// mockService 是要被注入到控制器的依赖。
type mockService struct {
	Greeting string
}

// diController 使用 inject tag 声明依赖。
type diController struct {
	Controller
	Svc *mockService `inject:"true"`
}

// TestInjectResolvesField 验证容器绑定后 Inject 自动填充 inject 字段。
func TestInjectResolvesField(t *testing.T) {
	container := core.NewApp().Container()
	core.BindValue(container, &mockService{Greeting: "hi"})
	ctrl := &diController{}
	if err := Inject(container, ctrl); err != nil {
		t.Fatal(err)
	}
	if ctrl.Svc == nil {
		t.Fatal("expected Svc injected, got nil")
	}
	if ctrl.Svc.Greeting != "hi" {
		t.Fatalf("injected Svc.Greeting = %q", ctrl.Svc.Greeting)
	}
}

// TestInjectUnboundReturnsError 验证未绑定类型注入报错。
func TestInjectUnboundReturnsError(t *testing.T) {
	container := core.NewApp().Container()
	ctrl := &diController{}
	if err := Inject(container, ctrl); err == nil {
		t.Fatal("expected error for unbound inject field")
	}
}

// TestRegisterControllerTriggersInject 验证注册后 Kernel.Boot 自动注入。
func TestRegisterControllerTriggersInject(t *testing.T) {
	app := core.NewApp()
	core.BindValue(app.Container(), &mockService{Greeting: "boot"})
	RegisterController("/di", &diController{})
	k := &Kernel{}
	if err := k.Boot(app.Container()); err != nil {
		t.Fatal(err)
	}
	// 验证注入生效：从已注册控制器取出核对。
	for _, c := range RegisteredControllers() {
		if dc, ok := c.(*diController); ok {
			if dc.Svc == nil || dc.Svc.Greeting != "boot" {
				t.Fatalf("Boot did not inject Svc: %+v", dc.Svc)
			}
		}
	}
}

// TestModulePrefix 验证 Module 自动套 /{name} 前缀。
func TestModulePrefix(t *testing.T) {
	e := thttp.NewWithApp(core.NewApp())
	var hit string
	e.Router().Module("admin", func(r core.Router) {
		r.GET("/user", func(c *core.Ctx) {
			hit = c.Path()
		})
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/user", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("module route status = %d, want 200", w.Code)
	}
	if hit != "/admin/user" {
		t.Fatalf("module route path = %q, want /admin/user", hit)
	}
}

// mockValidator 实现 Validator 接口，记录 CheckStruct 是否被调用。
type mockValidator struct {
	called bool
}

func (m *mockValidator) Validate(any, tvalid.RuleSpec) error { return nil }
func (m *mockValidator) CheckStruct(any, ...string) error {
	m.called = true
	return nil
}

// TestValidateUsesDefaultValidator 验证 Request.Validate 走可替换的 DefaultValidator。
func TestValidateUsesDefaultValidator(t *testing.T) {
	mv := &mockValidator{}
	SetDefaultValidator(mv)
	defer SetDefaultValidator(&tvalidValidator{})

	e := thttp.NewWithApp(core.NewApp())
	e.Router().POST("/v", func(c *core.Ctx) {
		_ = Req(c).Validate(&loginReq{Username: "u", Password: "p", Age: 20})
	})
	req := httptest.NewRequest(http.MethodPost, "/v", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(httptest.NewRecorder(), req)
	if !mv.called {
		t.Fatal("expected DefaultValidator.CheckStruct to be used by Request.Validate")
	}
}
