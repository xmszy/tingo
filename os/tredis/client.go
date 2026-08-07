// Package tredis 提供 Redis 客户端封装。
//
// 设计要点：
//   - 零外部依赖：直接实现 Redis RESP 协议，无需 go-redis 或 redigo。
//   - 连接池 + 自动重连 + 管道 + 订阅。
//   - 接口抽象：通过 Dialer 接口支持直连/哨兵/集群。
//
// 性能：基于 net.Conn 裸读写，RESP 协议手工编解码，零反射零中间介质。
package tredis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ──────────────── 错误 ────────────────

var (
	ErrPoolExhausted = errors.New("tredis: connection pool exhausted")
	ErrClosed        = errors.New("tredis: client closed")
	ErrNil           = errors.New("tredis: nil")
)

// ──────────────── 配置 ────────────────

// Config Redis 连接配置。
type Config struct {
	Addr            string        `json:"addr" toml:"addr" yaml:"addr"`                        // 默认 "127.0.0.1:6379"
	Password        string        `json:"password" toml:"password" yaml:"password"`            // 密码
	DB              int           `json:"db" toml:"db" yaml:"db"`                              // 数据库编号
	PoolSize        int           `json:"pool_size" toml:"pool_size" yaml:"pool_size"`         // 连接池大小，默认 10
	MinIdle         int           `json:"min_idle" toml:"min_idle" yaml:"min_idle"`            // 最小空闲连接数
	DialTimeout     time.Duration `json:"dial_timeout" toml:"dial_timeout" yaml:"dial_timeout"` // 连接超时，默认 5s
	ReadTimeout     time.Duration `json:"read_timeout" toml:"read_timeout" yaml:"read_timeout"` // 读超时，默认 3s
	WriteTimeout    time.Duration `json:"write_timeout" toml:"write_timeout" yaml:"write_timeout"` // 写超时，默认 3s
	MaxRetries      int           `json:"max_retries" toml:"max_retries" yaml:"max_retries"`   // 最大重试次数
	IdleTimeout     time.Duration `json:"idle_timeout" toml:"idle_timeout" yaml:"idle_timeout"` // 空闲超时，默认 5min
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		Addr:         "127.0.0.1:6379",
		PoolSize:     10,
		MinIdle:      2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		MaxRetries:   3,
		IdleTimeout:  5 * time.Minute,
	}
}

// ──────────────── Client ────────────────

// Client Redis 客户端。
type Client struct {
	cfg    Config
	pool   *connPool
	closed atomic.Bool
	mu     sync.Mutex
}

// New 创建 Redis 客户端。
func New(cfg Config) *Client {
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = 10
	}
	if cfg.MinIdle <= 0 {
		cfg.MinIdle = cfg.PoolSize / 5
	}
	if cfg.MinIdle > cfg.PoolSize {
		cfg.MinIdle = cfg.PoolSize
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	return &Client{
		cfg:  cfg,
		pool: newConnPool(cfg),
	}
}

// Close 关闭客户端和所有连接。
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	return c.pool.Close()
}

// Do 执行任意 Redis 命令。
func (c *Client) Do(ctx context.Context, cmd string, args ...any) (any, error) {
	return c.exec(ctx, cmd, args)
}

// Pipe 管线操作。cmds 每个元素为 [2]any{命令名, 参数}。
// 例: c.Pipe(ctx, [2]any{"SET", []any{"key", "val"}}, [2]any{"GET", []any{"key"}})
func (c *Client) Pipe(ctx context.Context, cmds ...[2]any) ([]any, error) {
	return c.pipe(ctx, cmds...)
}

// ──────────────── Key 操作 ────────────────

// Set 设置 key。
func (c *Client) Set(ctx context.Context, key string, value any) error { _, err := c.Do(ctx, "SET", key, value); return err }

// SetEX 设置带过期时间的 key。
func (c *Client) SetEX(ctx context.Context, key string, value any, ttl time.Duration) error {
	_, err := c.Do(ctx, "SETEX", key, int(ttl.Seconds()), value)
	return err
}

// Get 获取 key。
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	v, err := c.Do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", ErrNil
	}
	switch s := v.(type) {
	case string:
		return s, nil
	case []byte:
		return string(s), nil
	default:
		return fmt.Sprint(v), nil
	}
}

