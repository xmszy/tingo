package core

import (
	"testing"
)

/* ---- 基础 Bind/Resolve ---- */

type svcA struct{ v int }

func TestContainerBindResolve(t *testing.T) {
	c := NewContainer()
	Bind(c, func(*Container) (*svcA, error) {
		return &svcA{v: 42}, nil
	})
	got, err := Resolve[*svcA](c)
	if err != nil {
		t.Fatal(err)
	}
	if got.v != 42 {
		t.Fatalf("got %d", got.v)
	}
}

func TestContainerSingleton(t *testing.T) {
	c := NewContainer()
	Bind(c, func(*Container) (*svcA, error) {
		return &svcA{}, nil
	})
	a1, _ := Resolve[*svcA](c)
	a2, _ := Resolve[*svcA](c)
	if a1 != a2 {
		t.Fatal("singleton expected same instance")
	}
}

func TestContainerTransient(t *testing.T) {
	c := NewContainer()
	BindTransient(c, func(*Container) (*svcA, error) {
		return &svcA{}, nil
	})
	a1, _ := Resolve[*svcA](c)
	a2, _ := Resolve[*svcA](c)
	if a1 == a2 {
		t.Fatal("transient expected different instances")
	}
}

/* ---- 接口绑定 ---- */

type Repo interface{ Name() string }
type mysqlRepo struct{}

func (mysqlRepo) Name() string { return "mysql" }

func TestContainerBindInterface(t *testing.T) {
	c := NewContainer()
	BindInterface[Repo](c, func(*Container) (*mysqlRepo, error) {
		return &mysqlRepo{}, nil
	})
	r, err := Resolve[Repo](c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Name() != "mysql" {
		t.Fatalf("unexpected repo: %v", r)
	}
}

/* ---- 命名绑定 ---- */

func TestContainerNamed(t *testing.T) {
	c := NewContainer()
	BindNamed(c, "x", func(*Container) (*svcA, error) {
		return &svcA{v: 1}, nil
	})
	BindNamed(c, "y", func(*Container) (*svcA, error) {
		return &svcA{v: 2}, nil
	})
	x, err := ResolveNamed[*svcA](c, "x")
	if err != nil || x.v != 1 {
		t.Fatalf("named x: %v %d", err, x.v)
	}
	y, err := ResolveNamed[*svcA](c, "y")
	if err != nil || y.v != 2 {
		t.Fatalf("named y: %v %d", err, y.v)
	}
	// 未命中命名时回退到无名称绑定。
	Bind(c, func(*Container) (*svcA, error) { return &svcA{v: 9}, nil })
	def, err := ResolveNamed[*svcA](c, "missing")
	if err != nil || def.v != 9 {
		t.Fatalf("named fallback: %v %d", err, def.v)
	}
	if !HasNamed[*svcA](c, "x") || HasNamed[*svcA](c, "nope") {
		t.Fatal("HasNamed incorrect")
	}
}

func TestContainerScopeOverridesParent(t *testing.T) {
	c := NewContainer()
	Bind(c, func(*Container) (*svcA, error) { return &svcA{v: 1}, nil })
	s := c.NewScope()
	Bind(s, func(*Container) (*svcA, error) { return &svcA{v: 2}, nil })
	child, _ := Resolve[*svcA](s)
	if child.v != 2 {
		t.Fatalf("scope override failed: %d", child.v)
	}
	parent, _ := Resolve[*svcA](c)
	if parent.v != 1 {
		t.Fatalf("parent unchanged: %d", parent.v)
	}
}
