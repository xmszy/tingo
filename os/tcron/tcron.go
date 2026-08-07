// Package tcron 提供零外部依赖的定时任务调度。
//
// 特性：
//   - 支持标准 5 字段 cron 表达式（分 时 日 月 周）；
//   - 支持字段中的 , - * / 以及列表；
//   - 基于 time.Timer 的精确调度，无忙等；
//   - 任务可并发安全增删，支持 Start/Stop；
//   - 不引入第三方 cron 库，保持核心零依赖。
package tcron

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Task 是一个定时任务。
type Task struct {
	name     string
	spec     string
	fields   [5][]int // 0:min 1:hour 2:dom 3:month 4:dow
	fn       func()
	next     time.Time
	disabled bool
}

// Scheduler 调度器。
type Scheduler struct {
	mu      sync.Mutex
	tasks   map[string]*Task
	stop    chan struct{}
	done    chan struct{}
	running atomic.Bool // 防止 Start 被重复调用产生多个 loop goroutine
	loc     *time.Location
	now     func() time.Time
}

// New 创建调度器，loc 用于时区（传 nil 用 time.Local）。
func New(loc *time.Location) *Scheduler {
	if loc == nil {
		loc = time.Local
	}
	return &Scheduler{
		tasks: map[string]*Task{},
		loc:   loc,
		now:   time.Now,
	}
}

// Add 注册一个 cron 任务。
func (s *Scheduler) Add(name, spec string, fn func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fields, err := parse(spec)
	if err != nil {
		return err
	}
	s.tasks[name] = &Task{name: name, spec: spec, fields: fields, fn: fn}
	return nil
}

// Remove 删除任务。
func (s *Scheduler) Remove(name string) {
	s.mu.Lock()
	delete(s.tasks, name)
	s.mu.Unlock()
}

// Enable/Disable 启停任务。
func (s *Scheduler) Enable(name string, on bool) {
	s.mu.Lock()
	if t, ok := s.tasks[name]; ok {
		t.disabled = !on
	}
	s.mu.Unlock()
}

// Start 启动调度循环（阻塞直到 Stop）。重复调用安全：仅首次生效。
func (s *Scheduler) Start() {
	if !s.running.CompareAndSwap(false, true) {
		return // 已在运行，避免重复 Start 导致 goroutine 泄漏/竞态
	}
	s.mu.Lock()
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()
	go s.loop()
}

// Stop 停止调度。重复调用安全：仅首次生效。
func (s *Scheduler) Stop() {
	if !s.running.CompareAndSwap(true, false) {
		return // 未在运行，避免对已关闭的 stop channel 二次 close
	}
	s.mu.Lock()
	stop := s.stop
	s.mu.Unlock()
	close(stop)
	<-s.done
}

func (s *Scheduler) loop() {
	defer close(s.done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		s.mu.Lock()
		next := s.nextRunLocked()
		s.mu.Unlock()
		if next.IsZero() {
			timer.Reset(time.Hour)
		} else {
			delta := time.Until(next)
			if delta < 0 {
				delta = time.Millisecond
			}
			timer.Reset(delta)
		}
		select {
		case <-s.stop:
			return
		case now := <-timer.C:
			s.fireLocked(now)
		}
	}
}

// fireLocked 触发所有到点的任务。调用方需持有 s.mu 或在无锁时操作（此处简化：复制待执行列表）。
func (s *Scheduler) fireLocked(now time.Time) {
	s.mu.Lock()
	type pending struct {
		fn func()
	}
	var pend []pending
	for _, t := range s.tasks {
		if t.disabled {
			continue
		}
		if !t.next.IsZero() && !now.Before(t.next) {
			pend = append(pend, pending{fn: t.fn})
			t.next = time.Time{} // 等待下次计算
		}
	}
	s.mu.Unlock()
	for _, p := range pend {
		p.fn()
	}
}

