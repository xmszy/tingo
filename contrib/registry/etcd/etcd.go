// Package etcd 提供基于 etcd 的注册中心实现。
//
// 需要：go get go.etcd.io/etcd/client/v3
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/xmszy/tingo/contrib/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdRegistry 基于 etcd 的服务注册发现实现。
type etcdRegistry struct {
	client    *clientv3.Client
	prefix    string // etcd key 前缀，如 /services/
	leaseID   clientv3.LeaseID
	leaseTTL  int64 // 租约 TTL 秒数
	keepAlive <-chan *clientv3.LeaseKeepAliveResponse
	mu        sync.Mutex
}

// Option 配置选项。
type Option func(*etcdRegistry)

// WithPrefix 设置 etcd key 前缀，默认为 /services/。
func WithPrefix(prefix string) Option {
	return func(r *etcdRegistry) { r.prefix = prefix }
}

// WithLeaseTTL 设置租约 TTL 秒数，默认为 10 秒。
func WithLeaseTTL(ttl int64) Option {
	return func(r *etcdRegistry) { r.leaseTTL = ttl }
}

// New 创建 etcd 注册中心。
//
// 使用示例：
//
//	cli, _ := etcd.New(clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}})
//	reg := etcd.New(cli, etcd.WithPrefix("/myapp/"), etcd.WithLeaseTTL(15))
func New(client *clientv3.Client, opts ...Option) registry.Registry {
	r := &etcdRegistry{
		client:   client,
		prefix:   "/services/",
		leaseTTL: 10,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// serviceKey 构建 etcd key：{prefix}{serviceName}/{address}
func (r *etcdRegistry) serviceKey(name, address string) string {
	return path.Join(r.prefix, name, strings.ReplaceAll(address, ":", "_"))
}

// servicePrefix 构建服务名前缀（用于列出所有实例）。
func (r *etcdRegistry) servicePrefix(name string) string {
	return path.Join(r.prefix, name) + "/"
}

// Register 注册服务实例并启动自动续约。
func (r *etcdRegistry) Register(inst registry.Instance) error {
	if inst.TTL <= 0 {
		inst.TTL = time.Duration(r.leaseTTL) * time.Second
	}
	key := r.serviceKey(inst.Name, inst.Address)
	data, err := json.Marshal(inst)
	if err != nil {
		return fmt.Errorf("etcd register marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 申请租约
	leaseResp, err := r.client.Grant(ctx, int64(inst.TTL.Seconds()))
	if err != nil {
		return fmt.Errorf("etcd register grant lease: %w", err)
	}

	// 写入 key
	_, err = r.client.Put(ctx, key, string(data), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return fmt.Errorf("etcd register put: %w", err)
	}

	// 启动心跳续约
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.keepAlive != nil {
		r.client.Revoke(context.Background(), r.leaseID)
	}
	r.leaseID = leaseResp.ID
	ch, err := r.client.KeepAlive(context.Background(), leaseResp.ID)
	if err != nil {
		return fmt.Errorf("etcd register keepalive: %w", err)
	}
	r.keepAlive = ch
	return nil
}

// Deregister 注销服务实例（撤销租约）。
func (r *etcdRegistry) Deregister(inst registry.Instance) error {
	key := r.serviceKey(inst.Name, inst.Address)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd deregister delete: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.keepAlive != nil && r.leaseID != 0 {
		r.client.Revoke(context.Background(), r.leaseID)
		r.leaseID = 0
		r.keepAlive = nil
	}
	return nil
}

// Discover 获取指定服务的存活实例列表。
func (r *etcdRegistry) Discover(name string) ([]registry.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := r.client.Get(ctx, r.servicePrefix(name), clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd discover: %w", err)
	}

	instances := make([]registry.Instance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var inst registry.Instance
		if err := json.Unmarshal(kv.Value, &inst); err != nil {
			continue
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// Watch 监听服务实例变化（基于 etcd watch）。
func (r *etcdRegistry) Watch(name string, cb func([]registry.Instance)) (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	wch := r.client.Watch(ctx, r.servicePrefix(name), clientv3.WithPrefix())

	go func() {
		for wresp := range wch {
			if wresp.Err() != nil {
				continue
			}
			instances, _ := r.Discover(name)
			cb(instances)
		}
	}()

	return func() { cancel() }, nil
}
