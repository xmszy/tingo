// Package tmode 提供运行模式管理。
// 设计要点：
//   - 基于标准库，零外部依赖。
//   - 支持 dev/test/prod 三种模式切换。
package tmode

import "sync/atomic"

const (
	Dev  = "dev"
	Test = "test"
	Prod = "prod"
)

var mode atomic.Value

func init() { mode.Store(Dev) }

// Set 设置运行模式。
func Set(m string) { mode.Store(m) }

// Get 获取当前运行模式。
func Get() string { return mode.Load().(string) }

// IsDev 是否为开发模式。
func IsDev() bool { return Get() == Dev }

// IsTest 是否为测试模式。
func IsTest() bool { return Get() == Test }

// IsProd 是否为生产模式。
func IsProd() bool { return Get() == Prod }
