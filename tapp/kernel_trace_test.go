package tapp

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/xmszy/tingo/os/ttrace"
)

// TestKernelBootAutoTrace 验证 debug 配置驱动下 Kernel.Boot 自动挂载工具栏。
func TestKernelBootAutoTrace(t *testing.T) {
	// 模拟 frame/t 注入的提供器：启用工具栏，使用默认配置。
	prev := TraceConfigProvider
	TraceConfigProvider = func() (ttrace.Config, bool) {
		return ttrace.Default().Config, true
	}
	defer func() { TraceConfigProvider = prev }()

	k := NewKernel()
	k.Boot(nil)

	found := false
	for _, m := range k.Middlewares() {
		name := runtime.FuncForPC(reflect.ValueOf(m).Pointer()).Name()
		if contains(name, "Toolbar") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("debug=true 时未自动挂载调试工具栏；当前中间件数=%d", len(k.Middlewares()))
	}
}

// TestKernelBootNoTrace 验证未启用时不挂载工具栏。
func TestKernelBootNoTrace(t *testing.T) {
	prev := TraceConfigProvider
	TraceConfigProvider = func() (ttrace.Config, bool) {
		return ttrace.Config{}, false
	}
	defer func() { TraceConfigProvider = prev }()

	k := NewKernel()
	k.Boot(nil)

	for _, m := range k.Middlewares() {
		name := runtime.FuncForPC(reflect.ValueOf(m).Pointer()).Name()
		if contains(name, "Toolbar") {
			t.Fatalf("未启用时不应挂载调试工具栏")
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
