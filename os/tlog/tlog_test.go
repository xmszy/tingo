package tlog

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestBasicLevels(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelDebug, Flags: FStd})
	l.Debug("hello")
	l.Info("world")
	if !strings.Contains(buf.String(), "hello") || !strings.Contains(buf.String(), "world") {
		t.Fatalf("output missing: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "DEBUG") || !strings.Contains(buf.String(), "INFO") {
		t.Fatalf("level missing: %q", buf.String())
	}
}

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelWarn, Flags: FStd})
	l.Info("skipped")
	l.Warn("shown")
	out := buf.String()
	if strings.Contains(out, "skipped") {
		t.Fatal("info should be filtered")
	}
	if !strings.Contains(out, "shown") {
		t.Fatal("warn should show")
	}
}

func TestFields(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelInfo, Flags: 0})
	l.Infow("event", F("user", "ada"), F("code", 200))
	out := buf.String()
	if !strings.Contains(out, "user=ada") || !strings.Contains(out, "code=200") {
		t.Fatalf("fields missing: %q", out)
	}
}

func TestCaller(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelInfo, Flags: FFile | FFunc})
	l.Info("with caller")
	out := buf.String()
	if !strings.Contains(out, "tlog_test.go") {
		t.Fatalf("file not shown: %q", out)
	}
}

func TestAsync(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelInfo, Flags: 0, Async: true, AsyncBuffer: 16})
	l.Info("async msg")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "async msg") {
		t.Fatalf("async message lost: %q", buf.String())
	}
}

// TestHook 验证 AddHook 在日志写出后被回调，且 Clone 继承钩子。
func TestHook(t *testing.T) {
	var buf bytes.Buffer
	var got []string
	l := NewWithConfig(Config{Writer: &buf, Level: LevelInfo, Flags: 0})
	l.AddHook(HookFunc(func(e entry) error {
		got = append(got, e.msg)
		return nil
	}))
	l.Info("hooked")
	if len(got) != 1 || got[0] != "hooked" {
		t.Fatalf("hook not invoked: %v", got)
	}
	// Clone 继承钩子
	sub := l.Clone()
	sub.Warn("cloned")
	if len(got) != 2 || got[1] != "cloned" {
		t.Fatalf("hook not inherited by clone: %v", got)
	}
}

// TestRotatingWriter 验证超过阈值后发生轮转（当前文件 -> .1）。
func TestRotatingWriter(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/app.log"
	w, err := NewRotatingWriter(path, 32, 3) // 阈值 32 字节
	if err != nil {
		t.Fatal(err)
	}
	// 写入超过阈值，应触发轮转产生 .1
	for i := 0; i < 3; i++ {
		_, _ = w.Write([]byte("hello world\n")) // 12 字节，第三次累计 > 32
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated backup .1: %v", err)
	}
}

func BenchmarkLogSync(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelInfo, Flags: FStd})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		l.Infow("req", F("path", "/x"), F("ms", 3))
	}
}

func BenchmarkLogAsync(b *testing.B) {
	var buf bytes.Buffer
	l := NewWithConfig(Config{Writer: &buf, Level: LevelInfo, Flags: FStd, Async: true, AsyncBuffer: 4096})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		l.Infow("req", F("path", "/x"), F("ms", 3))
	}
	_ = l.Close()
}
