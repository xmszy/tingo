package middleware

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tcfg"
	"github.com/xmszy/tingo/os/tlog"
)

// LoggerConfig 是访问日志中间件的配置。
type LoggerConfig struct {
	// Output 是日志输出目标，默认 os.Stdout。
	Output io.Writer
	// Log 是统一日志通道；非 nil 时访问日志走 tlog，Output/Colored 失效。
	Log *tlog.Logger
	// SkipPaths 是不记录日志的路径集合，如健康检查。
	SkipPaths []string
	// SkipNotFound 为 true 时，未命中路由的 404/405 响应不写访问日志。
	// 默认开启，可避免探测/扫描/DDoS 类噪声在日志中放大；如需审计所有请求置为 false。
	SkipNotFound bool
	// Colored 决定是否输出 ANSI 颜色。
	Colored bool
	// Formatter 自定义日志行格式。
	Formatter func(LogRecord) string
}

// LogRecord 是一条访问日志的结构化数据。
type LogRecord struct {
	// Time 是请求结束时刻。
	Time time.Time
	// Latency 是请求处理耗时。
	Latency time.Duration
	// Status 是响应状态码。
	Status int
	// Method 是 HTTP 方法。
	Method string
	// Path 是请求路径（含 query）。
	Path string
	// IP 是客户端 IP。
	IP string
	// App 是命中的应用名。
	App string
	// Size 是响应体字节数。
	Size int
	// RequestID 是请求追踪 ID。
	RequestID string
	// Err 是处理过程中记录的错误信息。
	Err string
}

// Logger 返回访问日志中间件。
func Logger(opts ...func(*LoggerConfig)) core.Handler {
	cfg := LoggerConfig{Output: os.Stdout, Colored: true}
	for _, o := range opts {
		o(&cfg)
	}
	return newLogger(cfg)
}

// LoggerWithLog 将访问日志输出到指定的 tlog.Logger 通道。
func LoggerWithLog(l *tlog.Logger) func(*LoggerConfig) {
	return func(c *LoggerConfig) { c.Log = l }
}

type compiledLoggerConfig struct {
	config       LoggerConfig
	skip         map[string]struct{}
	skipNotFound bool
}

// AccessLogger 是可在 Boot 阶段原子注入配置的访问日志处理器。
type AccessLogger struct {
	current atomic.Pointer[compiledLoggerConfig]
}

func NewAccessLogger() *AccessLogger { return &AccessLogger{} }

func (l *AccessLogger) Configure(config LoggerConfig) { l.current.Store(compileLogger(config)) }

// ConfigureFromTree 从 log 与 log.access 命名空间装配默认访问日志。
func (l *AccessLogger) ConfigureFromTree(reader tcfg.Reader) error {
	if reader.Has("log.access.enabled") && !reader.Bool("log.access.enabled", true) {
		l.current.Store(nil)
		return nil
	}
	logger, err := tlog.NewFromTree(reader)
	if err != nil {
		return err
	}
	// skip_not_found 默认开启：未命中路由的 404/405 不写访问日志，
	// 避免探测/扫描/DDoS 类噪声在日志中放大。显式配置 false 可审计全部请求。
	skipNotFound := reader.Bool("log.access.skip_not_found", true)
	l.Configure(LoggerConfig{
		Log:         logger,
		SkipPaths:   reader.Strings("log.access.skip_paths"),
		SkipNotFound: skipNotFound,
		Colored:     false,
	})
	return nil
}

func (l *AccessLogger) Handler() core.Handler {
	return loggerHandler(func() *compiledLoggerConfig { return l.current.Load() })
}

func newLogger(config LoggerConfig) core.Handler {
	compiled := compileLogger(config)
	return loggerHandler(func() *compiledLoggerConfig { return compiled })
}

func compileLogger(config LoggerConfig) *compiledLoggerConfig {
	var skip map[string]struct{}
	if len(config.SkipPaths) > 0 {
		skip = make(map[string]struct{}, len(config.SkipPaths))
		for _, path := range config.SkipPaths {
			skip[path] = struct{}{}
		}
	}
	if config.Formatter == nil {
		if config.Colored {
			config.Formatter = coloredLine
		} else {
			config.Formatter = plainLine
		}
	}
	return &compiledLoggerConfig{config: config, skip: skip, skipNotFound: config.SkipNotFound}
}