// Del 删除 key。
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	v, err := c.Do(ctx, "DEL", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// Exists 检查 key 是否存在。
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	v, err := c.Do(ctx, "EXISTS", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// Expire 设置过期时间。
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	v, err := c.Do(ctx, "EXPIRE", key, int(ttl.Seconds()))
	if err != nil {
		return false, err
	}
	return toInt64(v) == 1, nil
}

// TTL 获取剩余时间。
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	v, err := c.Do(ctx, "TTL", key)
	if err != nil {
		return 0, err
	}
	sec := toInt64(v)
	if sec < 0 {
		return time.Duration(sec) * time.Second, nil
	}
	return time.Duration(sec) * time.Second, nil
}

// Keys 模式匹配。
func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	v, err := c.Do(ctx, "KEYS", pattern)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		result = append(result, toString(item))
	}
	return result, nil
}

// Incr 自增。
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	v, err := c.Do(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// Decr 自减。
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	v, err := c.Do(ctx, "DECR", key)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// IncrBy 按步长自增。
func (c *Client) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	v, err := c.Do(ctx, "INCRBY", key, delta)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// ──────────────── Hash 操作 ────────────────

// HSet 设置 Hash 字段。
func (c *Client) HSet(ctx context.Context, key string, values ...string) (int64, error) {
	args := make([]any, 1+len(values))
	args[0] = key
	for i, v := range values {
		args[i+1] = v
	}
	v, err := c.Do(ctx, "HSET", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// HGet 获取 Hash 字段。
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	v, err := c.Do(ctx, "HGET", key, field)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", ErrNil
	}
	return toString(v), nil
}

// HGetAll 获取整个 Hash。
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	v, err := c.Do(ctx, "HGETALL", key)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, nil
	}
	result := make(map[string]string, len(arr)/2)
	for i := 0; i < len(arr)-1; i += 2 {
		result[toString(arr[i])] = toString(arr[i+1])
	}
	return result, nil
}

// HDel 删除 Hash 字段。
func (c *Client) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	args := make([]any, 1+len(fields))
	args[0] = key
	for i, f := range fields {
		args[i+1] = f
	}
	v, err := c.Do(ctx, "HDEL", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// HExists 检查 Hash 字段是否存在。
func (c *Client) HExists(ctx context.Context, key, field string) (bool, error) {
	v, err := c.Do(ctx, "HEXISTS", key, field)
	if err != nil {
		return false, err
	}
	return toInt64(v) == 1, nil
}

// HLen 获取 Hash 字段数。
func (c *Client) HLen(ctx context.Context, key string) (int64, error) {
	v, err := c.Do(ctx, "HLEN", key)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// ──────────────── List 操作 ────────────────

// LPush 左侧插入。
func (c *Client) LPush(ctx context.Context, key string, values ...any) (int64, error) {
	args := make([]any, 1+len(values))
	args[0] = key
	copy(args[1:], values)
	v, err := c.Do(ctx, "LPUSH", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// RPush 右侧插入。
func (c *Client) RPush(ctx context.Context, key string, values ...any) (int64, error) {
	args := make([]any, 1+len(values))
	args[0] = key
	copy(args[1:], values)
	v, err := c.Do(ctx, "RPUSH", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// LPop 左侧弹出。
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	v, err := c.Do(ctx, "LPOP", key)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", ErrNil
	}
	return toString(v), nil
}

// RPop 右侧弹出。
func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	v, err := c.Do(ctx, "RPOP", key)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", ErrNil
	}
	return toString(v), nil
}

// LRange 获取列表范围。
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	v, err := c.Do(ctx, "LRANGE", key, start, stop)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		result = append(result, toString(item))
	}
	return result, nil
}

// LLen 获取列表长度。
func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	v, err := c.Do(ctx, "LLEN", key)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// ──────────────── Set 操作 ────────────────

// SAdd 添加成员。
func (c *Client) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	args := make([]any, 1+len(members))
	args[0] = key
	copy(args[1:], members)
	v, err := c.Do(ctx, "SADD", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// SMembers 获取所有成员。
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	v, err := c.Do(ctx, "SMEMBERS", key)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		result = append(result, toString(item))
	}
	return result, nil
}

// SIsMember 判断是否为成员。
func (c *Client) SIsMember(ctx context.Context, key, member string) (bool, error) {
	v, err := c.Do(ctx, "SISMEMBER", key, member)
	if err != nil {
		return false, err
	}
	return toInt64(v) == 1, nil
}

