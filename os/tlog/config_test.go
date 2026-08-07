package tlog

import (
	"testing"

	"github.com/xmszy/tingo/os/tcfg"
)

func TestConfigFromTree(t *testing.T) {
	cfg, err := ConfigFromTree(tcfg.Tree{"log": map[string]any{
		"level": "debug", "async": true, "async_buffer": 64,
		"prefix": "demo", "flags": []any{"time", "level", "file"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Level != LevelDebug || !cfg.Async || cfg.AsyncBuffer != 64 || cfg.Prefix != "demo" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Flags != FTime|FLevel|FFile {
		t.Fatalf("flags = %v", cfg.Flags)
	}
}

func TestConfigFromTreeRejectsInvalidLevel(t *testing.T) {
	if _, err := ConfigFromTree(tcfg.Tree{"log": map[string]any{"level": "verbose"}}); err == nil {
		t.Fatal("invalid level was accepted")
	}
}
