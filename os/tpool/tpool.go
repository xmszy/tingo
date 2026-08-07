// Package tpool 提供协程池。
// 设计要点：
//   - 基于标准库 sync，零外部依赖。
//   - 固定大小协程池，任务分发给空闲 worker，无任务时阻塞等待。
package tpool

import (
	"runtime"
	"sync"
)

// Pool 协程池。
type Pool struct {
	size    int
	tasks   chan func()
	wg      sync.WaitGroup
	stopped bool
	mu      sync.Mutex
}

// New 创建协程池。size<=0 时自动取 runtime.NumCPU() 作为并发度。
func New(size int) *Pool {
	if size <= 0 {
		size = runtime.NumCPU()
	}
	p := &Pool{
		size:  size,
		tasks: make(chan func(), size*10),
	}
	for i := 0; i < size; i++ {
		go p.worker()
	}
	return p
}

// Submit 提交任务。
func (p *Pool) Submit(fn func()) {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.wg.Add(1)
	p.mu.Unlock()
	p.tasks <- fn
}

// Wait 等待所有任务完成。
func (p *Pool) Wait() { p.wg.Wait() }

// Stop 停止接收新任务，等待已有任务完成。
func (p *Pool) Stop() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *Pool) worker() {
	for fn := range p.tasks {
		fn()
		p.wg.Done()
	}
}

// ──────────────── 全局池 ────────────────

var defaultPool = New(0) // 0 表示使用 runtime.NumCPU

// Submit 全局提交任务。
func Submit(fn func()) { defaultPool.Submit(fn) }

// Wait 全局等待。
func Wait() { defaultPool.Wait() }
