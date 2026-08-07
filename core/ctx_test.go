package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"unsafe"

	"github.com/gin-gonic/gin"
)

// TestCtxLayoutCompatible 是整个框架性能模型的基石断言。
//
// Ctx 通过 `type Ctx gin.Context` 定义，Go 规范保证二者底层类型相同，
// 因此大小与对齐必须完全一致，(*Ctx)(g) 转换才是零成本且安全的。
// 一旦 gin 升级导致布局变化，此测试会立即失败。
func TestCtxLayoutCompatible(t *testing.T) {
	if a, b := unsafe.Sizeof(Ctx{}), unsafe.Sizeof(gin.Context{}); a != b {
		t.Fatalf("size mismatch: Ctx=%d gin.Context=%d", a, b)
	}
	if a, b := unsafe.Alignof(Ctx{}), unsafe.Alignof(gin.Context{}); a != b {
		t.Fatalf("align mismatch: Ctx=%d gin.Context=%d", a, b)
	}
}

// TestCtxConversionIdentity 验证指针转换前后是同一块内存，
// 即转换不发生任何拷贝。
func TestCtxConversionIdentity(t *testing.T) {
	g := &gin.Context{}
	c := FromGin(g)
	if unsafe.Pointer(c) != unsafe.Pointer(g) {
		t.Fatal("FromGin must not copy: pointer identity lost")
	}
	if ToGin(c) != g {
		t.Fatal("ToGin must return the original pointer")
	}
	if c.G() != g {
		t.Fatal("G() must return the original pointer")
	}
}

// TestCtxConversionZeroAlloc 验证转换路径不产生堆分配。
func TestCtxConversionZeroAlloc(t *testing.T) {
	g := &gin.Context{}
	got := testing.AllocsPerRun(1000, func() {
		c := FromGin(g)
		_ = ToGin(c)
	})
	if got != 0 {
		t.Fatalf("conversion should be alloc-free, got %.1f allocs/op", got)
	}
}

// TestCtxSharesStateWithGin 验证通过 Ctx 与 gin.Context 读写的是同一份状态，
// 证明二者确为同一对象的两个视图。
func TestCtxSharesStateWithGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	g, _ := gin.CreateTestContext(w)
	g.Request = httptest.NewRequest(http.MethodGet, "/ping?a=1", nil)

	c := FromGin(g)

	// 经 Ctx 写入，经 gin 读出
	c.Set("k", "v")
	if got := g.GetString("k"); got != "v" {
		t.Fatalf("state not shared, gin side got %q", got)
	}
	// 经 gin 写入，经 Ctx 读出
	g.Set("k2", 42)
	if got := c.GetInt("k2"); got != 42 {
		t.Fatalf("state not shared, ctx side got %d", got)
	}
	// 请求字段可直接访问
	if c.Method() != http.MethodGet || c.Path() != "/ping" || c.Query("a") != "1" {
		t.Fatalf("request accessors broken: %s %s %s", c.Method(), c.Path(), c.Query("a"))
	}
}

/* ------------------------------------------------------------------ */
/* 基准：证明 tingo 相对 gin 的额外开销为零                               */
/* ------------------------------------------------------------------ */

func BenchmarkGinContextDirect(b *testing.B) {
	g := &gin.Context{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sinkAny = g
	}
}

func BenchmarkCtxConversion(b *testing.B) {
	g := &gin.Context{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sinkAny = FromGin(g)
	}
}

var sinkAny any
