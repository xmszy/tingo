// Package tlog 提供轻量、高性能的结构化日志。
//
// 设计要点：
//   - 零外部依赖（不引入 fatih/color、otel 等），仅用标准库；
//   - 默认同步写 os.Stderr，可切换为异步（channel + 后台 goroutine），
//     异步模式下热路径仅一次 channel 发送，不阻塞业务；
//   - 支持分级（Debug/Info/Warn/Error/Fatal）、结构化字段（key-value）、
//     caller 行号、调用函数名、时间格式化；
//   - 通过泛型 Get[T]/Set 可零成本注入到 Context 或在请求间复用。
package tlog

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level 是日志级别。
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String 返回级别名。
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "???"
	}
}

// 默认时间格式：2006-01-02T15:04:05.000Z07:00
const defaultTimeFormat = "2006-01-02T15:04:05.000Z07:00"

// Config 是日志配置。
type Config struct {
	// Writer 是底层写入目标，默认 os.Stderr。
	Writer io.Writer
	// Level 是最低输出级别，低于该级别的日志被丢弃。
	Level Level
	// Async 开启异步写入（后台 goroutine 消费 channel）。
	Async bool
	// AsyncBuffer 是异步 channel 容量，默认 1024。
	AsyncBuffer int
	// Flags 控制附加信息（时间/文件行/调用函数）。
	Flags Flag
	// TimeFormat 覆盖默认时间格式。
	TimeFormat string
	// Prefix 每行前缀标签。
	Prefix string
	// File 指定日志文件路径。非空时自动启用基于文件大小的轮转写入器
	// （通过 RotatingWriter，配合 RotateMaxSize/RotateMaxBackups）。
	// 设置 File 后，Writer 字段被忽略。
	File string
	// RotateMaxSize 是单日志文件大小上限（字节），<=0 表示 100MB。
	RotateMaxSize int64
	// RotateMaxBackups 是保留的备份文件数，<=0 表示不限制。
	RotateMaxBackups int
}

// Flag 控制附加输出信息位。
type Flag int

const (
	// FTime 输出时间。
	FTime Flag = 1 << iota
	// FFile 输出短文件名与行号（file.go:23）。
	FFile
	// FFunc 输出调用函数名。
	FFunc
	// FLevel 输出级别名。
	FLevel
	// FStd 默认组合：时间 + 级别。
	FStd = FTime | FLevel
)

// DefaultConfig 返回合理的默认配置。
func DefaultConfig() Config {
	return Config{
		Writer:      os.Stderr,
		Level:       LevelInfo,
		Async:       false,
		AsyncBuffer: 1024,
		Flags:       FStd,
		TimeFormat:  defaultTimeFormat,
	}
}

// Hook 是日志钩子，每条日志写出前会依次回调（同步、不阻塞主写入路径太久）。
// 可用于转发到外部系统（如 Kafka、ES）、附加公共字段、按级别告警等。
type Hook interface {
	// OnLog 在日志格式化写出后调用。返回的错误会被忽略（避免影响主流程）。
	OnLog(e entry) error
}

// HookFunc 是把函数适配为 Hook 的便捷类型。
type HookFunc func(e entry) error

// OnLog 实现 Hook 接口。
func (f HookFunc) OnLog(e entry) error { return f(e) }

// Logger 是日志实例。可克隆派生（链式配置），派生实例共享底层 writer。
type Logger struct {
	cfg    Config
	mu     sync.Mutex // 保护 cfg 字段读写
	wmu    sync.Mutex // 保护底层 writer 写入顺序（与 Close 隔离）
	ch     chan entry // 异步 channel
	done   chan struct{}
	closed bool
	hooks  []Hook
	hmu    sync.RWMutex // 保护 hooks 读写
	rotateCloser io.Closer // 文件轮转写入器（若有），Close 时一并关闭
}

type entry struct {
	level  Level
	time   time.Time
	file   string
	line   int
	fn     string
	msg    string
	fields []Field
}

// Field 是结构化键值对。
type Field struct {
	Key   string
	Value any
}

// F 构造一个结构化字段。
func F(key string, value any) Field { return Field{Key: key, Value: value} }

// New 创建 logger。
func New() *Logger { return &Logger{cfg: DefaultConfig()} }

// NewWithConfig 用自定义配置创建 logger。
func NewWithConfig(c Config) *Logger {
	l := &Logger{cfg: c}
	if l.cfg.Writer == nil {
		l.cfg.Writer = os.Stderr
	}
	if l.cfg.TimeFormat == "" {
		l.cfg.TimeFormat = defaultTimeFormat
	}
	// 配置了 File 时，自动启用基于文件大小的轮转写入器（忽略 Writer 字段）。
	if l.cfg.File != "" {
		rw, err := NewRotatingWriter(l.cfg.File, l.cfg.RotateMaxSize, l.cfg.RotateMaxBackups)
		if err != nil {
			// 打开失败不应让进程崩溃：降级为 stderr 并附带提示。
			fmt.Fprintf(os.Stderr, "tlog: open rotate file %q failed: %v, fallback to stderr\n", l.cfg.File, err)
		} else {
			l.cfg.Writer = rw
			l.rotateCloser = rw
		}
	}
	if l.cfg.Async {
		l.startAsync()
	}
	return l
}