func loggerHandler(load func() *compiledLoggerConfig) core.Handler {
	return func(c *core.Ctx) {
		compiled := load()
		if compiled == nil {
			c.Next()
			return
		}
		path := c.Path()
		if _, skipped := compiled.skip[path]; skipped {
			c.Next()
			return
		}

		start := time.Now()
		raw := c.RawQuery()
		c.Next()
		// 未命中路由的 404/405 默认不记日志，避免探测/扫描噪声放大。
		if compiled.skipNotFound && (c.Res().Status() == http.StatusNotFound || c.Res().Status() == http.StatusMethodNotAllowed) {
			return
		}
		if raw != "" {
			path += "?" + raw
		}
		record := LogRecord{
			Time:      time.Now(),
			Latency:   time.Since(start),
			Status:    c.Res().Status(),
			Method:    c.Method(),
			Path:      path,
			IP:        c.IP(),
			App:       c.App(),
			Size:      c.Res().Size(),
			RequestID: c.RequestID(),
			Err:       c.G().Errors.ByType(gin.ErrorTypePrivate).String(),
		}
		emit(compiled.config, record)
	}
}

// emit 按配置输出日志行：有注入的 logger 走 tlog 通道，否则直接写 Output。
func emit(cfg LoggerConfig, rec LogRecord) {
	if cfg.Log != nil {
		fields := []tlog.Field{
			tlog.F("status", rec.Status),
			tlog.F("latency", rec.Latency.String()),
			tlog.F("ip", rec.IP),
			tlog.F("method", rec.Method),
			tlog.F("path", rec.Path),
		}
		if rec.App != "" {
			fields = append(fields, tlog.F("app", rec.App))
		}
		if rec.RequestID != "" {
			fields = append(fields, tlog.F("req_id", rec.RequestID))
		}
		if rec.Err != "" {
			fields = append(fields, tlog.F("error", rec.Err))
		}
		cfg.Log.Infow("http-access", fields...)
		return
	}
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	format := cfg.Formatter
	if format == nil {
		if cfg.Colored {
			format = coloredLine
		} else {
			format = plainLine
		}
	}
	fmt.Fprint(out, format(rec))
}

// ANSI 颜色码。
const (
	colReset  = "\033[0m"
	colRed    = "\033[97;41m"
	colGreen  = "\033[97;42m"
	colYellow = "\033[90;43m"
	colBlue   = "\033[97;44m"
	colCyan   = "\033[97;46m"
	colGray   = "\033[90m"
)

// statusColor 按状态码返回背景色。
func statusColor(status int) string {
	switch {
	case status >= 200 && status < 300:
		return colGreen
	case status >= 300 && status < 400:
		return colBlue
	case status >= 400 && status < 500:
		return colYellow
	default:
		return colRed
	}
}

// methodColor 按方法返回背景色。
func methodColor(m string) string {
	switch m {
	case "GET":
		return colBlue
	case "POST":
		return colCyan
	case "PUT", "PATCH":
		return colYellow
	case "DELETE":
		return colRed
	default:
		return colGray
	}
}

// coloredLine 生成带颜色的日志行。
func coloredLine(r LogRecord) string {
	app := ""
	if r.App != "" {
		app = " [" + r.App + "]"
	}
	line := fmt.Sprintf("%s %s %3d %s|%13v|%15s|%s %-7s %s%s %s\n",
		r.Time.Format("2006/01/02 15:04:05"),
		statusColor(r.Status), r.Status, colReset,
		r.Latency, r.IP,
		methodColor(r.Method), r.Method, colReset,
		app, r.Path,
	)
	if r.Err != "" {
		line += colRed + r.Err + colReset + "\n"
	}
	return line
}

// plainLine 生成无颜色的日志行。
func plainLine(r LogRecord) string {
	app := ""
	if r.App != "" {
		app = " [" + r.App + "]"
	}
	line := fmt.Sprintf("%s | %3d | %13v | %15s | %-7s%s %s\n",
		r.Time.Format("2006/01/02 15:04:05"),
		r.Status, r.Latency, r.IP, r.Method, app, r.Path,
	)
	if r.Err != "" {
		line += "  error: " + r.Err + "\n"
	}
	return line
}