// nextRunLocked 计算最近的下一次执行时间（仅已到点的会先被 fire）。
func (s *Scheduler) nextRunLocked() time.Time {
	base := s.now().In(s.loc)
	var best time.Time
	for _, t := range s.tasks {
		if t.disabled {
			continue
		}
		if t.next.IsZero() {
			t.next = nextTime(t, base)
		}
		if best.IsZero() || t.next.Before(best) {
			best = t.next
		}
	}
	return best
}

// nextTime 计算从 base 之后满足 spec 的最早时间（最多向前搜索 5 年）。
func nextTime(t *Task, base time.Time) time.Time {
	cur := base.Truncate(time.Minute).Add(time.Minute)
	limit := cur.AddDate(5, 0, 0)
	for cur.Before(limit) {
		if match(t, cur) {
			return cur
		}
		cur = cur.Add(time.Minute)
	}
	return time.Time{}
}

// match 判断时间是否满足字段。dom 与 dow 同时非 * 时为 OR，否则各自 AND。
func match(t *Task, tm time.Time) bool {
	if !inField(t.fields[0], tm.Minute()) {
		return false
	}
	if !inField(t.fields[1], tm.Hour()) {
		return false
	}
	if !inField(t.fields[3], int(tm.Month())) {
		return false
	}
	domStar := isStar(t.fields[2])
	dowStar := isStar(t.fields[4])
	dow := int(tm.Weekday())
	dowMatch := inField(t.fields[4], dow) || inField(t.fields[4], dow%7+7)
	domMatch := inField(t.fields[2], tm.Day())
	switch {
	case domStar && dowStar:
		return true
	case domStar:
		return dowMatch
	case dowStar:
		return domMatch
	default:
		return domMatch || dowMatch
	}
}

func isStar(f []int) bool { return len(f) == 1 && f[0] == -1 }

func inField(f []int, v int) bool {
	for _, x := range f {
		if x == -1 || x == v {
			return true
		}
	}
	return false
}

// parse 解析 5 字段 cron 表达式。
func parse(spec string) ([5][]int, error) {
	parts := strings.Fields(spec)
	if len(parts) != 5 {
		return [5][]int{}, errInvalidSpec(spec)
	}
	var out [5][]int
	ranges := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}
	for i, p := range parts {
		vals, err := parseField(p, ranges[i][0], ranges[i][1])
		if err != nil {
			return out, err
		}
		out[i] = vals
	}
	return out, nil
}

func parseField(p string, min, max int) ([]int, error) {
	if p == "*" {
		return []int{-1}, nil
	}
	var out []int
	for seg := range strings.SplitSeq(p, ",") {
		lo, hi, step := min, max, 1
		if before, after, found := strings.Cut(seg, "/"); found {
			stepStr := after
			st, err := strconv.Atoi(stepStr)
			if err != nil || st <= 0 {
				return nil, errInvalidField(p)
			}
			step = st
			seg = before
		}
		if seg == "*" {
			lo, hi = min, max
		} else if before, after, found := strings.Cut(seg, "-"); found {
			a, err1 := strconv.Atoi(before)
			b, err2 := strconv.Atoi(after)
			if err1 != nil || err2 != nil {
				return nil, errInvalidField(p)
			}
			lo, hi = a, b
		} else {
			a, err := strconv.Atoi(seg)
			if err != nil {
				return nil, errInvalidField(p)
			}
			lo, hi = a, a
		}
		if lo < min || hi > max {
			return nil, errInvalidField(p)
		}
		for v := lo; v <= hi; v += step {
			out = append(out, v)
		}
	}
	return out, nil
}

type cronError struct{ msg string }

func (e cronError) Error() string { return e.msg }
func errInvalidSpec(s string) error {
	return cronError{"tcron: invalid spec " + strconv.Quote(s)}
}
func errInvalidField(s string) error {
	return cronError{"tcron: invalid field " + strconv.Quote(s)}
}