// Clone 浅拷贝配置，用于派生带不同级别/前缀的子 logger。继承钩子列表。
func (l *Logger) Clone() *Logger {
	n := &Logger{cfg: l.cfg, hooks: l.snapshotHooks()}
	if l.cfg.Async {
		n.startAsync()
	}
	return n
}

// snapshotHooks 返回当前钩子列表的快照（读锁保护）。
func (l *Logger) snapshotHooks() []Hook {
	l.hmu.RLock()
	defer l.hmu.RUnlock()
	if len(l.hooks) == 0 {
		return nil
	}
	h := make([]Hook, len(l.hooks))
	copy(h, l.hooks)
	return h
}

// SetLevel 设置最低输出级别（链式）。
func (l *Logger) SetLevel(lv Level) *Logger {
	l.mu.Lock()
	l.cfg.Level = lv
	l.mu.Unlock()
	return l
}

// SetPrefix 设置前缀标签（链式）。
func (l *Logger) SetPrefix(p string) *Logger {
	l.mu.Lock()
	l.cfg.Prefix = p
	l.mu.Unlock()
	return l
}

// SetWriter 切换底层 writer（链式）。
func (l *Logger) SetWriter(w io.Writer) *Logger {
	l.mu.Lock()
	l.cfg.Writer = w
	l.mu.Unlock()
	return l
}

// SetFlags 设置附加信息位（链式）。
func (l *Logger) SetFlags(f Flag) *Logger {
	l.mu.Lock()
	l.cfg.Flags = f
	l.mu.Unlock()
	return l
}

// AddHook 注册一个日志钩子。钩子在每条日志格式化写出后同步回调。
// 派生（Clone/With）的 logger 会继承父级的钩子列表。
func (l *Logger) AddHook(h Hook) *Logger {
	l.hmu.Lock()
	l.hooks = append(l.hooks, h)
	l.hmu.Unlock()
	return l
}

func (l *Logger) startAsync() {
	buf := l.cfg.AsyncBuffer
	if buf <= 0 {
		buf = 1024
	}
	l.ch = make(chan entry, buf)
	l.done = make(chan struct{})
	go l.consume()
}

func (l *Logger) consume() {
	for e := range l.ch {
		l.write(e)
	}
	close(l.done)
}

// Close 关闭 logger，刷新异步缓冲并关闭底层轮转写入器。
func (l *Logger) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	if l.cfg.Async && l.ch != nil {
		close(l.ch)
		l.mu.Unlock()
		<-l.done // 等待消费 goroutine 退出（其写入使用独立 wmu，不会与此处死锁）
	} else {
		l.closed = true
		l.mu.Unlock()
	}
	var result error
	if l.rotateCloser != nil {
		result = l.rotateCloser.Close()
	}
	return result
}

// log 是核心写入路径。
func (l *Logger) log(lv Level, depth int, msg string, fields []Field) {
	if lv < l.cfg.Level {
		return
	}
	e := entry{level: lv, time: time.Now(), msg: msg, fields: fields}
	if l.cfg.Flags&(FFile|FFunc) != 0 {
		// +2: log -> public method -> caller
		if pc, file, line, ok := runtime.Caller(depth); ok {
			if l.cfg.Flags&FFile != 0 {
				e.file = filepath.Base(file)
				e.line = line
			}
			if l.cfg.Flags&FFunc != 0 {
				fn := runtime.FuncForPC(pc)
				if fn != nil {
					name := fn.Name()
					if i := strings.LastIndex(name, "."); i >= 0 {
						name = name[i+1:]
					}
					e.fn = name
				}
			}
		}
	}
	if l.cfg.Async {
		// 热路径：仅一次非阻塞发送（缓冲未满时）。
		select {
		case l.ch <- e:
		default:
			// 缓冲满时降级为同步，保证不丢日志。
			l.write(e)
		}
		return
	}
	l.write(e)
}

func (l *Logger) write(e entry) {
	var b strings.Builder
	if l.cfg.Prefix != "" {
		b.WriteString(l.cfg.Prefix)
		b.WriteByte(' ')
	}
	if l.cfg.Flags&FTime != 0 {
		b.WriteString(e.time.Format(l.cfg.TimeFormat))
		b.WriteByte(' ')
	}
	if l.cfg.Flags&FLevel != 0 {
		b.WriteString(e.level.String())
		b.WriteByte(' ')
	}
	if l.cfg.Flags&FFunc != 0 && e.fn != "" {
		b.WriteString(e.fn)
		b.WriteString("() ")
	}
	if l.cfg.Flags&FFile != 0 && e.file != "" {
		fmt.Fprintf(&b, "%s:%d ", e.file, e.line)
	}
	b.WriteString(e.msg)
	if len(e.fields) > 0 {
		b.WriteByte(' ')
		for i, f := range e.fields {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%s=%v", f.Key, f.Value)
		}
	}
	b.WriteByte('\n')

	l.wmu.Lock()
	_, _ = io.WriteString(l.cfg.Writer, b.String())
	l.wmu.Unlock()

	// 钩子：同步回调（忽略错误，避免影响主流程）。
	if hs := l.snapshotHooks(); len(hs) > 0 {
		for _, h := range hs {
			_ = h.OnLog(e)
		}
	}
}

