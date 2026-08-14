package tapp

import (
	"net/http"
	"testing"

	"github.com/xmszy/tingo/core"
)

// annoCtrl 实现 RouteAnnotated 用于注解路由测试。
type annoCtrl struct {
	Controller
	hits int
}

func (c *annoCtrl) Annotations() []RouteMeta {
	return []RouteMeta{
		{Method: "GET", Path: "/list", Handler: "List"},
	}
}

// List 是注解声明的处理方法。
func (c *annoCtrl) List(ctx *core.Ctx) {
	c.hits++
	ctx.String("ok")
}

// TestRouteMeta 验证注解结构字段。
func TestRouteMeta(t *testing.T) {
	info := RouteMeta{Method: http.MethodGet, Path: "/x", Handler: "X"}
	if info.Method != http.MethodGet || info.Path != "/x" || info.Handler != "X" {
		t.Fatalf("RouteMeta broken: %+v", info)
	}
}

// TestAnnotations 验证控制器返回正确的路由声明。
func TestAnnotations(t *testing.T) {
	c := &annoCtrl{}
	metas := c.Annotations()
	if len(metas) != 1 || metas[0].Path != "/list" || metas[0].Handler != "List" {
		t.Fatalf("Annotations broken: %+v", metas)
	}
}

// TestRegisterAnnotated 验证登记到全局注册表可被遍历。
func TestRegisterAnnotated(t *testing.T) {
	RegisterAnnotated("/user", &annoCtrl{})
	registered := false
	annotatedMu.RLock()
	for _, e := range annotatedRegistry {
		if e.prefix == "/user" {
			registered = true
		}
	}
	annotatedMu.RUnlock()
	if !registered {
		t.Fatal("RegisterAnnotated did not add entry")
	}
}

// TestJoinRoutePath 验证路径拼接规则。
func TestJoinRoutePath(t *testing.T) {
	cases := []struct {
		prefix, p, want string
	}{
		{"/user", "/list", "/list"},    // 绝对路径忽略 prefix
		{"/user", "list", "/user/list"}, // 相对路径拼接
		{"", "list", "/list"},
		{"/user", "", "/user"},
	}
	for _, c := range cases {
		if got := joinRoutePath(c.prefix, c.p); got != c.want {
			t.Fatalf("joinRoutePath(%q,%q) = %q, want %q", c.prefix, c.p, got, c.want)
		}
	}
}
