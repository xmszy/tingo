// Package ttrace 提供运行期调试工具栏。
//
// 在 debug 模式下自动把调试面板注入 HTML 页面底部：右下角常驻一行「耗时」黑条，
// 点击后弹出完整的调试面板。面板按固定分区组织（基本 / 文件 / 流程 / 错误 / SQL / 调试）。
//
// 用法：
//
//	// 方式一：tingo 原生中间件（推荐）
//	engine.Use(ttrace.Default().Handler())
//	// 或门面一行：
//	t.Use(t.EnableToolbar())
//
//	// 方式二：net/http 标准中间件
//	handler := ttrace.Default().Middleware()(mux)
//
// 业务可通过 ttrace.Trace("info", "...") 把调试信息推送到「调试」面板。
package ttrace

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ── 配置 ─────────────────────────────────────────────────────────

// Config 工具栏配置。
//
//	Type    : "Html"（页面注入）或 "Console"（命令行输出）
//	Channel : 日志通道名（仅在 Console 模式相关）
//	Panels  : 各分区显隐开关（默认全 true）
type Config struct {
	Type    string `toml:"type" json:"type"`
	Channel string `toml:"channel" json:"channel"`
	Panels  Panels `toml:"panels" json:"panels"`
}

// Panels 控制工具栏各分区的显隐：
//
//	base   : 基本（请求/内存/时间戳）
//	file   : 文件（本次加载的文件）
//	info   : 流程（运行时间线）
//	error  : 错误/警告（渲染错误与 panic）
//	sql    : SQL 查询
//	log    : 调试（业务通过 Trace() 记录的内容）
type Panels struct {
	Base  bool `toml:"base" json:"base"`
	File  bool `toml:"file" json:"file"`
	Info  bool `toml:"info" json:"info"`
	Error bool `toml:"error" json:"error"`
	SQL   bool `toml:"sql" json:"sql"`
	Log   bool `toml:"log" json:"log"`
}

// Toolbar 调试工具栏。
type Toolbar struct {
	Config    Config
	startTime time.Time
	skipPaths []string // 不注入的路径前缀
}

// Default 返回默认工具栏（Html 类型、所有分区启用）。
func Default() *Toolbar {
	return NewWithConfig(Config{
		Type:    "Html",
		Channel: "trace",
		Panels: Panels{
			Base:  true,
			File:  true,
			Info:  true,
			Error: true,
			SQL:   true,
			Log:   true,
		},
	})
}

// NewWithConfig 用指定配置构造工具栏。
func NewWithConfig(cfg Config) *Toolbar {
	return &Toolbar{
		Config:    cfg,
		startTime: time.Now(),
	}
}

// SkipPaths 设置不注入工具栏的路径前缀（如 /api /static /debug）。
func (tb *Toolbar) SkipPaths(paths ...string) {
	tb.skipPaths = paths
}

// IsEnabled 判断工具栏是否启用。
func (tb *Toolbar) IsEnabled() bool {
	return tb.Config.Type != "" && tb.Config.Type != "false"
}

// ── 全局收集器 ─────────────────────────────────────────────────────
//
// 下列收集器在运行期累计信息，工具栏在请求结束时读取并渲染到对应分区。

// SQLRecord 一条 SQL 查询记录。
type SQLRecord struct {
	SQL      string
	Duration time.Duration
	Time     time.Time
}

// ErrorRecord 一条捕获到的错误/警告记录。
type ErrorRecord struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Trace   string    `json:"trace,omitempty"`
}

var (
	sqlMu     sync.Mutex
	sqlBuffer []SQLRecord

	errMu     sync.Mutex
	errBuffer []ErrorRecord

	logMu     sync.Mutex
	logBuffer = map[string][]string{} // 分区 -> 行
)

