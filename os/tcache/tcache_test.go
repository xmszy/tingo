package tcache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xmszy/tingo/core"
)

func TestSetGet(t *testing.T) {
	c := New()
	c.Set("k", "v", 0)
	v, ok := c.Get("k")
	if !ok || v.(string) != "v" {
		t.Fatalf("expected v, got %v ok=%v", v, ok)
	}
}

func TestGetGeneric(t *testing.T) {
	c := New()
	SetT(c, "n", 100, 0)
	n, ok := Get[int](c, "n")
	if !ok || n != 100 {
		t.Fatalf("expected 100, got %v ok=%v", n, ok)
	}
}

func TestExpiry(t *testing.T) {
	c := New()
	c.Set("e", "x", 30*time.Millisecond)
	if !c.Has("e") {
		t.Fatal("should exist immediately")
	}
	time.Sleep(60 * time.Millisecond)
	if c.Has("e") {
		t.Fatal("should have expired")
	}
}

func TestGetOr(t *testing.T) {
	c := New()
	calls := 0
	fn := func() (any, error) { calls++; return "computed", nil }
	v, err := c.GetOr("g", time.Minute, fn)
	if err != nil || v.(string) != "computed" {
		t.Fatalf("unexpected: %v %v", v, err)
	}
	// 第二次应命中缓存，不再次调用 fn。
	v, _ = c.GetOr("g", time.Minute, fn)
	if v.(string) != "computed" || calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDeleteClear(t *testing.T) {
	c := New()
	c.Set("a", 1, 0)
	c.Set("b", 2, 0)
	if c.Len() < 2 {
		t.Fatalf("len wrong: %d", c.Len())
	}
	c.Delete("a")
	if c.Has("a") {
		t.Fatal("delete failed")
	}
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("clear failed: %d", c.Len())
	}
}

func TestConcurrent(t *testing.T) {
	c := New()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(id int) {
			for j := 0; j < 1000; j++ {
				c.Set(string(rune('a'+id))+string(rune('0'+j%10)), id, 0)
				c.Get("x")
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

func TestCacheCloseStopsSweeper(t *testing.T) {
	cache := New(Options{SweepInterval: time.Millisecond})
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerCachesAndIsolatesNamedConnections(t *testing.T) {
	manager, err := NewManager(ManagerConfig{
		Default: "main",
		Connections: map[string]ConnectionConfig{
			"main":   {Driver: "memory"},
			"backup": {Driver: "memory"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ctx := context.Background()
	first, err := manager.Connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Connection(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}
	backup, err := manager.Connection(ctx, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == backup {
		t.Fatalf("named instances are not isolated: %p %p %p", first, second, backup)
	}
	if err := first.Set(ctx, "key", "value", 0); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := backup.Get(ctx, "key"); err != nil || ok {
		t.Fatalf("backup cache leaked value: ok=%v err=%v", ok, err)
	}
}

func TestManagerCreatesConnectionOnceConcurrently(t *testing.T) {
	registry := core.NewDriverRegistry[ConnectionConfig, Driver]()
	var creates atomic.Int32
	if err := registry.Register("counting", func(context.Context, ConnectionConfig) (Driver, error) {
		creates.Add(1)
		return NewMemoryDriver(Options{}), nil
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ManagerConfig{
		Default: "main", Registry: registry,
		Connections: map[string]ConnectionConfig{"main": {Driver: "counting"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := manager.Connection(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if creates.Load() != 1 {
		t.Fatalf("factory calls = %d", creates.Load())
	}
}

func TestServiceBindsManagerToApp(t *testing.T) {
	service, err := NewService(ManagerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	app := core.NewApp()
	if err := app.Register(service); err != nil {
		t.Fatal(err)
	}
	if err := app.Boot(context.Background()); err != nil {
		t.Fatal(err)
	}
	resolved, err := core.Resolve[*Manager](app.Container())
	if err != nil || resolved != service.manager {
		t.Fatalf("resolved manager = %p, err=%v", resolved, err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkCacheSetGet(b *testing.B) {
	c := New()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Set("k", 0, 0)
		c.Get("k")
	}
}
