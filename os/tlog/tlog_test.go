package tlog

import (
	"bytes"
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
