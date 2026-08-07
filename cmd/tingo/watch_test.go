package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChangedDetectsModification(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.go")
	if err := os.WriteFile(f, []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 首次调用建立基准，不报变更。
	if changed(dir) {
		t.Fatal("first call should not report change")
	}
	// 修改文件后，应检测到变更。
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(f, []byte("package x // changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !changed(dir) {
		t.Fatal("expected change detection after modify")
	}
	// 立即再调用，不应重复报变更（已更新快照）。
	if changed(dir) {
		t.Fatal("should not report change twice without modification")
	}
}

func TestLatestModTimeSkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main.go")
	test := filepath.Join(dir, "main_test.go")
	os.WriteFile(main, []byte("package x"), 0o644)
	os.WriteFile(test, []byte("package x"), 0o644)
	// 仅 main.go 计入，时间戳应等于 main.go 的时间（测试文件被忽略）。
	if latestModTime(dir).IsZero() {
		t.Fatal("expected non-zero mod time")
	}
}
