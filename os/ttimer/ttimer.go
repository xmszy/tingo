// Package ttimer 提供定时器管理。
// 设计要点：
//   - 基于标准库 time，零外部依赖。
//   - 提供一次性定时和周期定时。
package ttimer

import (
	"sync"
	"time"
)

// Timer 定时器管理器。
type Timer struct {
	mu      sync.Mutex
	timers  map[*tick]struct{}
	wg      sync.WaitGroup
	stopped bool
}

type tick struct {
	timer    *time.Ticker
	done     chan struct{}
	fn       func()
	interval time.Duration
}

// New 创建定时器。
func New() *Timer { return &Timer{timers: make(map[*tick]struct{})} }

// Add 添加周期任务（返回取消函数）。
func (t *Timer) Add(interval time.Duration, fn func()) func() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return func() {}
	}
	tk := &tick{
		timer:    time.NewTicker(interval),
		done:     make(chan struct{}),
		fn:       fn,
		interval: interval,
	}
	t.timers[tk] = struct{}{}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer tk.timer.Stop()
		for {
			select {
			case <-tk.timer.C:
				tk.fn()
			case <-tk.done:
				return
			}
		}
	}()
	return func() { t.remove(tk) }
}

// After 延迟执行一次性任务。
func (t *Timer) After(delay time.Duration, fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return
	}
	tk := &tick{
		done: make(chan struct{}),
		fn:   fn,
	}
	t.timers[tk] = struct{}{}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		select {
		case <-time.After(delay):
			fn()
		case <-tk.done:
			return
		}
		t.mu.Lock()
		delete(t.timers, tk)
		t.mu.Unlock()
	}()
}

// Go 异步执行并加入 WaitGroup。
func (t *Timer) Go(fn func()) {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		fn()
	}()
}

// Stop 停止所有定时器。
func (t *Timer) Stop() {
	t.mu.Lock()
	t.stopped = true
	for tk := range t.timers {
		close(tk.done)
	}
	t.timers = make(map[*tick]struct{})
	t.mu.Unlock()
	t.wg.Wait()
}

func (t *Timer) remove(tk *tick) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.timers[tk]; ok {
		close(tk.done)
		delete(t.timers, tk)
	}
}

// ──────────────── 全局默认定时器 ────────────────

var defaultTimer = New()

// Add 全局添加周期任务。
func Add(interval time.Duration, fn func()) func() { return defaultTimer.Add(interval, fn) }

// After 全局延迟执行。
func After(delay time.Duration, fn func()) { defaultTimer.After(delay, fn) }

// Go 全局异步执行。
func Go(fn func()) { defaultTimer.Go(fn) }