// LogSQL 记录一条 SQL 查询（供 tdb 等组件调用）。
func LogSQL(sql string, duration time.Duration) {
	sqlMu.Lock()
	defer sqlMu.Unlock()
	sqlBuffer = append(sqlBuffer, SQLRecord{SQL: sql, Duration: duration, Time: time.Now()})
	if len(sqlBuffer) > 200 {
		sqlBuffer = sqlBuffer[len(sqlBuffer)-100:]
	}
}

// GetSQL 获取当前所有已记录的 SQL 查询。
func GetSQL() []SQLRecord {
	sqlMu.Lock()
	defer sqlMu.Unlock()
	out := make([]SQLRecord, len(sqlBuffer))
	copy(out, sqlBuffer)
	return out
}

// ClearSQL 清空 SQL 记录（请求开始时调用）。
func ClearSQL() {
	sqlMu.Lock()
	defer sqlMu.Unlock()
	sqlBuffer = nil
}

// LogError 记录一条错误（供 Error 分区展示）。
func LogError(message string) {
	errMu.Lock()
	defer errMu.Unlock()
	errBuffer = append(errBuffer, ErrorRecord{Time: time.Now(), Message: message})
	if len(errBuffer) > 50 {
		errBuffer = errBuffer[len(errBuffer)-25:]
	}
}

// AddPanic 记录一个 panic 值及其堆栈。
func AddPanic(v any, stack []byte) {
	msg := ""
	if v != nil {
		if e, ok := v.(interface{ Error() string }); ok {
			msg = e.Error()
		} else {
			msg = fmt.Sprintf("%v", v)
		}
	}
	trace := ""
	if len(stack) > 0 {
		trace = string(stack)
		if len(trace) > 1200 {
			trace = trace[:1200] + "…"
		}
	}
	errMu.Lock()
	defer errMu.Unlock()
	errBuffer = append(errBuffer, ErrorRecord{Time: time.Now(), Message: msg, Trace: trace})
	if len(errBuffer) > 50 {
		errBuffer = errBuffer[len(errBuffer)-25:]
	}
}

// GetErrors 获取当前所有已记录的错误。
func GetErrors() []ErrorRecord {
	errMu.Lock()
	defer errMu.Unlock()
	out := make([]ErrorRecord, len(errBuffer))
	copy(out, errBuffer)
	return out
}

// ClearErrors 清空错误记录（请求开始时调用）。
func ClearErrors() {
	errMu.Lock()
	defer errMu.Unlock()
	errBuffer = nil
}

// Trace 写入一条调试记录到「调试」分区。
//
// 业务可通过 ttrace.Trace("用户登录 uid=1", "info") 把自定义信息推送到面板。
// level 为日志级别（info/error/debug/sql...），用作分区内分组标签。
func Trace(msg string, level string) {
	if level == "" {
		level = "log"
	}
	logMu.Lock()
	defer logMu.Unlock()
	logBuffer[level] = append(logBuffer[level], msg)
	if len(logBuffer[level]) > 200 {
		logBuffer[level] = logBuffer[level][len(logBuffer[level])-100:]
	}
}

// Record 是 Trace 的别名，便于业务层沿用旧写法。
func Record(msg string, level string) { Trace(msg, level) }

// GetTrace 返回某级别下的所有调试记录。
func GetTrace(level string) []string {
	logMu.Lock()
	defer logMu.Unlock()
	out := make([]string, len(logBuffer[level]))
	copy(out, logBuffer[level])
	return out
}

// AllTrace 返回全部分区记录（级别 -> 行）。
func AllTrace() map[string][]string {
	logMu.Lock()
	defer logMu.Unlock()
	out := make(map[string][]string, len(logBuffer))
	for k, v := range logBuffer {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// ClearTrace 清空调试记录（请求开始时调用）。
func ClearTrace() {
	logMu.Lock()
	defer logMu.Unlock()
	logBuffer = map[string][]string{}
}

// memStats 读取当前内存统计（base 分区使用）。
func memStats() runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}
