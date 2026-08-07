// Package ttime 提供时间处理工具。
// 设计要点：
//   - 基于标准库 time，零外部依赖。
//   - 提供增强的 time.Time 封装和便捷时间操作。
package ttime

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ──────────────── Time 增强类型 ────────────────

// Time 增强的时间类型（内嵌 time.Time）。
type Time struct {
	time.Time
}

// New 创建 Time。参数支持：time.Time / int64(Unix) / string。
func New(v ...any) *Time {
	if len(v) == 0 {
		return &Time{Time: time.Now()}
	}
	switch val := v[0].(type) {
	case time.Time:
		return &Time{Time: val}
	case *time.Time:
		return &Time{Time: *val}
	case int64:
		return &Time{Time: time.Unix(val, 0)}
	case int:
		return &Time{Time: time.Unix(int64(val), 0)}
	case string:
		t := parseTime(val)
		return &Time{Time: t}
	default:
		return &Time{Time: time.Now()}
	}
}

func parseTime(s string) time.Time {
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
		"20060102150405",
		"20060102",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Now 返回当前时间的 Time。
func Now() *Time { return &Time{Time: time.Now()} }

// Unix 从 Unix 时间戳创建 Time。
func Unix(sec int64, nsec ...int64) *Time {
	if len(nsec) > 0 {
		return &Time{Time: time.Unix(sec, nsec[0])}
	}
	return &Time{Time: time.Unix(sec, 0)}
}

// UnixMilli 从毫秒时间戳创建 Time。
func UnixMilli(ms int64) *Time { return &Time{Time: time.UnixMilli(ms)} }

// StrToTime 字符串转 Time，带自定义布局。
func StrToTime(s, layout string) (*Time, error) {
	t, err := time.Parse(layout, s)
	if err != nil {
		return nil, err
	}
	return &Time{Time: t}, nil
}

// ──────────────── Time 方法 ────────────────

// String 返回 "2006-01-02 15:04:05" 格式字符串。
func (t *Time) String() string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// UnixMilli 返回毫秒时间戳。
func (t *Time) UnixMilli() int64 { return t.Unix() * 1000 }

// ──────────────── 全局时间函数 ────────────────

// Timestamp 返回当前 Unix 时间戳（秒）。
func Timestamp() int64 { return time.Now().Unix() }

// TimestampMilli 返回当前毫秒时间戳。
func TimestampMilli() int64 { return time.Now().UnixMilli() }

// TimestampNano 返回当前纳秒时间戳。
func TimestampNano() int64 { return time.Now().UnixNano() }

// Date 格式化当前时间（2006-01-02 15:04:05）。
func Date(layout ...string) string {
	f := "2006-01-02 15:04:05"
	if len(layout) > 0 {
		f = layout[0]
	}
	return time.Now().Format(f)
}

// Datetime 返回日期时间字符串 "2006-01-02 15:04:05"。
func Datetime() string { return Date() }

// Today 返回今天日期 "2006-01-02"。
func Today() string { return Date("2006-01-02") }

// UnixToDate 时间戳转日期字符串。
func UnixToDate(ts int64) string { return time.Unix(ts, 0).Format("2006-01-02") }

// UnixToDatetime 时间戳转日期时间字符串。
func UnixToDatetime(ts int64) string { return time.Unix(ts, 0).Format("2006-01-02 15:04:05") }

// StrToTimeLayout 根据布局解析时间字符串。
func StrToTimeLayout(s, layout string) (time.Time, error) { return time.Parse(layout, s) }

// ──────────────── 常用常量 ────────────────

const (
	Day  = 24 * time.Hour
	Week = 7 * Day
)

// ──────────────── 便捷构造 ────────────────

// Seconds 将秒数转为 time.Duration。
func Seconds(n int) time.Duration { return time.Duration(n) * time.Second }

// Minutes 将分钟数转为 time.Duration。
func Minutes(n int) time.Duration { return time.Duration(n) * time.Minute }

// Hours 将小时数转为 time.Duration。
func Hours(n int) time.Duration { return time.Duration(n) * time.Hour }

// ──────────────── 日期比较 ────────────────

// After 判断 t 是否在 u 之后。
func After(t, u time.Time) bool { return t.After(u) }

// Before 判断 t 是否在 u 之前。
func Before(t, u time.Time) bool { return t.Before(u) }

// Equal 判断两个时间是否相等（去掉时区影响）。
func Equal(t, u time.Time) bool { return t.Equal(u) }

// ──────────────── 时间差 ────────────────

// Since 返回从 t 到现在的时间间隔。
func Since(t time.Time) time.Duration { return time.Since(t) }

// Until 返回从现在到 t 的时间间隔。
func Until(t time.Time) time.Duration { return time.Until(t) }

// ──────────────── 工具函数 ────────────────

// FormatDuration 将 Duration 格式化为 "1h2m3s" 字符串。
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, strconv.FormatFloat(float64(d)/float64(time.Second), 'f', 3, 64)+"s")
	}
	return strings.Join(parts, "")
}
