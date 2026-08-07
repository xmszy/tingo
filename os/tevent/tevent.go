// Package tevent 提供轻量、零外部依赖的事件总线。
//
// 特性：
//   - 类型安全的泛型事件：Event[T]，监听器为 func(ctx, T) error；
//   - 同步分发（Dispatch）与异步分发（DispatchAsync，内部 goroutine 池）；
//   - 支持一次性监听（Once）与取消订阅（Unsubscribe）；
//   - 监听器 panic 被 recover 并转为错误，不影响其余监听器；
//   - 并发安全，可热注册/注销监听器。
package tevent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Handler 是事件监听器函数。
type Handler[T any] func(ctx context.Context, payload T) error

// Event 描述一个命名的事件类型。
type Event[T any] struct {
	name string
}

// New 声明一个事件（仅用于类型推断，如 ev := tevent.New[UserCreated]("user.created")）。
func New[T any](name string) Event[T] { return Event[T]{name: name} }

// Name 返回事件名。
func (e Event[T]) Name() string { return e.name }

// Bus 是事件总线。
type Bus struct {
	mu    sync.RWMutex
	subs  map[string][]*subEntry
	wg    sync.WaitGroup
	async bool
}

type subEntry struct {
	id        uint64
	h         any    // Handler[T] 装箱，避免类型参数化 map
	once      bool
	isPattern bool   // 前缀通配订阅（如 "user." 匹配 "user.login"）
}

// NewBus 创建事件总线。async=true 时 DispatchAsync 在 goroutine 中执行监听器。
func NewBus(async bool) *Bus {
	return &Bus{subs: map[string][]*subEntry{}, async: async}
}

var idGen uint64

// Subscribe 注册监听器，返回订阅 ID（用于取消）。
func Subscribe[T any](b *Bus, ev Event[T], h Handler[T]) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := atomic.AddUint64(&idGen, 1)
	b.subs[ev.name] = append(b.subs[ev.name], &subEntry{id: id, h: h, once: false})
	return id
}

// Once 注册仅触发一次的监听器。
func Once[T any](b *Bus, ev Event[T], h Handler[T]) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := atomic.AddUint64(&idGen, 1)
	b.subs[ev.name] = append(b.subs[ev.name], &subEntry{id: id, h: h, once: true})
	return id
}

// SubscribePattern 注册前缀通配订阅。
// pattern 为事件名前缀（如 "user."），当分发 "user.login" 时该监听器也会触发。
// 若要监听所有事件，使用 "" 或 "*" 作为 pattern，handler 使用 Handler[any]。
//
//	bus := tevent.NewBus(false)
//	tevent.SubscribePattern[any](bus, "user.", func(ctx context.Context, payload any) error {
//	    fmt.Println("user event:", payload)
//	    return nil
//	})
func SubscribePattern[T any](b *Bus, pattern string, h Handler[T]) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := atomic.AddUint64(&idGen, 1)
	b.subs[pattern] = append(b.subs[pattern], &subEntry{id: id, h: h, once: false, isPattern: true})
	return id
}

// Unsubscribe 按事件名与订阅 ID 取消监听。
func Unsubscribe(b *Bus, name string, id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.subs[name]
	// 新分配切片，避免原地复用底层数组（list[:0]）污染并发持有同一底层数组的读者。
	out := make([]*subEntry, 0, len(list))
	for _, s := range list {
		if s.id != id {
			out = append(out, s)
		}
	}
	b.subs[name] = out
}

// Dispatch 同步分发事件给所有监听器（含通配匹配），返回首个非 nil 错误（不中断其余监听器执行完）。
// 通配订阅：若注册过 pattern="user."，则分发 "user.login" 时也会触发，handler 类型断言优先匹配 Handler[T]，
// 其次退化为 Handler[any]（模式订阅常用 any 监听多个事件类型）。
func Dispatch[T any](b *Bus, ctx context.Context, ev Event[T], payload T) error {
	b.mu.RLock()
	list := collectListeners(b, ev.name)
	b.mu.RUnlock()

	var firstErr error
	removeIDs := make([]uint64, 0, len(list))
	for _, s := range list {
		if err := invokeHandler(ctx, payload, s); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
		if s.once {
			removeIDs = append(removeIDs, s.id)
		}
	}
	for _, id := range removeIDs {
		Unsubscribe(b, ev.name, id)
	}
	return firstErr
}

// collectListeners 收集精确匹配 + 前缀通配匹配的监听器。
func collectListeners(b *Bus, name string) []*subEntry {
	list := append([]*subEntry(nil), b.subs[name]...)
	// prefix wildcard matching
	for pattern, entries := range b.subs {
		if len(entries) == 0 {
			continue
		}
		if entries[0].isPattern && strings.HasPrefix(name, pattern) {
			list = append(list, entries...)
		}
	}
	return list
}

// invokeHandler 类型安全地调用监听器。优先匹配 Handler[T]，其次退化为 Handler[any]。
func invokeHandler[T any](ctx context.Context, payload T, s *subEntry) error {
	if h, ok := s.h.(Handler[T]); ok {
		return safeCall(ctx, payload, h)
	}
	if h, ok := s.h.(Handler[any]); ok {
		return safeCall(ctx, any(payload), h)
	}
	return nil
}

// DispatchAsync 异步分发（仅当 Bus 创建时 async=true 生效，否则退化为同步）。
// 调用方可通过 Wait 等待所有异步监听器完成。
func DispatchAsync[T any](b *Bus, ctx context.Context, ev Event[T], payload T) {
	if !b.async {
		_ = Dispatch(b, ctx, ev, payload)
		return
	}
	b.mu.RLock()
	list := collectListeners(b, ev.name)
	b.mu.RUnlock()
	for _, s := range list {
		var h Handler[T]
		var hAny Handler[any]
		var ok bool
		if h, ok = s.h.(Handler[T]); ok {
		} else if hAny, ok = s.h.(Handler[any]); ok {
		} else {
			continue
		}
		b.wg.Add(1)
		go func(s *subEntry, h Handler[T], hAny Handler[any]) {
			defer b.wg.Done()
			var err error
			if h != nil {
				err = safeCall(ctx, payload, h)
			} else {
				err = safeCall(ctx, any(payload), hAny)
			}
			if err != nil {
				fmt.Printf("[tevent] async handler error for %s: %v\n", ev.name, err)
			}
			if s.once {
				Unsubscribe(b, ev.name, s.id)
			}
		}(s, h, hAny)
	}
}

// Wait 阻塞直到所有异步分发完成。
func (b *Bus) Wait() { b.wg.Wait() }

// safeCall 调用监听器并 recover panic。
func safeCall[T any](ctx context.Context, payload T, h Handler[T]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tevent handler panic: %v", r)
		}
	}()
	return h(ctx, payload)
}

// Len 返回某事件的监听器数量（测试/调试用）。
func (b *Bus) Len(name string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs[name])
}
