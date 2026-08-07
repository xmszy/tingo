// Package redis 提供基于 go-redis/v9 的 Redis 客户端封装。
//
// 提供统一的 Redis 接口，支持单机、集群、哨兵三种模式，
// 并包装了字符串、哈希、列表、集合、有序集合、发布订阅等常用操作。
//
// 使用示例：
//
//	rds := redis.New(redis.Config{
//	    Addr:     "127.0.0.1:6379",
//	    Password: "",
//	    DB:       0,
//	})
package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config Redis 连接配置。
type Config struct {
	// Addr 单节点地址（host:port）
	Addr string `json:"addr" toml:"addr"`
	// ClusterAddrs 集群节点地址列表
	ClusterAddrs []string `json:"cluster_addrs" toml:"cluster_addrs"`
	// SentinelAddrs 哨兵节点地址列表
	SentinelAddrs []string `json:"sentinel_addrs" toml:"sentinel_addrs"`
	// SentinelMaster 哨兵主节点名
	SentinelMaster string `json:"sentinel_master" toml:"sentinel_master"`
	// Password 认证密码
	Password string `json:"password" toml:"password"`
	// DB 数据库编号（单机模式）
	DB int `json:"db" toml:"db"`
	// DialTimeout 连接超时
	DialTimeout time.Duration `json:"dial_timeout" toml:"dial_timeout"`
	// ReadTimeout 读超时
	ReadTimeout time.Duration `json:"read_timeout" toml:"read_timeout"`
	// WriteTimeout 写超时
	WriteTimeout time.Duration `json:"write_timeout" toml:"write_timeout"`
	// PoolSize 连接池大小
	PoolSize int `json:"pool_size" toml:"pool_size"`
	// MinIdleConns 最小空闲连接数
	MinIdleConns int `json:"min_idle_conns" toml:"min_idle_conns"`
}

// Redis 提供统一的 Redis 操作方法。
type Redis struct {
	client goredis.Cmdable
}

// New 创建一个 Redis 客户端。
// 根据 Config 自动选择单机、集群或哨兵模式。
func New(cfg Config) *Redis {
	var client goredis.Cmdable
	switch {
	case len(cfg.SentinelAddrs) > 0 && cfg.SentinelMaster != "":
		client = goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:       cfg.SentinelMaster,
			SentinelAddrs:    cfg.SentinelAddrs,
			SentinelPassword: cfg.Password,
			Password:         cfg.Password,
			DB:               cfg.DB,
			DialTimeout:      cfg.DialTimeout,
			ReadTimeout:      cfg.ReadTimeout,
			WriteTimeout:     cfg.WriteTimeout,
			PoolSize:         cfg.PoolSize,
			MinIdleConns:     cfg.MinIdleConns,
		})
	case len(cfg.ClusterAddrs) > 0:
		client = goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:        cfg.ClusterAddrs,
			Password:     cfg.Password,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
		})
	default:
		addr := cfg.Addr
		if addr == "" {
			addr = "127.0.0.1:6379"
		}
		client = goredis.NewClient(&goredis.Options{
			Addr:         addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
		})
	}
	return &Redis{client: client}
}

// Client 返回原始 go-redis 客户端（用于透传高级操作）。
func (r *Redis) Client() goredis.Cmdable { return r.client }

// ──────────────── 字符串操作 ────────────────

// Set 设置字符串值。
func (r *Redis) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取字符串值。
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// SetNX 仅当 key 不存在时设置（用于分布式锁）。
func (r *Redis) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

// Del 删除一个或多个 key。
func (r *Redis) Del(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Del(ctx, keys...).Result()
}

// Exists 检查 key 是否存在。
func (r *Redis) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// Expire 设置 key 过期时间。
func (r *Redis) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, expiration).Result()
}

// TTL 获取 key 剩余过期时间。
func (r *Redis) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Incr 自增 1。
func (r *Redis) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Decr 自减 1。
func (r *Redis) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// IncrBy 自增指定步长。
func (r *Redis) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

// ──────────────── 哈希操作 ────────────────

// HSet 设置哈希字段。
func (r *Redis) HSet(ctx context.Context, key string, values ...any) (int64, error) {
	return r.client.HSet(ctx, key, values...).Result()
}

