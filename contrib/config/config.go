// Package config 提供配置中心抽象，支持从远程配置中心拉取配置并保持热更新。
//
// 内置 file 后端（读取本地配置文件，mtime 轮询热更新），
// 通过 ConfigCenter 接口可接入 nacos/apollo/consul 等分布式配置中心。
package config

import (
	"os"
	"sync"
	"time"
)

// EventType 配置变更事件类型。
type EventType int

const (
	EventPut    EventType = iota // 新增或更新
	EventDelete                  // 删除
)

// Event 配置变更事件。
type Event struct {
	Type EventType
	Key  string
	Value string
}

// Watcher 配置监听器。
type Watcher func(event Event)

// ConfigCenter 配置中心接口。
type ConfigCenter interface {
	// Get 获取单个配置项的值。
	Get(key string) (string, error)

	// Set 写入配置项（如果后端支持）。
	Set(key, value string) error

	// Delete 删除配置项（如果后端支持）。
	Delete(key string) error

	// GetAll 获取所有配置项。
	GetAll() (map[string]string, error)

	// Watch 监听配置变更（返回取消函数）。
	Watch(watcher Watcher) (func(), error)

	// Close 关闭配置中心连接。
	Close() error
}

/* ----------------------------- 文件后端 ----------------------------- */

// fileConfig 基于本地文件的配置中心实现。
type fileConfig struct {
	mu       sync.RWMutex
	path     string
	data     map[string]string
	modTime  time.Time
	stopPoll chan struct{}
	watchers []watchEntry
	wmu      sync.RWMutex
	closeOnce sync.Once
}

// NewFile 创建文件配置中心，读取 path 指定的 JSON 文件。
//
// 文件格式：{"key1": "value1", "key2": "value2"}
// 当 path 为空时使用 "config/config.json"。
func NewFile(path string) ConfigCenter {
	if path == "" {
		path = "config/config.json"
	}
	fc := &fileConfig{
		path:     path,
		data:     make(map[string]string),
		stopPoll: make(chan struct{}),
	}
	fc.load()
	// 启动后台热加载（mtime 轮询）
	go fc.poll()
	return fc
}

func (fc *fileConfig) load() {
	b, err := os.ReadFile(fc.path)
	if err != nil {
		return
	}
	// 简单 JSON map 解析（零外部依赖）
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data = parseSimpleJSONMap(string(b))

	if info, err := os.Stat(fc.path); err == nil {
		fc.modTime = info.ModTime()
	}
}

func (fc *fileConfig) poll() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-fc.stopPoll:
			return
		case <-ticker.C:
			info, err := os.Stat(fc.path)
			if err != nil {
				continue
			}
			fc.mu.RLock()
			changed := !info.ModTime().Equal(fc.modTime)
			fc.mu.RUnlock()
			if !changed {
				continue
			}

			// 加载新数据并与旧数据对比
			oldData := fc.GetAllRaw()
			fc.load()
			newData := fc.GetAllRaw()

			// 触发 watchers
			fc.notifyChanges(oldData, newData)
		}
	}
}

// GetAllRaw 不加锁读取（内部使用）。
func (fc *fileConfig) GetAllRaw() map[string]string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	copy := make(map[string]string, len(fc.data))
	for k, v := range fc.data {
		copy[k] = v
	}
	return copy
}

func (fc *fileConfig) notifyChanges(old, new map[string]string) {
	fc.wmu.RLock()
	defer fc.wmu.RUnlock()

	// 检测变更
	for k, v := range new {
		if old[k] != v {
			event := Event{Type: EventPut, Key: k, Value: v}
			for _, e := range fc.watchers {
				e.watcher(event)
			}
		}
	}
	for k := range old {
		if _, ok := new[k]; !ok {
			event := Event{Type: EventDelete, Key: k}
			for _, e := range fc.watchers {
				e.watcher(event)
			}
		}
	}
}

func (fc *fileConfig) Get(key string) (string, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	v, ok := fc.data[key]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}

func (fc *fileConfig) Set(key, value string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data[key] = value
	return fc.persist()
}

func (fc *fileConfig) Delete(key string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	delete(fc.data, key)
	return fc.persist()
}

func (fc *fileConfig) GetAll() (map[string]string, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	copy := make(map[string]string, len(fc.data))
	for k, v := range fc.data {
		copy[k] = v
	}
	return copy, nil
}

// watchEntry 是一条监听记录，携带可比较的取消令牌。
type watchEntry struct {
	id      int
	watcher Watcher
}

var watchSeq int

func (fc *fileConfig) Watch(watcher Watcher) (func(), error) {
	fc.wmu.Lock()
	watchSeq++
	id := watchSeq
	fc.watchers = append(fc.watchers, watchEntry{id: id, watcher: watcher})
	fc.wmu.Unlock()
	// 取消函数携带 id 令牌，避免比较函数值（Go 不允许函数互比）。
	return func() {
		fc.wmu.Lock()
		defer fc.wmu.Unlock()
		for i, e := range fc.watchers {
			if e.id == id {
				fc.watchers = append(fc.watchers[:i], fc.watchers[i+1:]...)
				break
			}
		}
	}, nil
}

func (fc *fileConfig) Close() error {
	fc.closeOnce.Do(func() { close(fc.stopPoll) })
	return nil
}

func (fc *fileConfig) persist() error {
	b := marshalSimpleJSONMap(fc.data)
	return os.WriteFile(fc.path, []byte(b), 0o644)
}

// ──────────────── 简易 JSON 解析（零外部依赖）───────────────

// parseSimpleJSONMap 从 {"k":"v",...} 格式解析 map（仅支持 string→string）。
func parseSimpleJSONMap(s string) map[string]string {
	m := make(map[string]string)
	// 去除首尾空白与括号
	s = trimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return m
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return m
	}

	i := 0
	for i < len(s) {
		// 跳过空白与逗号
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		// 读取 key
		if s[i] != '"' {
			break
		}
		i++
		kStart := i
		for i < len(s) && s[i] != '"' {
			if s[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(s) {
			break
		}
		key := s[kStart:i]
		i++ // skip closing "
		// skip :
		for i < len(s) && s[i] != ':' {
			i++
		}
		i++ // skip :
		// skip whitespace
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		// read value
		if i >= len(s) || s[i] != '"' {
			break
		}
		i++
		vStart := i
		for i < len(s) && s[i] != '"' {
			if s[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(s) {
			break
		}
		value := s[vStart:i]
		i++
		m[key] = value
	}
	return m
}

func marshalSimpleJSONMap(m map[string]string) string {
	s := "{"
	first := true
	for k, v := range m {
		if !first {
			s += ","
		}
		first = false
		s += `"` + escapeJSON(k) + `":"` + escapeJSON(v) + `"`
	}
	s += "}"
	return s
}

func escapeJSON(s string) string {
	b := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		switch c {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			b = append(b, c)
		}
	}
	return string(b)
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s) - 1
	for end >= start && (s[end] == ' ' || s[end] == '\t' || s[end] == '\n' || s[end] == '\r') {
		end--
	}
	return s[start : end+1]
}
