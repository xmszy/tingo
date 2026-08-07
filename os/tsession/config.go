package tsession

import (
	"fmt"
	"time"

	"github.com/xmszy/tingo/os/tcfg"
)

// ConfigFromTree 从 session 命名空间构造会话配置。
func ConfigFromTree(tree tcfg.Reader) (Config, error) {
	cfg := Config{
		CookieName: "tingo_session",
		TTL:        24 * time.Hour,
		CookiePath: "/",
		HttpOnly:   true,
		HttpOnlySet: true, // 由配置树构造，意图已显式确定
	}
	if tree.Has("session.name") {
		cfg.CookieName = tree.String("session.name", cfg.CookieName)
	}
	if tree.Has("session.expire") {
		duration, err := time.ParseDuration(tree.String("session.expire"))
		if err != nil {
			return Config{}, fmt.Errorf("tsession: invalid session.expire: %w", err)
		}
		cfg.TTL = duration
	}
	if tree.Has("session.cookie_path") {
		cfg.CookiePath = tree.String("session.cookie_path", cfg.CookiePath)
	}
	if tree.Has("session.secure") {
		cfg.Secure = tree.Bool("session.secure")
	}
	if tree.Has("session.http_only") {
		cfg.HttpOnly = tree.Bool("session.http_only")
	}
	return cfg, nil
}

// NewFromTree 从配置树创建会话管理器。
func NewFromTree(tree tcfg.Reader) (*Manager, error) {
	cfg, err := ConfigFromTree(tree)
	if err != nil {
		return nil, err
	}
	return New(cfg), nil
}
