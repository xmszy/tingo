package rate

import (
	"testing"
	"time"
)

func TestAllowBurst(t *testing.T) {
	l := New(10, 5) // 5 burst, 10/s
	// 突发前 5 个应通过。
	for i := 0; i < 5; i++ {
		if !l.Allow("ip1") {
			t.Fatalf("request %d should pass", i)
		}
	}
	if l.Allow("ip1") {
		t.Fatal("6th request should be limited")
	}
	// 等待补充后应再次通过。
	time.Sleep(200 * time.Millisecond)
	if !l.Allow("ip1") {
		t.Fatal("after refill should pass")
	}
}

func TestDifferentKeys(t *testing.T) {
	l := New(1, 1)
	if !l.Allow("a") {
		t.Fatal("a should pass")
	}
	if !l.Allow("b") {
		t.Fatal("b should pass (separate bucket)")
	}
}

func TestMiddlewareType(t *testing.T) {
	// Middleware 必须返回可注册到引擎的 core.Handler（编译期检查）。
	var h = Middleware(10, 5)
	if h == nil {
		t.Fatal("middleware should not be nil")
	}
}
