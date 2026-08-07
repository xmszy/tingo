package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// bindTarget 覆盖 uri / query / body 三个来源，并含默认值。
type bindTarget struct {
	ID   string `uri:"id"`
	Page int    `form:"page,default=1"  json:"-"`
	Size int    `form:"size,default=20" json:"-"`
	Name string `form:"name" json:"name"`
}

// newCtx 构造一个可用于绑定的 Ctx。
func newCtx(method, target, body string, params gin.Params) *Ctx {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	g, _ := gin.CreateTestContext(w)
	if body != "" {
		g.Request = httptest.NewRequest(method, target, strings.NewReader(body))
		g.Request.Header.Set("Content-Type", "application/json")
	} else {
		g.Request = httptest.NewRequest(method, target, nil)
	}
	g.Params = params
	return FromGin(g)
}

// TestBindAllAppliesDefaults 锁定「query 为空时默认值仍生效」这一行为。
//
// 这是一个回归测试：早期实现为省一次绑定而在 RawQuery 为空时跳过 query 绑定，
// 导致 default 标签失效、分页返回空列表。
func TestBindAllAppliesDefaults(t *testing.T) {
	c := newCtx(http.MethodGet, "/users", "", nil)

	var got bindTarget
	if err := c.BindAll(&got); err != nil {
		t.Fatal(err)
	}
	if got.Page != 1 || got.Size != 20 {
		t.Fatalf("defaults not applied: page=%d size=%d", got.Page, got.Size)
	}
}

// TestBindAllQueryOverridesDefaults 验证显式传参覆盖默认值。
func TestBindAllQueryOverridesDefaults(t *testing.T) {
	c := newCtx(http.MethodGet, "/users?page=3&size=5", "", nil)

	var got bindTarget
	if err := c.BindAll(&got); err != nil {
		t.Fatal(err)
	}
	if got.Page != 3 || got.Size != 5 {
		t.Fatalf("query did not override: page=%d size=%d", got.Page, got.Size)
	}
}

// TestBindAllMergesSources 验证 uri、query、body 三来源合并。
func TestBindAllMergesSources(t *testing.T) {
	c := newCtx(http.MethodPost, "/users/7?page=2", `{"name":"ada"}`,
		gin.Params{{Key: "id", Value: "7"}})

	var got bindTarget
	if err := c.BindAll(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "7" {
		t.Errorf("uri not bound: %q", got.ID)
	}
	if got.Page != 2 {
		t.Errorf("query not bound: %d", got.Page)
	}
	if got.Name != "ada" {
		t.Errorf("body not bound: %q", got.Name)
	}
}

// TestBindAllEmptyBodyIsNotAnError 验证 POST 空体不报错。
func TestBindAllEmptyBodyIsNotAnError(t *testing.T) {
	c := newCtx(http.MethodPost, "/users", "", nil)

	var got bindTarget
	if err := c.BindAll(&got); err != nil {
		t.Fatalf("empty body should not fail: %v", err)
	}
	if got.Page != 1 {
		t.Fatalf("defaults should still apply, got page=%d", got.Page)
	}
}
