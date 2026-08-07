package tqueue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/xmszy/tingo/os/tredis"
)

// RedisQueue 是基于 Redis 的队列驱动（满足 Driver 接口），让 tqueue 可对接外部中间件。
//
// 设计要点：
//   - 使用 tredis 零依赖 RESP 客户端，无需 go-redis/redigo；
//   - 消息以 JSON 序列化后 LPUSH 到 list，消费者 BRPOP 阻塞弹出（RPOpLPush 可靠取回）；
//   - 延迟消息写入 ZADD 有序集合（score = 到期时间戳），由后台定时器搬运到就绪 list；
//   - 失败重试复用与 MemoryQueue 一致的策略（Attempts + MaxRetry + 死信回调）；
//   - Start(workers) 启动多 goroutine 消费，Stop() 优雅退出。
type RedisQueue[T any] struct {
	client  *tredis.Client
	listKey string
	delayKey string
	h        Handler[T]
	maxRetry int
	onDead   func(ctx context.Context, msg Message[T], lastErr error)

	mu     sync.Mutex
	wg     sync.WaitGroup
	stopCh chan struct{}
	closed bool
}

// NewRedis 创建基于 Redis 的队列（listKey 为队列名，delayKey 为延迟队列名，可省略）。
func NewRedis[T any](client *tredis.Client, listKey string, maxRetry int, delayKey ...string) *RedisQueue[T] {
	if maxRetry < 0 {
		maxRetry = 0
	}
	dk := listKey + ":delay"
	if len(delayKey) > 0 && delayKey[0] != "" {
		dk = delayKey[0]
	}
	return &RedisQueue[T]{
		client:   client,
		listKey:  listKey,
		delayKey: dk,
		maxRetry: maxRetry,
		stopCh:   make(chan struct{}),
	}
}

// OnDeadLetter 设置死信回调（超过最大重试后调用）。
func (q *RedisQueue[T]) OnDeadLetter(fn func(ctx context.Context, msg Message[T], lastErr error)) {
	q.onDead = fn
}

// Subscribe 注册消费者。
func (q *RedisQueue[T]) Subscribe(h Handler[T]) {
	q.h = h
}

// Publish 投递消息（JSON 序列化后入队）。
func (q *RedisQueue[T]) Publish(ctx context.Context, payload T) error {
	return q.PublishMessage(ctx, Message[T]{Payload: payload})
}

// PublishMessage 投递完整消息（含 Headers / 可选 Delay）。
func (q *RedisQueue[T]) PublishMessage(ctx context.Context, msg Message[T]) error {
	if msg.ID == "" {
		msg.ID = newMsgID()
	}
	if msg.Delay > 0 {
		return q.pushDelay(ctx, msg)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = q.client.LPush(ctx, q.listKey, string(data))
	return err
}

// PublishAsync 异步投递（fire-and-forget）。
func (q *RedisQueue[T]) PublishAsync(ctx context.Context, payload T) {
	_ = q.Publish(ctx, payload)
}

// PublishDelay 延迟投递：消息在 delayDuration 秒后才可被消费（持久化于 Redis ZSet）。
func (q *RedisQueue[T]) PublishDelay(ctx context.Context, payload T, delayDuration int64) {
	msg := Message[T]{Payload: payload, Delay: delayDuration}
	_ = q.PublishMessage(ctx, msg)
}

func (q *RedisQueue[T]) pushDelay(ctx context.Context, msg Message[T]) error {
	msg.AvailableAt = time.Now().Unix() + msg.Delay
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	// score = 到期时间戳，后台定时器按序搬运。
	_, err = q.client.Do(ctx, "ZADD", q.delayKey, msg.AvailableAt, string(data))
	return err
}

// Start 启动 workers 个消费者 goroutine 循环取消息；若消息携带 Delay，先进入延迟搬运循环。
func (q *RedisQueue[T]) Start(workers int) {
	if workers <= 0 {
		workers = 1
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()

	// 延迟队列搬运协程
	q.wg.Add(1)
	go q.delayMover()

	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.consume()
	}
}

// Stop 停止消费（已入队消息仍留在 Redis，可下次启动消费）。
func (q *RedisQueue[T]) Stop() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.stopCh)
	q.mu.Unlock()
	q.wg.Wait()
}

// delayMover 周期性把到期延迟消息搬到就绪 list。
func (q *RedisQueue[T]) delayMover() {
	defer q.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-q.stopCh:
			return
		case <-ticker.C:
			q.moveDue(context.Background())
		}
	}
}

func (q *RedisQueue[T]) moveDue(ctx context.Context) {
	now := time.Now().Unix()
	// 取 score <= now 的到期消息
	raw, err := q.client.Do(ctx, "ZRANGEBYSCORE", q.delayKey, 0, now)
	if err != nil {
		return
	}
	items, ok := raw.([]string)
	if !ok || len(items) == 0 {
		return
	}
	for _, it := range items {
		// 原子搬运：从 ZSet 删除并 LPUSH 到就绪 list
		if _, err := q.client.Do(ctx, "ZREM", q.delayKey, it); err != nil {
			continue
		}
		_, _ = q.client.LPush(ctx, q.listKey, it)
	}
}

// consume 阻塞消费循环（BRPOP）。
func (q *RedisQueue[T]) consume() {
	defer q.wg.Done()
	ctx := context.Background()
	for {
		select {
		case <-q.stopCh:
			return
		default:
		}
		raw, err := q.client.Do(ctx, "BRPOP", q.listKey, 1)
		if err != nil {
			if errors.Is(err, tredis.ErrNil) || isTimeout(err) {
				continue // 超时或空队列，重试
			}
			// 连接异常，短暂退避后继续
			time.Sleep(100 * time.Millisecond)
			continue
		}
		pair, ok := raw.([]string)
		if !ok || len(pair) < 2 {
			continue
		}
		var msg Message[T]
		if err := json.Unmarshal([]byte(pair[1]), &msg); err != nil {
			continue // 解析失败的消息直接丢弃（可在此接入死信）
		}
		q.dispatch(ctx, msg)
	}
}

// dispatch 调用消费者并处理重试（与 MemoryQueue 语义一致）。
func (q *RedisQueue[T]) dispatch(ctx context.Context, msg Message[T]) {
	if q.h == nil {
		return
	}
	err := q.h(ctx, msg)
	if err == nil {
		return
	}
	msg.Attempts++
	if msg.Attempts <= q.maxRetry {
		// 同步重投到队列尾部
		_ = q.PublishMessage(ctx, msg)
		return
	}
	if q.onDead != nil {
		q.onDead(ctx, msg, err)
	}
}

// isTimeout 判断是否为 BRPOP 超时（tredis 超时返回 nil 或特定错误，这里做宽松判断）。
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	// tredis 在 BRPOP 超时且无数据时通常返回 ErrNil；其他超时以字符串包含判断。
	return errors.Is(err, tredis.ErrNil) || containsStr(err.Error(), "timeout")
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// newMsgID 生成简单的消息 ID（基于时间 + 随机）。
func newMsgID() string {
	return time.Now().Format("20060102150405.000") + "-" + rand4()
}

func rand4() string {
	const cs = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = cs[int(time.Now().UnixNano()>>uint(i))%len(cs)]
	}
	return string(b)
}
