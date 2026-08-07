package tsession

import (
	"testing"
	"time"

	"github.com/xmszy/tingo/os/tcfg"
)

func TestConfigFromTree(t *testing.T) {
	cfg, err := ConfigFromTree(tcfg.Tree{"session": map[string]any{
		"name": "sid", "expire": "2h", "cookie_path": "/api", "secure": true, "http_only": false,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieName != "sid" || cfg.TTL != 2*time.Hour || cfg.CookiePath != "/api" || !cfg.Secure || cfg.HttpOnly {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestConfigFromTreeRejectsInvalidExpire(t *testing.T) {
	if _, err := ConfigFromTree(tcfg.Tree{"session": map[string]any{"expire": "later"}}); err == nil {
		t.Fatal("invalid expire was accepted")
	}
}