// SRem 移除成员。
func (c *Client) SRem(ctx context.Context, key string, members ...any) (int64, error) {
	args := make([]any, 1+len(members))
	args[0] = key
	copy(args[1:], members)
	v, err := c.Do(ctx, "SREM", args...)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// SCard 获取集合大小。
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	v, err := c.Do(ctx, "SCARD", key)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// ──────────────── Pub/Sub ────────────────

// Publish 发布消息。
func (c *Client) Publish(ctx context.Context, channel, message string) (int64, error) {
	v, err := c.Do(ctx, "PUBLISH", channel, message)
	if err != nil {
		return 0, err
	}
	return toInt64(v), nil
}

// Subscription 订阅消息。
type Subscription struct {
	Channel string
	Message string
	Err     error
}

// Subscribe 订阅频道，返回 channel 接收消息。调用方 cancel ctx 即可取消。
func (c *Client) Subscribe(ctx context.Context, channels ...string) (<-chan Subscription, error) {
	if len(channels) == 0 {
		return nil, errors.New("tredis: no channels specified")
	}
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	// 发送 SUBSCRIBE 命令
	args := make([]string, 0, len(channels)+1)
	args = append(args, "*"+strconv.Itoa(len(channels)+1))
	args = append(args, "$"+strconv.Itoa(len("SUBSCRIBE")))
	args = append(args, "SUBSCRIBE")
	for _, ch := range channels {
		args = append(args, "$"+strconv.Itoa(len(ch)))
		args = append(args, ch)
	}
	if _, err := fmt.Fprintf(conn, "%s\r\n", strings.Join(args, "\r\n")); err != nil {
		conn.Close()
		return nil, err
	}

	subCh := make(chan Subscription, 64)
	go func() {
		defer close(subCh)
		defer conn.Close()

		reader := bufio.NewReader(conn)
		for {
			select {
			case <-ctx.Done():
				// 发送 UNSUBSCRIBE
				fmt.Fprintf(conn, "*2\r\n$11\r\nUNSUBSCRIBE\r\n$1\r\n*\r\n")
				return
			default:
			}
			resp, err := readRESP(reader)
			if err != nil {
				select {
				case subCh <- Subscription{Err: err}:
				case <-ctx.Done():
					return
				}
				return
			}
			arr, ok := resp.([]any)
			if !ok || len(arr) < 3 {
				continue
			}
			msgType := toString(arr[0])
			if msgType != "message" {
				continue
			}
			select {
			case subCh <- Subscription{
				Channel: toString(arr[1]),
				Message: toString(arr[2]),
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return subCh, nil
}

// ──────────────── 结果解析 ────────────────

func toInt64(v any) int64 {
	switch s := v.(type) {
	case int64:
		return s
	case string:
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(s), 10, 64)
		return n
	}
	return 0
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprint(v)
	}
}

// ──────────────── RESP 协议实现 ────────────────

func (c *Client) exec(ctx context.Context, cmd string, args []any) (any, error) {
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		conn, err := c.pool.Get()
		if err != nil {
			return nil, err
		}
		result, err := c.writeCmd(conn, cmd, args)
		if err != nil {
			conn.Close()
			c.pool.Put(nil) // 归还空位
			if i < c.cfg.MaxRetries && !c.closed.Load() {
				time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
				continue
			}
			return nil, err
		}
		c.pool.Put(conn)
		return result, nil
	}
}

func (c *Client) pipe(ctx context.Context, cmds ...[2]any) ([]any, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return nil, err
	}
	defer c.pool.Put(conn)

	reader := bufio.NewReader(conn)
	results := make([]any, 0, len(cmds))

	for _, cmd := range cmds {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		cmdName := toString(cmd[0])
		cmdArgs := cmd[1]
		var args []any
		switch v := cmdArgs.(type) {
		case []any:
			args = v
		default:
			args = []any{v}
		}
		if _, err := c.buildRESP(conn, cmdName, args); err != nil {
			return nil, err
		}
		res, err := readRESP(reader)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (c *Client) writeCmd(conn net.Conn, cmd string, args []any) (any, error) {
	if _, err := c.buildRESP(conn, cmd, args); err != nil {
		return nil, err
	}
	return readRESP(bufio.NewReader(conn))
}

// buildRESP 构建 RESP 协议写入。
func (c *Client) buildRESP(conn net.Conn, cmd string, args []any) (int, error) {
	conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	var b strings.Builder
	b.WriteByte('*')
	b.WriteString(strconv.Itoa(1 + len(args)))
	b.WriteString("\r\n")
	writeBulk(&b, []byte(cmd))
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			writeBulk(&b, []byte(v))
		case []byte:
			writeBulk(&b, v)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			writeBulk(&b, fmt.Appendf(nil, "%d", v))
		case float32, float64:
			writeBulk(&b, fmt.Appendf(nil, "%v", v))
		default:
			writeBulk(&b, fmt.Appendf(nil, "%v", v))
		}
	}
	return io.WriteString(conn, b.String())
}

func writeBulk(b *strings.Builder, data []byte) {
	b.WriteByte('$')
	b.WriteString(strconv.Itoa(len(data)))
	b.WriteString("\r\n")
	b.Write(data)
	b.WriteString("\r\n")
}

// readRESP 读取 RESP 协议响应。
func readRESP(reader *bufio.Reader) (any, error) {
	line, err := readLine(reader)
	if err != nil {
		return nil, err
	}
	if len(line) == 0 {
		return nil, errors.New("tredis: empty response")
	}
	switch line[0] {
	case '+': // 简单字符串
		return string(line[1:]), nil
	case '-': // 错误
		return nil, errors.New(string(line[1:]))
	case ':': // 整数
		return strconv.ParseInt(string(line[1:]), 10, 64)
	case '$': // 批量字符串
		n, _ := strconv.Atoi(string(line[1:]))
		if n < 0 {
			return nil, nil // null
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		return buf[:n], nil
	case '*': // 数组
		n, _ := strconv.Atoi(string(line[1:]))
		if n < 0 {
			return nil, nil
		}
		arr := make([]any, 0, n)
		for i := 0; i < n; i++ {
			v, err := readRESP(reader)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	default:
		// 内联命令
		return string(line), nil
	}
}

func readLine(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		line = append(line, b)
		if len(line) >= 2 && line[len(line)-2] == '\r' && line[len(line)-1] == '\n' {
			return line[:len(line)-2], nil
		}
	}
}

// ──────────────── 连接池 ────────────────

type connPool struct {
	cfg     Config
	ch      chan net.Conn
	created int32
	mu      sync.Mutex
}

func newConnPool(cfg Config) *connPool {
	p := &connPool{
		cfg: cfg,
		ch:  make(chan net.Conn, cfg.PoolSize),
	}
	// 预热最小空闲连接
	for i := 0; i < cfg.MinIdle; i++ {
		if conn, err := p.dial(); err == nil {
			p.ch <- conn
			atomic.AddInt32(&p.created, 1)
		}
	}
	return p
}

func (p *connPool) Get() (net.Conn, error) {
	select {
	case conn := <-p.ch:
		return conn, nil
	default:
	}
	created := atomic.LoadInt32(&p.created)
	if created < int32(p.cfg.PoolSize) {
		conn, err := p.dial()
		if err == nil {
			atomic.AddInt32(&p.created, 1)
			return conn, nil
		}
	}
	// 等待归还
	select {
	case conn := <-p.ch:
		return conn, nil
	case <-time.After(200 * time.Millisecond):
		return nil, ErrPoolExhausted
	}
}

func (p *connPool) Put(conn net.Conn) {
	if conn == nil {
		return
	}
	select {
	case p.ch <- conn:
	default:
		conn.Close()
	}
}

func (p *connPool) Close() error {
	close(p.ch)
	var lastErr error
	for conn := range p.ch {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (p *connPool) dial() (net.Conn, error) {
	return p.cfg.Dial()
}

// Dial 创建新连接。
func (cfg Config) Dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", cfg.Addr, cfg.DialTimeout)
	if err != nil {
		return nil, err
	}
	if cfg.Password != "" {
		if _, err := authRESP(conn, cfg.Password); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if cfg.DB > 0 {
		if _, err := selectRESP(conn, cfg.DB); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

func authRESP(conn net.Conn, password string) (any, error) {
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(conn, "*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(password), password)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return readRESP(bufio.NewReader(conn))
}

func selectRESP(conn net.Conn, db int) (any, error) {
	dbStr := strconv.Itoa(db)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(conn, "*2\r\n$6\r\nSELECT\r\n$%d\r\n%s\r\n", len(dbStr), dbStr)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return readRESP(bufio.NewReader(conn))
}

// dial Redis 直连（供 Client 内部使用）。
func (c *Client) dial() (net.Conn, error) {
	return c.cfg.Dial()
}