// HGet 获取哈希字段值。
func (r *Redis) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

// HGetAll 获取哈希所有字段。
func (r *Redis) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HDel 删除哈希字段。
func (r *Redis) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return r.client.HDel(ctx, key, fields...).Result()
}

// HExists 检查哈希字段是否存在。
func (r *Redis) HExists(ctx context.Context, key, field string) (bool, error) {
	return r.client.HExists(ctx, key, field).Result()
}

// HKeys 获取哈希所有字段名。
func (r *Redis) HKeys(ctx context.Context, key string) ([]string, error) {
	return r.client.HKeys(ctx, key).Result()
}

// HLen 获取哈希字段数。
func (r *Redis) HLen(ctx context.Context, key string) (int64, error) {
	return r.client.HLen(ctx, key).Result()
}

// ──────────────── 列表操作 ────────────────

// LPush 左侧入队。
func (r *Redis) LPush(ctx context.Context, key string, values ...any) (int64, error) {
	return r.client.LPush(ctx, key, values...).Result()
}

// RPush 右侧入队。
func (r *Redis) RPush(ctx context.Context, key string, values ...any) (int64, error) {
	return r.client.RPush(ctx, key, values...).Result()
}

// LPop 左侧出队。
func (r *Redis) LPop(ctx context.Context, key string) (string, error) {
	return r.client.LPop(ctx, key).Result()
}

// RPop 右侧出队。
func (r *Redis) RPop(ctx context.Context, key string) (string, error) {
	return r.client.RPop(ctx, key).Result()
}

// LLen 获取列表长度。
func (r *Redis) LLen(ctx context.Context, key string) (int64, error) {
	return r.client.LLen(ctx, key).Result()
}

// LRange 获取列表区间元素。
func (r *Redis) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

// ──────────────── 集合操作 ────────────────

// SAdd 添加集合元素。
func (r *Redis) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	return r.client.SAdd(ctx, key, members...).Result()
}

// SMembers 获取集合所有元素。
func (r *Redis) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// SIsMember 检查元素是否在集合中。
func (r *Redis) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// SRem 移除集合元素。
func (r *Redis) SRem(ctx context.Context, key string, members ...any) (int64, error) {
	return r.client.SRem(ctx, key, members...).Result()
}

// SCard 获取集合元素数量。
func (r *Redis) SCard(ctx context.Context, key string) (int64, error) {
	return r.client.SCard(ctx, key).Result()
}

// ──────────────── 有序集合操作 ────────────────

// ZAdd 添加有序集合元素。
func (r *Redis) ZAdd(ctx context.Context, key string, members ...goredis.Z) (int64, error) {
	return r.client.ZAdd(ctx, key, members...).Result()
}

// ZRange 获取有序集合区间元素（按分数升序）。
func (r *Redis) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

// ZRevRange 获取有序集合区间元素（按分数降序）。
func (r *Redis) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRevRange(ctx, key, start, stop).Result()
}

// ZRem 移除有序集合元素。
func (r *Redis) ZRem(ctx context.Context, key string, members ...any) (int64, error) {
	return r.client.ZRem(ctx, key, members...).Result()
}

// ZCard 获取有序集合元素数量。
func (r *Redis) ZCard(ctx context.Context, key string) (int64, error) {
	return r.client.ZCard(ctx, key).Result()
}

// ──────────────── Pub/Sub ────────────────

// Publish 发布消息到频道。
func (r *Redis) Publish(ctx context.Context, channel string, message any) (int64, error) {
	return r.client.Publish(ctx, channel, message).Result()
}

// Subscribe 订阅频道（仅单机模式支持）。
func (r *Redis) Subscribe(ctx context.Context, channels ...string) *goredis.PubSub {
	if c, ok := r.client.(*goredis.Client); ok {
		return c.Subscribe(ctx, channels...)
	}
	return nil
}

// ──────────────── 连接管理 ────────────────

// Ping 检查连接。
func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close 关闭连接。
func (r *Redis) Close() error {
	if c, ok := r.client.(*goredis.Client); ok {
		return c.Close()
	}
	if c, ok := r.client.(*goredis.ClusterClient); ok {
		return c.Close()
	}
	return nil
}
