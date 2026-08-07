// Package tqueue 提供零外部依赖的轻量任务队列。
//
// 设计要点：
//   - 内置 MemoryQueue：基于 tevent 事件总线解耦生产者与消费者；
//   - 消息为泛型 Message[T]，含 ID、Payload、Attempts；
//   - 支持 worker 并发消费（带限流信号量）；
//   - 失败重试（MaxRetry）+ 死信回调；
//   - 所有组件零外部依赖，可直接替换为 Redis/Kafka 驱动（实现 Driver 接口）。
package tqueue

import (
	"context"
	"time"

	"github.com/xmszy/tingo/os/tevent"
)

// Message 是队列消息。
// Headers 携带请求级元数据（traceID、用户ID 等），由业务方自行写入和读取。
type Message[T any] struct {
	ID       string
	Payload  T
	Attempts int
	Headers  map[string]string
	// Delay 指定消息延迟多久后才可被消费（秒）。0 表示立即可消费。
	Delay int64
	// AvailableAt 消息可消费的时间戳（由 PublishDelay 计算填充）。
	AvailableAt int64
}

// GetHeader 安全读取 header 值。
func (m *Message[T]) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// SetHeader 设置 header 值。
func (m *Message[T]) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	m.Headers[key] = value
}

// Handler 是消费者处理函数，返回 error 表示处理失败（将重试）。
type Handler[T any] func(ctx context.Context, msg Message[T]) error

// Driver 是可替换的底层队列驱动接口（扩展点）。
type Driver[T any] interface {
	Publish(ctx context.Context, msg Message[T]) error
	// Subscribe 注册消费逻辑；start 控制是否立即启动消费循环。
	Subscribe(h Handler[T])
	Start(workers int)
	Stop()
}

// MemoryQueue 是基于 tevent 的内存队列。
type MemoryQueue[T any] struct {
	bus     *tevent.Bus
	ev      tevent.Event[Message[T]]
	subID   uint64
	h       Handler[T]
	maxRetry int
	onDead  func(ctx context.Context, msg Message[T], lastErr error)
}

// NewMemory 创建内存队列。async=true 时消费者在独立 goroutine 执行。
func NewMemory[T any](async bool, maxRetry int) *MemoryQueue[T] {
	if maxRetry < 0 {
		maxRetry = 0
	}
	return &MemoryQueue[T]{
		bus:      tevent.NewBus(async),
		ev:       tevent.New[Message[T]]("tqueue.msg"),
		maxRetry: maxRetry,
	}
}

// OnDeadLetter 设置死信回调（超过最大重试后调用）。
func (q *MemoryQueue[T]) OnDeadLetter(fn func(ctx context.Context, msg Message[T], lastErr error)) {
	q.onDead = fn
}

// Subscribe 注册消费者。
func (q *MemoryQueue[T]) Subscribe(h Handler[T]) {
	q.h = h
	q.subID = tevent.Subscribe(q.bus, q.ev, func(ctx context.Context, msg Message[T]) error {
		return q.dispatch(ctx, msg)
	})
}

// dispatch 调用消费者并处理重试。
func (q *MemoryQueue[T]) dispatch(ctx context.Context, msg Message[T]) error {
	err := q.h(ctx, msg)
	if err == nil {
		return nil
	}
	msg.Attempts++
	if msg.Attempts <= q.maxRetry {
		// 同步重投（退避策略由调用方在 Handler 内控制）。
		return tevent.Dispatch(q.bus, ctx, q.ev, msg)
	}
	if q.onDead != nil {
		q.onDead(ctx, msg, err)
	}
	return nil
}

// Publish 投递消息（同步分发，消费者即时执行）。
func (q *MemoryQueue[T]) Publish(ctx context.Context, payload T) error {
	msg := Message[T]{Payload: payload}
	return tevent.Dispatch(q.bus, ctx, q.ev, msg)
}

// PublishMessage 投递完整消息（含 Headers），用于携带请求级元数据如 traceID。
func (q *MemoryQueue[T]) PublishMessage(ctx context.Context, msg Message[T]) error {
	return tevent.Dispatch(q.bus, ctx, q.ev, msg)
}

// PublishAsync 异步投递（仅 Bus async=true 时真正异步）。
func (q *MemoryQueue[T]) PublishAsync(ctx context.Context, payload T) {
	msg := Message[T]{Payload: payload}
	tevent.DispatchAsync(q.bus, ctx, q.ev, msg)
}

// PublishMessageAsync 异步投递完整消息。
func (q *MemoryQueue[T]) PublishMessageAsync(ctx context.Context, msg Message[T]) {
	tevent.DispatchAsync(q.bus, ctx, q.ev, msg)
}

// PublishDelay 延迟投递：消息在 delayDuration 秒后才可被消费。
//
// 实现：内部启动一个 timer，到期时才真正发布到总线。
// 注意：进程重启会丢失未到期的延迟消息；需要持久化场景请使用 Redis/Kafka 驱动。
func (q *MemoryQueue[T]) PublishDelay(ctx context.Context, payload T, delayDuration int64) {
	msg := Message[T]{Payload: payload, Delay: delayDuration}
	q.publishWithDelay(ctx, msg)
}

// PublishMessageDelay 延迟投递完整消息（含 Headers）。
func (q *MemoryQueue[T]) PublishMessageDelay(ctx context.Context, msg Message[T], delayDuration int64) {
	msg.Delay = delayDuration
	q.publishWithDelay(ctx, msg)
}

// publishWithDelay 延迟发布实现。
func (q *MemoryQueue[T]) publishWithDelay(ctx context.Context, msg Message[T]) {
	if msg.Delay <= 0 {
		tevent.DispatchAsync(q.bus, ctx, q.ev, msg)
		return
	}
	// 使用独立 goroutine 等待到期后再发布
	go func() {
		<-time.After(time.Duration(msg.Delay) * time.Second)
		msg.AvailableAt = time.Now().Unix()
		msg.Delay = 0 // 清除延迟标记，避免重试时再次延迟
		tevent.DispatchAsync(q.bus, context.Background(), q.ev, msg)
	}()
}

// Wait 等待所有异步分发完成。
func (q *MemoryQueue[T]) Wait() { q.bus.Wait() }
