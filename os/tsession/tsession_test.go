package tsession

import (
	"testing"
	"time"
)

func TestMemoryRoundTrip(t *testing.T) {
	m := New(Config{})
	s, err := m.LoadOrCreate("")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" {
		t.Fatal("empty id")
	}
	s.Set("uid", 123)
	s.Set("name", "tom")
	if err := m.Save(s); err != nil {
		t.Fatal(err)
	}
	// 重新载入
	s2, err := m.LoadOrCreate(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	uid, ok := Get[int](s2, "uid")
	if !ok || uid != 123 {
		t.Fatalf("uid=%d ok=%v", uid, ok)
	}
	name, _ := s2.Get("name")
	if name != "tom" {
		t.Fatalf("name=%v", name)
	}
}

func TestDestroy(t *testing.T) {
	m := New(Config{})
	s, _ := m.LoadOrCreate("")
	s.Set("k", "v")
	_ = m.Save(s)
	if err := m.Destroy(s); err != nil {
		t.Fatal(err)
	}
	s2, _ := m.LoadOrCreate(s.ID)
	if v, ok := s2.Get("k"); ok || v != nil {
		t.Fatal("session should be destroyed")
	}
}

func TestTTL(t *testing.T) {
	m := New(Config{TTL: time.Hour})
	if m.Config().TTL != time.Hour {
		t.Fatal("ttl not set")
	}
}
