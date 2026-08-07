// Package registry 提供服务注册与发现抽象。
//
// 出于「零外部依赖、聚焦 Web 框架本体」的约束，内置仅实现 file 后端
// （单节点/开发态足够），并定义统一的 Registry 接口，便于后续接入 etcd/nacos
// 等分布式注册中心（实现 Registry 接口即可，无需改动业务代码）。
package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Instance 是一个服务实例。
type Instance struct {
	Name    string `json:"name"`
	Address string `json:"address"` // host:port
	Weight  int    `json:"weight"`
	Meta    map[string]string `json:"meta,omitempty"`
	// TTL 是心跳有效期；file 后端用时间戳判断过期。
	TTL time.Duration `json:"-"`
	// ExpiresAt 是实例过期绝对时间（file 后端持久化用，TTL>0 时有效）。
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// Registry 是服务注册发现接口。
type Registry interface {
	// Register 注册一个实例（持续心跳由调用方负责周期调用）。
	Register(inst Instance) error
	// Deregister 注销实例。
	Deregister(inst Instance) error
	// Discover 返回某服务当前存活的实例列表。
	Discover(name string) ([]Instance, error)
	// Watch 阻塞监听服务实例变化（file 后端为轮询，返回关闭函数）。
	Watch(name string, cb func([]Instance)) (stop func(), err error)
}

/* ----------------------------- 文件后端 ----------------------------- */

// fileRegistry 基于本地 JSON 文件存储实例（单节点/开发态）。
type fileRegistry struct {
	mu   sync.RWMutex
	path string
}

// NewFile 创建文件型注册中心，数据持久化到 path（如 runtime/registry.json）。
func NewFile(path string) Registry {
	if path == "" {
		path = filepath.Join("runtime", "registry.json")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return &fileRegistry{path: path}
}

func (r *fileRegistry) load() map[string][]Instance {
	b, err := os.ReadFile(r.path)
	if err != nil {
		return map[string][]Instance{}
	}
	var store map[string][]Instance
	if json.Unmarshal(b, &store) != nil {
		return map[string][]Instance{}
	}
	return store
}

func (r *fileRegistry) save(store map[string][]Instance) error {
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, b, 0o644)
}

func (r *fileRegistry) Register(inst Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	store := r.load()
	list := store[inst.Name]
	now := time.Now()
	if inst.TTL > 0 {
		inst.ExpiresAt = now.Add(inst.TTL)
	} else {
		inst.ExpiresAt = time.Time{}
	}
	// 去重：同地址则刷新。
	for i, it := range list {
		if it.Address == inst.Address {
			list[i] = inst
			store[inst.Name] = list
			return r.save(store)
		}
	}
	store[inst.Name] = append(list, inst)
	return r.save(store)
}

func (r *fileRegistry) Deregister(inst Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	store := r.load()
	list := store[inst.Name]
	out := list[:0]
	for _, it := range list {
		if it.Address != inst.Address {
			out = append(out, it)
		}
	}
	store[inst.Name] = out
	return r.save(store)
}

func (r *fileRegistry) Discover(name string) ([]Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	store := r.load()
	return alive(store[name]), nil
}

func (r *fileRegistry) Watch(name string, cb func([]Instance)) (func(), error) {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				inst, _ := r.Discover(name)
				cb(inst)
			}
		}
	}()
	return func() { close(stop) }, nil
}

// alive 过滤掉已过期（ExpiresAt 早于现在）的实例。
func alive(list []Instance) []Instance {
	if len(list) == 0 {
		return nil
	}
	now := time.Now()
	out := make([]Instance, 0, len(list))
	for _, it := range list {
		if !it.ExpiresAt.IsZero() && now.After(it.ExpiresAt) {
			continue
		}
		out = append(out, it)
	}
	return out
}
