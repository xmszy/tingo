package thttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
	terrors "github.com/xmszy/tingo/errors"
	"github.com/xmszy/tingo/os/tcfg"
)

// do 发起一次测试请求。
func do(e *Engine, method, path string, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, r)
	return w
}

func TestEngineOwnsStartupOutput(t *testing.T) {
	previousDebugPrint := gin.DebugPrintFunc
	previousRoutePrint := gin.DebugPrintRouteFunc
	t.Cleanup(func() {
		gin.DebugPrintFunc = previousDebugPrint
		gin.DebugPrintRouteFunc = previousRoutePrint
	})

	var output bytes.Buffer
	gin.DebugPrintFunc = func(format string, values ...any) {
		fmt.Fprintf(&output, format, values...)
	}
	gin.DebugPrintRouteFunc = func(method, path, handler string, handlers int) {
		fmt.Fprintf(&output, "%s %s %s %d", method, path, handler, handlers)
	}

	e := NewWithApp(core.NewApp())
	e.Router().GET("/health", func(c *core.Ctx) { c.String("ok") })
	if output.Len() != 0 {
		t.Fatalf("gin debug output leaked through tingo: %s", output.String())
	}
}

func TestStartupURL(t *testing.T) {
	// schemeURL 是启动日志的地址拼装契约：绑定 0.0.0.0/:: 时仍由 printStartup
	// 在外面拼 localhost + 局域网地址多行输出，这里只测单地址拼装。
	cases := []struct {
		name string
		host string
		port string
		tls  bool
		want string
	}{
		{name: "all interfaces localhost", host: "localhost", port: "8080", want: "http://localhost:8080"},
		{name: "ephemeral port", host: "localhost", port: "49152", want: "http://localhost:49152"},
		{name: "lan ip", host: "192.168.1.10", port: "8080", want: "http://192.168.1.10:8080"},
		{name: "https", host: "127.0.0.1", port: "443", tls: true, want: "https://127.0.0.1:443"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := schemeURL("http", test.host, test.port, test.tls); got != test.want {
				t.Fatalf("schemeURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLanIPv4ExcludesLoopback(t *testing.T) {
	ips := lanIPv4()
	for _, ip := range ips {
		if net.ParseIP(ip).IsLoopback() {
			t.Fatalf("lanIPv4 returned loopback address %q", ip)
		}
	}
}

/* ------------------------------------------------------------------ */
/* 多签名 handler 适配                                                  */
/* ------------------------------------------------------------------ */

type echoReq struct {
	Name string `json:"name" form:"name"`
	Age  int    `json:"age"  form:"age"`
}

type echoRes struct {
	Greeting string `json:"greeting"`
	Age      int    `json:"age"`
}

func TestHandlerSignatures(t *testing.T) {
	core.ResetApps()
	e := New()
	r := e.Router()

	// 1. 原生签名
	r.GET("/native", func(c *core.Ctx) { c.String("native") })

	// 2. 带错误返回
	r.GET("/err", func(c *core.Ctx) error {
		return terrors.ErrForbidden
	})

	// 3. 泛型包装，零反射
	r.POST("/generic", core.W(func(c *core.Ctx, req *echoReq) (*echoRes, error) {
		return &echoRes{Greeting: "hi " + req.Name, Age: req.Age}, nil
	}))

	// 4. 反射适配的签名
	r.POST("/reflect", func(c *core.Ctx, req *echoReq) (*echoRes, error) {
		return &echoRes{Greeting: "yo " + req.Name, Age: req.Age}, nil
	})

	t.Run("native", func(t *testing.T) {
		w := do(e, http.MethodGet, "/native", "")
		if w.Code != http.StatusOK || w.Body.String() != "native" {
			t.Fatalf("got %d %q", w.Code, w.Body.String())
		}
	})

	t.Run("error is mapped to status and code", func(t *testing.T) {
		w := do(e, http.MethodGet, "/err", "")
		if w.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", w.Code)
		}
		var got terrors.Error
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Code != terrors.CodeForbidden {
			t.Fatalf("want code %s, got %s", terrors.CodeForbidden, got.Code)
		}
	})

	for _, tc := range []struct{ path, want string }{
		{"/generic", "hi ada"},
		{"/reflect", "yo ada"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := do(e, http.MethodPost, tc.path, `{"name":"ada","age":36}`)
			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
			}
			var env struct {
				Data echoRes `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if env.Data.Greeting != tc.want || env.Data.Age != 36 {
				t.Fatalf("got %+v", env.Data)
			}
		})
	}
}

/* ------------------------------------------------------------------ */
/* 多应用装配                                                           */
/* ------------------------------------------------------------------ */

// adminApp 是前缀挂载的应用。
type adminApp struct{}

func (adminApp) Config() core.AppConfig { return core.AppConfig{Prefix: "/admin"} }
func (adminApp) Routes(r core.Router) {
	r.GET("/ping", func(c *core.Ctx) { c.String("admin:" + c.App()) })
}

// indexApp 是挂在根路径的默认应用。
type indexApp struct{}

func (indexApp) Config() core.AppConfig { return core.AppConfig{Default: true} }
func (indexApp) Routes(r core.Router) {
	r.GET("/ping", func(c *core.Ctx) { c.String("index:" + c.App()) })
}

// bootApp 用于验证 Boot 钩子。
type bootApp struct{ booted *bool }

func (a bootApp) Config() core.AppConfig { return core.AppConfig{Prefix: "/boot"} }
func (a bootApp) Boot() error            { *a.booted = true; return nil }
func (a bootApp) Routes(r core.Router)   { r.GET("/x", func(c *core.Ctx) { c.String("ok") }) }

func TestMultiApp(t *testing.T) {
	core.ResetApps()
	booted := false
	core.RegisterApp("admin", adminApp{})
	core.RegisterApp("index", indexApp{})
	core.RegisterApp("boot", bootApp{booted: &booted})

	e := New()

	cases := []struct{ path, want string }{
		{"/admin/ping", "admin:admin"},
		{"/ping", "index:index"},
		{"/boot/x", "ok"},
	}
	for _, tc := range cases {
		w := do(e, http.MethodGet, tc.path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", tc.path, w.Code)
		}
		if w.Body.String() != tc.want {
			t.Fatalf("%s: want %q, got %q", tc.path, tc.want, w.Body.String())
		}
	}

	if !booted {
		t.Fatal("Boot hook was not invoked")
	}
}

// TestAppMiddlewareIsolation 验证应用级中间件只作用于本应用。
type mwApp struct{ tag string }

func (a mwApp) Config() core.AppConfig { return core.AppConfig{Prefix: "/" + a.tag} }
func (a mwApp) Middlewares() []core.Handler {
	tag := a.tag
	return []core.Handler{func(c *core.Ctx) {
		c.SetHeader("X-App-Tag", tag)
		c.Next()
	}}
}
func (a mwApp) Routes(r core.Router) {
	r.GET("/t", func(c *core.Ctx) { c.String("ok") })
}

func TestAppMiddlewareIsolation(t *testing.T) {
	core.ResetApps()
	core.RegisterApp("alpha", mwApp{tag: "alpha"})
	core.RegisterApp("beta", mwApp{tag: "beta"})

	e := New()

	if got := do(e, http.MethodGet, "/alpha/t", "").Header().Get("X-App-Tag"); got != "alpha" {
		t.Fatalf("alpha app got tag %q", got)
	}
	if got := do(e, http.MethodGet, "/beta/t", "").Header().Get("X-App-Tag"); got != "beta" {
		t.Fatalf("beta app got tag %q", got)
	}
}

func TestEngineAppIsolation(t *testing.T) {
	first := core.NewApp()
	second := core.NewApp()
	first.RegisterApplication("first", indexApp{})
	second.RegisterApplication("second", indexApp{})

	firstEngine := NewWithApp(first)
	secondEngine := NewWithApp(second)

	if got := do(firstEngine, http.MethodGet, "/ping", "").Body.String(); got != "index:first" {
		t.Fatalf("first engine response = %q", got)
	}
	if got := do(secondEngine, http.MethodGet, "/ping", "").Body.String(); got != "index:second" {
		t.Fatalf("second engine response = %q", got)
	}
}

/* ------------------------------------------------------------------ */
/* 资源路由与约定式控制器                                                 */
/* ------------------------------------------------------------------ */

// userCtrl 只实现部分 REST 动作，验证未实现的动作不会被注册。
type userCtrl struct{}

func (userCtrl) Index(c *core.Ctx)  { c.String("index") }
func (userCtrl) Read(c *core.Ctx)   { c.String("read:" + c.Param("id")) }
func (userCtrl) Save(c *core.Ctx)   { c.String("save") }
func (userCtrl) Create(c *core.Ctx) { c.String("create") }

func TestResourceRoutes(t *testing.T) {
	core.ResetApps()
	e := New()
	e.Router().Resource("/users", userCtrl{})

	cases := []struct {
		method, path, want string
		code               int
	}{
		{http.MethodGet, "/users", "index", http.StatusOK},
		{http.MethodGet, "/users/create", "create", http.StatusOK},
		{http.MethodPost, "/users", "save", http.StatusOK},
		{http.MethodGet, "/users/7", "read:7", http.StatusOK},
		// Update / Delete 未实现，故未注册。
		// 由于 /users/:id 上存在 GET，gin 返回 405 而非 404。
		{http.MethodDelete, "/users/7", "", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		w := do(e, tc.method, tc.path, "")
		if w.Code != tc.code {
			t.Fatalf("%s %s: want %d, got %d", tc.method, tc.path, tc.code, w.Code)
		}
		if tc.want != "" && w.Body.String() != tc.want {
			t.Fatalf("%s %s: want %q, got %q", tc.method, tc.path, tc.want, w.Body.String())
		}
	}
}

// profileCtrl 验证约定式控制器的方法名映射。
type profileCtrl struct{}

func (profileCtrl) Index(c *core.Ctx)      { c.String("index") }
func (profileCtrl) GetSetting(c *core.Ctx) { c.String("get-setting") }
func (profileCtrl) PostAvatar(c *core.Ctx) { c.String("post-avatar") }
func (profileCtrl) UserInfo(c *core.Ctx)   { c.String("user-info") }

func TestControllerConvention(t *testing.T) {
	core.ResetApps()
	e := New()
	e.Router().Controller("/profile", profileCtrl{})

	cases := []struct {
		method, path, want string
	}{
		{http.MethodGet, "/profile", "index"},
		{http.MethodGet, "/profile/setting", "get-setting"},
		{http.MethodPost, "/profile/avatar", "post-avatar"},
		{http.MethodGet, "/profile/user_info", "user-info"},
	}
	for _, tc := range cases {
		w := do(e, tc.method, tc.path, "")
		if w.Code != http.StatusOK || w.Body.String() != tc.want {
			t.Fatalf("%s %s: got %d %q", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// GetSetting 注册为 GET，POST 应当不被允许
	if w := do(e, http.MethodPost, "/profile/setting", ""); w.Code == http.StatusOK {
		t.Fatal("POST /profile/setting should not be routed to a GET-only action")
	}

	// Index 同时兼容 /profile/index 这种“控制器/方法”写法
	if w := do(e, http.MethodGet, "/profile/index", ""); w.Code != http.StatusOK || w.Body.String() != "index" {
		t.Fatalf("GET /profile/index: got %d %q", w.Code, w.Body.String())
	}
}

/* ------------------------------------------------------------------ */
/* snake 命名转换                                                       */
/* ------------------------------------------------------------------ */

func TestSnake(t *testing.T) {
	cases := map[string]string{
		"UserInfo":   "user_info",
		"User":       "user",
		"HTTPServer": "http_server",
		"ID":         "id",
		"OAuthToken": "o_auth_token",
		"":           "",
		"lower":      "lower",
	}
	for in, want := range cases {
		if got := snake(in); got != want {
			t.Errorf("snake(%q) = %q, want %q", in, got, want)
		}
	}
}

/* ------------------------------------------------------------------ */
/* 404 / 405                                                           */
/* ------------------------------------------------------------------ */

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	core.ResetApps()
	e := New()
	e.Router().GET("/only-get", func(c *core.Ctx) { c.String("ok") })

	w := do(e, http.MethodGet, "/nope", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), terrors.CodeNotFound) {
		t.Fatalf("404 body should carry structured code, got %s", w.Body)
	}

	w = do(e, http.MethodPost, "/only-get", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", w.Code)
	}
}

func TestEnableAdmin(t *testing.T) {
	core.ResetApps()
	e := New()
	e.EnableAdmin()

	// 管理端点首页
	w := do(e, http.MethodGet, "/__admin/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /__admin/ want 200, got %d: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "Tingo Admin") {
		t.Fatalf("GET /__admin/ should contain 'Tingo Admin', got: %s", w.Body)
	}

	// 健康检查
	w = do(e, http.MethodGet, "/__admin/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /__admin/health want 200, got %d", w.Code)
	}

	// pprof 首页（httppprof.Index）
	w = do(e, http.MethodGet, "/__admin/debug/pprof/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /__admin/debug/pprof/ want 200, got %d: %s", w.Code, w.Body)
	}

	// pprof 子页面：不带 ?debug=1 时输出二进制（浏览器不可读），
	// 带 ?debug=1 时输出可读文本/HTML。
	profiles := []string{"goroutine", "heap", "allocs", "block", "mutex", "threadcreate"}
	for _, name := range profiles {
		w = do(e, http.MethodGet, "/__admin/debug/pprof/"+name+"?debug=1", "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /__admin/debug/pprof/%s?debug=1 want 200, got %d: %s", name, w.Code, w.Body)
		}
		if w.Body.Len() == 0 {
			t.Fatalf("GET /__admin/debug/pprof/%s?debug=1 body is empty", name)
		}
	}

	// pprof profile 和 trace 需要 seconds 参数，这里只验证路由可达
	w = do(e, http.MethodGet, "/__admin/debug/pprof/profile?seconds=0", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /__admin/debug/pprof/profile?seconds=0 want 200, got %d", w.Code)
	}
	w = do(e, http.MethodGet, "/__admin/debug/pprof/trace?seconds=0", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /__admin/debug/pprof/trace?seconds=0 want 200, got %d", w.Code)
	}

	// 确认管理员路径不会与自动路由冲突
	e.Router().GET("/hello", func(c *core.Ctx) { c.String("hello") })
	w = do(e, http.MethodGet, "/hello", "")
	if w.Code != http.StatusOK || w.Body.String() != "hello" {
		t.Fatalf("GET /hello want 200 'hello', got %d: %s", w.Code, w.Body)
	}
}

func TestConfigureAdminFromTree(t *testing.T) {
	core.ResetApps()

	// 测试 1：admin.enabled = false 时，不暴露任何 admin 端点
	e1 := New()
	err := e1.ConfigureAdminFromTree(tcfg.Tree{"admin": tcfg.Tree{"enabled": false}})
	if err != nil {
		t.Fatalf("ConfigureAdminFromTree(enabled=false): %v", err)
	}
	w := do(e1, http.MethodGet, "/__admin/", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("admin disabled: /__admin/ want 404, got %d", w.Code)
	}

	// 测试 2：admin.enabled = true 时，admin 端点可用
	core.ResetApps()
	e2 := New()
	err = e2.ConfigureAdminFromTree(tcfg.Tree{
		"admin": tcfg.Tree{
			"enabled":       true,
			"path_prefix":   "/__admin",
			"enable_pprof":  true,
			"enable_status": true,
		},
	})
	if err != nil {
		t.Fatalf("ConfigureAdminFromTree(enabled=true): %v", err)
	}
	w = do(e2, http.MethodGet, "/__admin/", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Tingo Admin") {
		t.Fatalf("admin enabled: /__admin/ want 200 with Tingo Admin, got %d: %s", w.Code, w.Body)
	}
	w = do(e2, http.MethodGet, "/__admin/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin enabled: /__admin/health want 200, got %d", w.Code)
	}

	// 测试 3：自定义前缀
	core.ResetApps()
	e3 := New()
	err = e3.ConfigureAdminFromTree(tcfg.Tree{
		"admin": tcfg.Tree{
			"enabled":     true,
			"path_prefix": "/_admin",
		},
	})
	if err != nil {
		t.Fatalf("ConfigureAdminFromTree(custom prefix): %v", err)
	}
	w = do(e3, http.MethodGet, "/_admin/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("custom prefix: /_admin/ want 200, got %d", w.Code)
	}
	// 原前缀已不可用
	w = do(e3, http.MethodGet, "/__admin/", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("custom prefix: /__admin/ want 404, got %d", w.Code)
	}
}
