package registry

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r := NewFile(path)

	a := Instance{Name: "user-svc", Address: "10.0.0.1:8080", Weight: 1}
	b := Instance{Name: "user-svc", Address: "10.0.0.2:8080", Weight: 1}
	if err := r.Register(a); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(b); err != nil {
		t.Fatal(err)
	}
	list, err := r.Discover("user-svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(list))
	}
	// 注销一个。
	if err := r.Deregister(a); err != nil {
		t.Fatal(err)
	}
	list, _ = r.Discover("user-svc")
	if len(list) != 1 {
		t.Fatalf("expected 1 after deregister, got %d", len(list))
	}
}

func TestFileRegistryTTLExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r := NewFile(path)
	inst := Instance{Name: "svc", Address: "1.2.3.4:80", TTL: 50 * time.Millisecond}
	if err := r.Register(inst); err != nil {
		t.Fatal(err)
	}
	// 立即发现应在 TTL 内。
	if list, _ := r.Discover("svc"); len(list) != 1 {
		t.Fatalf("should be alive immediately, got %d", len(list))
	}
	time.Sleep(80 * time.Millisecond)
	// 超过 TTL 后应被过滤（registeredAt 已是 80ms 前）。
	if list, _ := r.Discover("svc"); len(list) != 0 {
		t.Fatalf("should expire after TTL, got %d", len(list))
	}
}
