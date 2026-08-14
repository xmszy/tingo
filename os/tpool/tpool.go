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

/* ──────────────── 动态协程池 ──────────────── */

// DynamicPool 动态协程池：维持 [min, max] 区间内的 worker 数量。
// 当任务队列积压时自动扩容，空闲时可通过 Shrink 手动回落到 min。
// 适用于负载波动明显的场景（gf gpool 的动态模式对应物）。
type DynamicPool struct {
	min   int
	max   int
	tasks chan func()
	wg    sync.WaitGroup
	mu    sync.Mutex
	cur   int // 当前 worker 数
	stop  bool
}

// NewDynamicPool 创建动态协程池。min<=0 取 NumCPU；max<=min 时退化为固定 min（无动态扩容）。
func NewDynamicPool(min, max int) *DynamicPool {
	if min <= 0 {
		min = runtime.NumCPU()
	}
	if max < min {
		max = min
	}
	p := &DynamicPool{
		min:   min,
		max:   max,
		tasks: make(chan func(), min*10),
	}
	for i := 0; i < min; i++ {
		p.spawn()
	}
	return p
}

// spawn 在持有锁时新增一个 worker。
func (p *DynamicPool) spawn() {
	p.cur++
	go p.worker()
}

// Submit 提交任务；若队列满且未达 max，则临时扩容一个 worker 处理积压。
func (p *DynamicPool) Submit(fn func()) {
	p.mu.Lock()
	if p.stop {
		p.mu.Unlock()
		return
	}
	p.wg.Add(1)
	// 队列满且可扩容：多起一个 worker 加速消费。
	if len(p.tasks) >= cap(p.tasks) && p.cur < p.max {
		p.spawn()
	}
	p.mu.Unlock()
	p.tasks <- fn
}

// Scale 调整下限（重新预热到新的 min），max 不变。
func (p *DynamicPool) Scale(min int) {
	if min <= 0 {
		min = runtime.NumCPU()
	}
	if min > p.max {
		min = p.max
	}
	p.mu.Lock()
	p.min = min
	for p.cur < min {
		p.spawn()
	}
	p.mu.Unlock()
}

// Shrink 将 worker 数回落到 min（已空闲的 worker 会在下轮任务后退出）。
func (p *DynamicPool) Shrink() {
	p.mu.Lock()
	p.min = minInt(p.min, p.max)
	p.mu.Unlock()
}

// Stop 停止接收新任务并等待已有任务完成。
func (p *DynamicPool) Stop() {
	p.mu.Lock()
	p.stop = true
	p.mu.Unlock()
	p.wg.Wait()
}

// Wait 等待所有任务完成。
func (p *DynamicPool) Wait() { p.wg.Wait() }

func (p *DynamicPool) worker() {
	for fn := range p.tasks {
		fn()
		p.wg.Done()
		// 退出条件：超过 min 且当前空闲（队列已空）。
		p.mu.Lock()
		if p.cur > p.min && len(p.tasks) == 0 {
			p.cur--
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
