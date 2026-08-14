package tapp

import "testing"

// TestBaseImplementsInterfaces 验证 Base 满足 core 的应用接口且零值可用。
func TestBaseImplementsInterfaces(t *testing.T) {
	var b Base
	// 零值默认返回空中间件列表，是合理的空实现。
	if len(b.Middlewares()) != 0 {
		t.Fatal("Base.Middlewares() should be empty")
	}
	// Config 返回非 nil 的零值 AppConfig。
	_ = b.Config()
}

// TestBaseBoot 验证 Base.Boot 默认空实现返回 nil。
func TestBaseBoot(t *testing.T) {
	var b Base
	if err := b.Boot(); err != nil {
		t.Fatalf("Base.Boot should be nil, got %v", err)
	}
}
