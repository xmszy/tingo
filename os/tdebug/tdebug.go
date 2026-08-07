// Package tdebug 提供调试工具。
// 设计要点：
//   - 基于标准库 runtime/debug/runtime/pprof，零外部依赖。
//   - 提供堆栈追踪、调用栈打印、goroutine 信息等调试功能。
package tdebug

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Stack 返回当前 goroutine 完整堆栈信息。
func Stack(all ...bool) string {
	if len(all) > 0 && all[0] {
		buf := make([]byte, 4096)
		for {
			n := runtime.Stack(buf, true)
			if n < len(buf) {
				return string(buf[:n])
			}
			buf = make([]byte, len(buf)*2)
		}
	}
	return string(debug.Stack())
}

// PrintStack 打印当前堆栈到标准输出。
func PrintStack() { debug.PrintStack() }

// Caller 返回调用者的函数名、文件和行号。
// skip: 0=Caller自身, 1=调用Caller的函数, 2=上两级...
func Caller(skip ...int) string {
	s := 2
	if len(skip) > 0 {
		s = skip[0] + 1
	}
	pc, file, line, ok := runtime.Caller(s)
	if !ok {
		return "???"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return fmt.Sprintf("%s:%d %s", file, line, fn.Name())
}

// CallerFunc 返回调用者的函数名。
func CallerFunc(skip ...int) string {
	s := 2
	if len(skip) > 0 {
		s = skip[0] + 1
	}
	pc, _, _, ok := runtime.Caller(s)
	if !ok {
		return "???"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "???"
	}
	return fn.Name()
}

// CallerFile 返回调用者的文件名和行号。
func CallerFile(skip ...int) string {
	s := 2
	if len(skip) > 0 {
		s = skip[0] + 1
	}
	_, file, line, ok := runtime.Caller(s)
	if !ok {
		return "???"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// GoroutineID 获取当前 goroutine ID（供调试用，非官方 API）。
func GoroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// "goroutine 123 [running]:..."
	s := string(buf[:n])
	parts := strings.SplitN(s, " ", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return "?"
}

// NumGoroutine 返回当前 goroutine 数量。
func NumGoroutine() int { return runtime.NumGoroutine() }

// FreeOSMemory 触发 GC 释放内存给 OS。
func FreeOSMemory() { debug.FreeOSMemory() }
