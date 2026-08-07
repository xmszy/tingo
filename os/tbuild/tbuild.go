// Package tbuild 提供编译时注入的构建信息。
//
// 通过 ldflags 注入：
//
//	go build -ldflags="
//	  -X github.com/xmszy/tingo/os/tbuild.Version=v1.0.0
//	  -X github.com/xmszy/tingo/os/tbuild.GitCommit=abc123
//	  -X github.com/xmszy/tingo/os/tbuild.BuildTime=2024-01-01T00:00:00Z
//	"
//
// 运行时直接访问 tbuild.Version 等变量。
package tbuild

import (
	"fmt"
	"runtime"
)

// 编译时注入的版本信息。
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// FullVersion 返回完整的版本字符串。
func FullVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s, go: %s)", Version, GitCommit, BuildTime, runtime.Version())
}

// ShortVersion 返回简短版本字符串。
func ShortVersion() string {
	return fmt.Sprintf("v%s", Version)
}

// Info 返回所有构建信息的 map。
func Info() map[string]string {
	return map[string]string{
		"version":   Version,
		"gitCommit": GitCommit,
		"buildTime": BuildTime,
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	}
}
