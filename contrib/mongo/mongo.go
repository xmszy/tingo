// Package mongo 提供 MongoDB 客户端便捷封装。
//
// 基于 go.mongodb.org/mongo-driver/v2，提供：
//   - 连接池管理（单例 Client）
//   - Database/Collection 快捷获取
//   - Ping 健康检查
//   - 连接断开处理
//
// 用法：
//
//	client, _ := mongo.Connect(context.Background(), "mongodb://localhost:27017")
//	coll := client.Database("mydb").Collection("users")
//	_, _ = coll.InsertOne(ctx, bson.M{"name": "test"})
package mongo

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	clientsMu sync.RWMutex
	clients   = map[string]*Client{}
)

// Client 封装 MongoDB 连接。
type Client struct {
	*mongo.Client
	name string
	uri  string
}

// Config 连接配置。
type Config struct {
	// URI 连接字符串，如 "mongodb://localhost:27017"。
	URI string
	// Name 连接别名，默认 "default"。
	Name string
	// MinPoolSize 最小连接池大小。
	MinPoolSize uint64
	// MaxPoolSize 最大连接池大小，默认 100。
	MaxPoolSize uint64
	// ConnectTimeout 连接超时，默认 10 秒。
	ConnectTimeout time.Duration
	// MaxIdleTime 连接最大空闲时间。
	MaxIdleTime time.Duration
}

func (c *Config) norm() {
	if c.Name == "" {
		c.Name = "default"
	}
	if c.MaxPoolSize == 0 {
		c.MaxPoolSize = 100
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 * time.Second
	}
}

// Connect 连接 MongoDB 并注册为全局可复用客户端。
func Connect(ctx context.Context, cfg Config) (*Client, error) {
	cfg.norm()

	opts := options.Client().
		ApplyURI(cfg.URI).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetConnectTimeout(cfg.ConnectTimeout)
	if cfg.MaxIdleTime > 0 {
		opts.SetMaxConnIdleTime(cfg.MaxIdleTime)
	}

	c, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	tctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := c.Ping(tctx, nil); err != nil {
		return nil, err
	}

	client := &Client{Client: c, name: cfg.Name, uri: cfg.URI}
	clientsMu.Lock()
	clients[cfg.Name] = client
	clientsMu.Unlock()
	return client, nil
}

// ClientOf 获取已注册的 MongoDB 客户端（name="default"）。
func ClientOf(name ...string) *Client {
	n := "default"
	if len(name) > 0 && name[0] != "" {
		n = name[0]
	}
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	return clients[n]
}

// Disconnect 断开所有已注册客户端。
func DisconnectAll(ctx context.Context) error {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	var errs []error
	for name, c := range clients {
		if err := c.Client.Disconnect(ctx); err != nil {
			errs = append(errs, err)
		}
		delete(clients, name)
	}
	return errors.Join(errs...)
}

// DB 快捷获取数据库。
func (c *Client) DB(name string) *mongo.Database {
	return c.Client.Database(name)
}

// Ping 健康检查。
func (c *Client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx, nil)
}

// EnsureIndex 确保集合存在指定索引（不存在则创建）。
func EnsureIndex(ctx context.Context, coll *mongo.Collection, indexes []mongo.IndexModel) error {
	if len(indexes) == 0 {
		return nil
	}
	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

// BSON 工具：M 和 D 类型别名，减少 import 路径。
type (
	M = bson.M
	D = bson.D
	A = bson.A
	// ObjectID 类型别名。
	ObjectID = bson.ObjectID
)

// ErrNoDocuments 表示查询未找到文档。
var ErrNoDocuments = mongo.ErrNoDocuments
