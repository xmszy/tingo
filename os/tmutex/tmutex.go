// Package tmutex 提供增强互斥锁和命名内存锁。
//
// gmutex 增强 sync.Mutex：TryLock / LockFunc / TryLockFunc。
// gmlock 命名内存锁：基于 sync.Map，Key → *sync.Mutex。
//
// 命名锁典型用法：
//
//	tmutex.LockFunc("user:123", func() { ... })
//	tmutex.TryLockFunc("order:456", func() { ... })
//
// 增强锁用法：
//
//	m := tmutex.New()  // 替代 sync.Mutex
//	if m.TryLock() { defer m.Unlock(); ... }
//	m.LockFunc(func() { ... })
package tmutex

import (
	"sync"
	"time"
)

// ──────────────── Mutex（增强 sync.Mutex） ────────────────

// Mutex 增强互斥锁，封装 sync.Mutex 并提供 TryLock / LockFunc。
type Mutex struct {
	mu sync.Mutex
}

// New 创建 Mutex。
func New() *Mutex { return &Mutex{} }

// Lock 加锁（阻塞）。
func (m *Mutex) Lock() { m.mu.Lock() }

// Unlock 解锁。
func (m *Mutex) Unlock() { m.mu.Unlock() }

// TryLock 尝试加锁，立即返回 true/false。
func (m *Mutex) TryLock() bool { return m.mu.TryLock() }

// TryLockTimeout 在 timeout 时间内尝试加锁。
func (m *Mutex) TryLockTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if m.mu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// LockFunc 加锁后执行 fn，自动解锁（即使 panic）。
func (m *Mutex) LockFunc(fn func()) {
	m.Lock()
	defer m.Unlock()
	fn()
}

// TryLockFunc 尝试加锁后执行 fn，成功返回 true。
func (m *Mutex) TryLockFunc(fn func()) bool {
	if !m.TryLock() {
		return false
	}
	defer m.Unlock()
	fn()
	return true
}

// ──────────────── RWMutex（增强 sync.RWMutex） ────────────────

// RWMutex 增强读写互斥锁。
type RWMutex struct {
	mu sync.RWMutex
}

// NewRWMutex 创建 RWMutex。
func NewRWMutex() *RWMutex { return &RWMutex{} }

func (m *RWMutex) Lock()           { m.mu.Lock() }
func (m *RWMutex) Unlock()         { m.mu.Unlock() }
func (m *RWMutex) RLock()          { m.mu.RLock() }
func (m *RWMutex) RUnlock()        { m.mu.RUnlock() }
func (m *RWMutex) TryLock() bool { return m.mu.TryLock() }
func (m *RWMutex) TryRLock() bool { return m.mu.TryRLock() }
func (m *RWMutex) TryLockTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if m.mu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}
func (m *RWMutex) TryRLockTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if m.mu.TryRLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// LockFunc 加写锁后执行 fn。
func (m *RWMutex) LockFunc(fn func()) { m.Lock(); defer m.Unlock(); fn() }

// RLockFunc 加读锁后执行 fn。
func (m *RWMutex) RLockFunc(fn func()) { m.RLock(); defer m.RUnlock(); fn() }

// TryLockFunc 尝试加写锁后执行 fn。
func (m *RWMutex) TryLockFunc(fn func()) bool {
	if !m.TryLock() { return false }
	defer m.Unlock()
	fn()
	return true
}

// TryRLockFunc 尝试加读锁后执行 fn。
func (m *RWMutex) TryRLockFunc(fn func()) bool {
	if !m.TryRLock() { return false }
	defer m.RUnlock()
	fn()
	return true
}

// ──────────────── 命名锁 ────────────────

var namedMu sync.Map // key → *sync.Mutex

// MLock 命名加锁（阻塞）。
func MLock(key string) { mu := loadOrStoreMu(key); mu.Lock() }

// MUnlock 命名解锁。
func MUnlock(key string) {
	if v, ok := namedMu.Load(key); ok {
		v.(*sync.Mutex).Unlock()
	}
}

// MTryLock 命名尝试加锁。
func MTryLock(key string) bool { return loadOrStoreMu(key).TryLock() }

// MTryLockTimeout 命名尝试加锁（含超时）。
func MTryLockTimeout(key string, timeout time.Duration) bool {
	mu := loadOrStoreMu(key)
	deadline := time.Now().Add(timeout)
	for {
		if mu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// MLockFunc 命名加锁后执行 fn，自动解锁。
func MLockFunc(key string, fn func()) {
	MLock(key)
	defer MUnlock(key)
	fn()
}

// MTryLockFunc 命名尝试加锁后执行 fn，成功返回 true。
func MTryLockFunc(key string, fn func()) bool {
	if !MTryLock(key) {
		return false
	}
	defer MUnlock(key)
	fn()
	return true
}

func loadOrStoreMu(key string) *sync.Mutex {
	v, _ := namedMu.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}
