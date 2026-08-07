package tapp

import (
	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/errors"
	"github.com/xmszy/tingo/os/tlog"
)

/* ------------------------------------------------------------------ */
/* 日志上报器                                                            */
/* ------------------------------------------------------------------ */

// LogReporter 把异常写入 tlog，是 ExceptionHandle 的默认上报实现。
type LogReporter struct {
	// Logger 是目标日志器，为 nil 时使用全局默认日志器。
	Logger *tlog.Logger
}

// NewLogReporter 创建一个日志上报器。
func NewLogReporter(l *tlog.Logger) *LogReporter { return &LogReporter{Logger: l} }

// 确保实现 Reporter 接口。
var _ Reporter = (*LogReporter)(nil)

// Report 实现 Reporter：以结构化字段记录异常现场。
func (r *LogReporter) Report(c *core.Ctx, err error) {
	if err == nil {
		return
	}
	logger := r.Logger
	if logger == nil {
		logger = defaultLogger
	}

	e := errors.From(err)
	fields := make([]tlog.Field, 0, 8)
	fields = append(fields,
		tlog.F("code", e.Code),
		tlog.F("status", e.Status),
		tlog.F("method", c.Method()),
		tlog.F("path", c.Path()),
		tlog.F("ip", c.IP()),
	)
	if id := c.RequestID(); id != "" {
		fields = append(fields, tlog.F("request_id", id))
	}
	if meta := c.Route(); meta.Controller != "" {
		fields = append(fields, tlog.F("controller", meta.Controller), tlog.F("action", meta.Action))
	}
	if cause := e.Unwrap(); cause != nil {
		fields = append(fields, tlog.F("cause", cause.Error()))
	}
	logger.Errorw(e.Message, fields...)
}

// defaultLogger 是未显式指定日志器时使用的全局日志器。
var defaultLogger = tlog.New()

// SetDefaultLogger 替换默认日志器。
func SetDefaultLogger(l *tlog.Logger) {
	if l != nil {
		defaultLogger = l
	}
}
