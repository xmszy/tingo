package tcron

import (
	"fmt"
	"strings"
	"time"
)

// SecondSchedule 支持秒级精度的 cron 调度（6 字段：秒 分 时 日 月 周）。
type SecondSchedule struct {
	seconds matcher
	minutes matcher
	hours   matcher
	days    matcher
	months  matcher
	weeks   matcher
}

// NewSecondSchedule 解析 6 字段 cron 表达式创建秒级调度器。
//
// 格式：秒 分 时 日 月 周
// 示例：*/5 * * * * *  每 5 秒
//
//	0 */30 * * * *  每 30 分钟
//	0 0 9 * * 1-5  工作日 9:00
func NewSecondSchedule(expr string) (*SecondSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 6 {
		return nil, fmt.Errorf("tcron: second cron requires 6 fields, got %d", len(fields))
	}

	s := &SecondSchedule{}

	secondExpr := fields[0]
	minuteExpr := fields[1]
	hourExpr := fields[2]
	dayExpr := fields[3]
	monthExpr := fields[4]
	weekExpr := fields[5]

	if SecondExpr := secondExpr; SecondExpr != "" {
		sec, err := parseMatcher(SecondExpr, 0, 59)
		if err != nil {
			return nil, fmt.Errorf("tcron: invalid seconds: %w", err)
		}
		s.seconds = sec
	}

	min, err := parseMatcher(minuteExpr, 0, 59)
	if err != nil {
		return nil, fmt.Errorf("tcron: invalid minutes: %w", err)
	}
	s.minutes = min

	hour, err := parseMatcher(hourExpr, 0, 23)
	if err != nil {
		return nil, fmt.Errorf("tcron: invalid hours: %w", err)
	}
	s.hours = hour

	day, err := parseMatcher(dayExpr, 1, 31)
	if err != nil {
		return nil, fmt.Errorf("tcron: invalid days: %w", err)
	}
	s.days = day

	month, err := parseMatcher(monthExpr, 1, 12)
	if err != nil {
		return nil, fmt.Errorf("tcron: invalid months: %w", err)
	}
	s.months = month

	week, err := parseMatcher(weekExpr, 0, 6)
	if err != nil {
		return nil, fmt.Errorf("tcron: invalid weeks: %w", err)
	}
	s.weeks = week

	return s, nil
}

// Next 返回当前时间之后的下一次执行时间。
func (s *SecondSchedule) Next(now time.Time) time.Time {
	year, month, day := now.Date()
	hour, minute, second := now.Clock()

	// 检查当前秒
	if s.seconds.match(second) {
		candidate := time.Date(year, month, day, hour, minute, second, 0, now.Location())
		if candidate.After(now) {
			return candidate
		}
	}

	// 从下一秒开始找
	t := now.Add(time.Second)
	t = t.Truncate(time.Second)

	// 遍历查找（简化实现，高性能实现应用 calendar 算法）
	for range 100*365*24*60*60 {
		if s.matches(t) {
			return t
		}
		t = t.Add(time.Second)
	}

	return time.Time{}
}

func (s *SecondSchedule) matches(t time.Time) bool {
	if !s.seconds.match(t.Second()) {
		return false
	}
	if !s.minutes.match(t.Minute()) {
		return false
	}
	if !s.hours.match(t.Hour()) {
		return false
	}
	if !s.days.match(t.Day()) {
		return false
	}
	if !s.months.match(int(t.Month())) {
		return false
	}
	if !s.weeks.match(int(t.Weekday())) {
		return false
	}
	return true
}

// String 返回 cron 表达式字符串。
func (s *SecondSchedule) String() string {
	return fmt.Sprintf("%s %s %s %s %s %s",
		s.seconds.String(),
		s.minutes.String(),
		s.hours.String(),
		s.days.String(),
		s.months.String(),
		s.weeks.String(),
	)
}

// IsSecondExpr 判断是否为 6 字段秒级 cron 表达式。
func IsSecondExpr(expr string) bool {
	return len(strings.Fields(expr)) == 6
}

// parseMatcher 解析单个字段。
type matcher interface {
	match(v int) bool
	String() string
}

func parseMatcher(expr string, min, max int) (matcher, error) {
	if expr == "*" {
		return &starMatcher{}, nil
	}
	if strings.Contains(expr, "/") {
		return parseStepMatcher(expr, max)
	}
	if strings.Contains(expr, ",") {
		return parseListMatcher(expr, min, max)
	}
	if strings.Contains(expr, "-") {
		return parseRangeMatcher(expr, min, max)
	}
	return parseSingleMatcher(expr, min, max)
}

type starMatcher struct{}

func (s *starMatcher) match(v int) bool  { return true }
func (s *starMatcher) String() string    { return "*" }

type singleMatcher struct{ value int }

func (s *singleMatcher) match(v int) bool  { return v == s.value }
func (s *singleMatcher) String() string    { return fmt.Sprintf("%d", s.value) }

type rangeMatcher struct{ min, max int }

func (r *rangeMatcher) match(v int) bool  { return v >= r.min && v <= r.max }
func (r *rangeMatcher) String() string    { return fmt.Sprintf("%d-%d", r.min, r.max) }

type stepMatcher struct {
	matcher matcher
	step    int
	count   int
}

func (s *stepMatcher) match(v int) bool {
	return s.matcher.match(v) && v%s.step == 0
}
func (s *stepMatcher) String() string {
	return s.matcher.String() + "/" + fmt.Sprintf("%d", s.step)
}

type listMatcher struct{ items []int }

func (l *listMatcher) match(v int) bool {
	for _, item := range l.items {
		if v == item {
			return true
		}
	}
	return false
}
func (l *listMatcher) String() string {
	strs := make([]string, len(l.items))
	for i, v := range l.items {
		strs[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(strs, ",")
}

func parseSingleMatcher(expr string, min, max int) (matcher, error) {
	v, err := toInt(expr)
	if err != nil {
		return nil, err
	}
	if v < min || v > max {
		return nil, fmt.Errorf("value %d out of range [%d,%d]", v, min, max)
	}
	return &singleMatcher{value: v}, nil
}

func parseRangeMatcher(expr string, min, max int) (matcher, error) {
	parts := strings.Split(expr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range: %s", expr)
	}
	from, err := toInt(parts[0])
	if err != nil {
		return nil, err
	}
	to, err := toInt(parts[1])
	if err != nil {
		return nil, err
	}
	if from < min || to > max {
		return nil, fmt.Errorf("range [%d,%d] out of bounds [%d,%d]", from, to, min, max)
	}
	return &rangeMatcher{min: from, max: to}, nil
}

func parseStepMatcher(expr string, max int) (matcher, error) {
	parts := strings.SplitN(expr, "/", 2)
	step, err := toInt(parts[1])
	if err != nil {
		return nil, err
	}
	var base matcher
	if parts[0] == "*" {
		base = &starMatcher{}
	} else {
		base, err = parseMatcher(parts[0], 0, max)
		if err != nil {
			return nil, err
		}
	}
	return &stepMatcher{matcher: base, step: step}, nil
}

func parseListMatcher(expr string, min, max int) (matcher, error) {
	parts := strings.Split(expr, ",")
	items := make([]int, len(parts))
	for i, p := range parts {
		v, err := toInt(p)
		if err != nil {
			return nil, err
		}
		if v < min || v > max {
			return nil, fmt.Errorf("list value %d out of bounds [%d,%d]", v, min, max)
		}
		items[i] = v
	}
	return &listMatcher{items: items}, nil
}

func toInt(s string) (int, error) {
	var n int
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