// Debug / Info / Warn / Error / Fatal —— 接收任意数量参数。
func (l *Logger) Debug(args ...any) { l.log(LevelDebug, 2, fmt.Sprint(args...), nil) }
func (l *Logger) Info(args ...any)  { l.log(LevelInfo, 2, fmt.Sprint(args...), nil) }
func (l *Logger) Warn(args ...any)  { l.log(LevelWarn, 2, fmt.Sprint(args...), nil) }
func (l *Logger) Error(args ...any) { l.log(LevelError, 2, fmt.Sprint(args...), nil) }
func (l *Logger) Fatal(args ...any) {
	l.log(LevelFatal, 2, fmt.Sprint(args...), nil)
	_ = l.Close()
	os.Exit(1)
}

// Debugf / Infof / Warnf / Errorf / Fatalf —— 格式化。
func (l *Logger) Debugf(format string, args ...any) {
	l.log(LevelDebug, 2, fmt.Sprintf(format, args...), nil)
}
func (l *Logger) Infof(format string, args ...any) {
	l.log(LevelInfo, 2, fmt.Sprintf(format, args...), nil)
}
func (l *Logger) Warnf(format string, args ...any) {
	l.log(LevelWarn, 2, fmt.Sprintf(format, args...), nil)
}
func (l *Logger) Errorf(format string, args ...any) {
	l.log(LevelError, 2, fmt.Sprintf(format, args...), nil)
}
func (l *Logger) Fatalf(format string, args ...any) {
	l.log(LevelFatal, 2, fmt.Sprintf(format, args...), nil)
	_ = l.Close()
	os.Exit(1)
}

// With 结构化字段日志。继承钩子列表。
func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{cfg: l.cfg, ch: l.ch, done: l.done, hooks: l.snapshotHooks()}
}

// Debugw / Infow / Warnw / Errorw —— 带结构化字段。
func (l *Logger) Debugw(msg string, fields ...Field) { l.log(LevelDebug, 2, msg, fields) }
func (l *Logger) Infow(msg string, fields ...Field)  { l.log(LevelInfo, 2, msg, fields) }
func (l *Logger) Warnw(msg string, fields ...Field)  { l.log(LevelWarn, 2, msg, fields) }
func (l *Logger) Errorw(msg string, fields ...Field) { l.log(LevelError, 2, msg, fields) }

// ---- 包级默认 logger，便捷函数 ----

var std = New()

// SetDefault 替换包级默认 logger。
func SetDefault(l *Logger) { std = l }

// Debug / Info / ... 包级便捷函数。
func Debug(args ...any) { std.log(LevelDebug, 2, fmt.Sprint(args...), nil) }
func Info(args ...any)  { std.log(LevelInfo, 2, fmt.Sprint(args...), nil) }
func Warn(args ...any)  { std.log(LevelWarn, 2, fmt.Sprint(args...), nil) }
func Error(args ...any) { std.log(LevelError, 2, fmt.Sprint(args...), nil) }
func Fatal(args ...any) {
	std.log(LevelFatal, 2, fmt.Sprint(args...), nil)
	_ = std.Close()
	os.Exit(1)
}
func Debugf(format string, args ...any)  { std.log(LevelDebug, 2, fmt.Sprintf(format, args...), nil) }
func Infof(format string, args ...any)   { std.log(LevelInfo, 2, fmt.Sprintf(format, args...), nil) }
func Warnf(format string, args ...any)   { std.log(LevelWarn, 2, fmt.Sprintf(format, args...), nil) }
func Errorf(format string, args ...any)  { std.log(LevelError, 2, fmt.Sprintf(format, args...), nil) }
func Debugw(msg string, fields ...Field) { std.log(LevelDebug, 2, msg, fields) }
func Infow(msg string, fields ...Field)  { std.log(LevelInfo, 2, msg, fields) }
func Warnw(msg string, fields ...Field)  { std.log(LevelWarn, 2, msg, fields) }
func Errorw(msg string, fields ...Field) { std.log(LevelError, 2, msg, fields) }

// contextKey 用于把 logger 注入 context（避免 string key 冲突）。
type ctxKey struct{}

// WithLogger 将 logger 注入 context。
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext 从 context 取出 logger，缺省返回默认 logger。
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok && l != nil {
		return l
	}
	return std
}
