package tconv

import (
	"fmt"
	"time"

	"github.com/xmszy/tingo/os/ttime"
)

// ──────────────── 时间相关转换 ────────────────

// Duration 转为 time.Duration。
// 支持：time.Duration / 数字（按纳秒）/ "1.5s"、"300ms"、"2h" 等字符串。
func Duration(v any) time.Duration {
	switch val := v.(type) {
	case time.Duration:
		return val
	case int:
		return time.Duration(val)
	case int64:
		return time.Duration(val)
	case float64:
		return time.Duration(int64(val))
	case string:
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return time.Duration(Int64(v))
}

// GTime 转为增强时间类型 *ttime.Time。
// format 可指定解析布局（与 time.Parse 一致）；不指定时由 ttime 自动识别常见格式。
func GTime(v any, format ...string) (*ttime.Time, error) {
	switch val := v.(type) {
	case time.Time:
		return ttime.New(val), nil
	case *time.Time:
		return ttime.New(*val), nil
	case int64:
		return ttime.New(val), nil
	case int:
		return ttime.New(val), nil
	case string:
		if len(format) > 0 && format[0] != "" {
			t, err := time.Parse(format[0], val)
			if err != nil {
				return nil, fmt.Errorf("tconv: parse time %q with %q: %w", val, format[0], err)
			}
			return ttime.New(t), nil
		}
		return ttime.New(val), nil
	}
	return ttime.New(time.Now()), nil
}
