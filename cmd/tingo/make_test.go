package main

import (
	"go/format"
	"strings"
	"testing"
)

func TestRenderMakeAllTypes(t *testing.T) {
	for _, kind := range []string{"controller", "model", "middleware", "validate", "service"} {
		src := renderMake(kind, "user_order", "index", "example.com/app", "", "")
		if src == "" {
			t.Fatalf("%s produced empty", kind)
		}
		if _, err := format.Source([]byte(src)); err != nil {
			t.Fatalf("%s not gofmt: %v\n%s", kind, err, src)
		}
	}
}

func TestRenderMakeHasStructAndImports(t *testing.T) {
	src := renderMake("controller", "blog", "api", "example.com/app", "", "")
	if !strings.Contains(src, "type Blog struct{}") {
		t.Fatalf("missing struct:\n%s", src)
	}
	if !strings.Contains(src, "func (c *Blog) Index") {
		t.Fatalf("missing Index action:\n%s", src)
	}
	m := renderMake("model", "user", "api", "example.com/app", "", "")
	if !strings.Contains(m, `TableName() string { return "user" }`) {
		t.Fatalf("model missing TableName:\n%s", m)
	}
	if !strings.Contains(m, "func NewUser(connection ...string)") || !strings.Contains(m, "t.Database(connection...)") {
		t.Fatalf("model still requires manual DB wiring:\n%s", m)
	}
}
