package tcron

import (
	"testing"
	"time"
)

func TestParseBasic(t *testing.T) {
	s := New(time.UTC)
	err := s.Add("t", "*/5 * * * *", func() {})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.tasks["t"].fields[0]) == 0 {
		t.Fatal("minute field empty")
	}
}

func TestParseInvalid(t *testing.T) {
	s := New(time.UTC)
	if err := s.Add("x", "bad spec here", func() {}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNextTime(t *testing.T) {
	s := New(time.UTC)
	_ = s.Add("t", "0 * * * *", func() {}) // 每小时整点
	base := time.Date(2026, 8, 1, 10, 3, 0, 0, time.UTC)
	got := nextTime(s.tasks["t"], base)
	want := time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMatchDaily(t *testing.T) {
	s := New(time.UTC)
	_ = s.Add("t", "30 9 * * *", func() {})
	tk := s.tasks["t"]
	// 9:30 应命中，9:31 不命中
	if !match(tk, time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)) {
		t.Fatal("9:30 should match")
	}
	if match(tk, time.Date(2026, 8, 1, 9, 31, 0, 0, time.UTC)) {
		t.Fatal("9:31 should not match")
	}
}

func TestFire(t *testing.T) {
	s := New(time.UTC)
	fired := 0
	_ = s.Add("t", "* * * * *", func() { fired++ })
	// 手动驱动：设置 next 为过去，调用 fireLocked 应触发一次。
	tk := s.tasks["t"]
	tk.next = time.Now().Add(-time.Minute)
	s.fireLocked(time.Now())
	if fired != 1 {
		t.Fatalf("expected 1 fire, got %d", fired)
	}
}
