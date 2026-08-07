package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGinOfIdentity 校验 GinOf 的 unsafe 函数值转换是语义正确的。
//
// ginOf 依赖「func(*Ctx) 与 func(*gin.Context) 表示相同」这一前提，
// 该前提由 Ctx 的定义方式保证（见 TestCtxLayoutCompatible）。
// 本测试从行为层面再验证一次：转换后的函数必须收到同一个 context 指针，
// 且对它的读写与原生 gin handler 完全一致。
func TestGinOfIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var (
		called  bool
		gotPtr  *gin.Context
		wantPtr *gin.Context
	)

	h := GinOf(func(c *Ctx) {
		called = true
		gotPtr = c.G()
		c.Set("touched", true)
	})

	w := httptest.NewRecorder()
	g, _ := gin.CreateTestContext(w)
	g.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	wantPtr = g

	h(g)

	if !called {
		t.Fatal("converted handler was not invoked")
	}
	if gotPtr != wantPtr {
		t.Fatalf("handler received a different context: got %p, want %p", gotPtr, wantPtr)
	}
	if v, ok := g.Get("touched"); !ok || v != true {
		t.Fatal("writes made through Ctx are not visible on the gin.Context")
	}
}

// TestHandlerOfIdentity 校验反向转换同样正确。
func TestHandlerOfIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPtr *gin.Context
	h := HandlerOf(func(c *gin.Context) {
		gotPtr = c
		c.Set("touched", true)
	})

	w := httptest.NewRecorder()
	g, _ := gin.CreateTestContext(w)
	g.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h(FromGin(g))

	if gotPtr != g {
		t.Fatalf("handler received a different context: got %p, want %p", gotPtr, g)
	}
	if v, ok := g.Get("touched"); !ok || v != true {
		t.Fatal("writes are not visible across the conversion")
	}
}

// TestGinOfZeroAlloc 验证转换本身不分配。
func TestGinOfZeroAlloc(t *testing.T) {
	h := Handler(func(c *Ctx) {})
	got := testing.AllocsPerRun(1000, func() {
		sinkGin = GinOf(h)
	})
	if got != 0 {
		t.Fatalf("GinOf must be alloc-free, got %.1f allocs/op", got)
	}
}

var sinkGin gin.HandlerFunc

// TestGinChainPreservesOrder 验证中间件链转换后顺序不变。
func TestGinChainPreservesOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var order []string
	chain := GinChain([]Handler{
		func(c *Ctx) { order = append(order, "a"); c.Next() },
		func(c *Ctx) { order = append(order, "b"); c.Next() },
		func(c *Ctx) { order = append(order, "c") },
	})

	if len(chain) != 3 {
		t.Fatalf("chain length = %d, want 3", len(chain))
	}

	g := gin.New()
	g.GET("/", chain...)
	g.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("middleware order = %v, want [a b c]", order)
	}
}

// TestAdaptSupportedSignatures 验证各类签名都能被适配且不 panic。
func TestAdaptSupportedSignatures(t *testing.T) {
	cases := map[string]any{
		"func(*Ctx)":         func(c *Ctx) {},
		"Handler":            Handler(func(c *Ctx) {}),
		"func(*gin.Context)": func(c *gin.Context) {},
		"gin.HandlerFunc":    gin.HandlerFunc(func(c *gin.Context) {}),
		"func(*Ctx) error":   func(c *Ctx) error { return nil },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Adapt(fn); got == nil {
				t.Fatal("Adapt returned nil")
			}
		})
	}
}
