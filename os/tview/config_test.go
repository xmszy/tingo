package tview

import (
	"testing"

	"github.com/xmszy/tingo/os/tcfg"
)

func TestConfigFromTree(t *testing.T) {
	cfg := ConfigFromTree(tcfg.Tree{"view": map[string]any{
		"root": "template", "extension": ".gohtml", "left_delim": "[[", "right_delim": "]]",
	}})
	if cfg.Root != "template" || cfg.Extension != ".gohtml" || cfg.LeftDelim != "[[" || cfg.RightDelim != "]]" {
		t.Fatalf("config = %+v", cfg)
	}
}
