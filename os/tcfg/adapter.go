package tcfg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Adapter 是只读配置来源契约。
type Adapter interface {
	Available(ctx context.Context, resource ...string) bool
	Get(ctx context.Context, path string) (any, error)
	Data(ctx context.Context) (Tree, error)
}

// WatcherFunc 在配置快照变化或重载失败后执行。
type WatcherFunc func(context.Context, error)

// WatcherAdapter 是可监听配置来源的附加契约。
type WatcherAdapter interface {
	Adapter
	AddWatcher(name string, watcher WatcherFunc) error
	RemoveWatcher(name string)
	WatcherNames() []string
	IsWatching(name string) bool
	StartWatch(ctx context.Context, interval time.Duration) error
	StopWatch()
}

// ContentAdapter 从内存快照提供配置。
type ContentAdapter struct{ tree Tree }

func NewContentAdapter(tree Tree) *ContentAdapter {
	return &ContentAdapter{tree: tree.Clone()}
}

func NewContentAdapterBytes(format string, content []byte) (*ContentAdapter, error) {
	expanded, err := expandEnvironment(content, "content")
	if err != nil {
		return nil, err
	}
	data, err := parseBytes(format, expanded)
	if err != nil {
		return nil, err
	}
	return NewContentAdapter(Tree(data)), nil
}

func (a *ContentAdapter) Available(context.Context, ...string) bool { return a != nil }
func (a *ContentAdapter) Get(_ context.Context, path string) (any, error) {
	value, _ := a.tree.Lookup(path)
	return cloneValue(value), nil
}
func (a *ContentAdapter) Data(context.Context) (Tree, error) { return a.tree.Clone(), nil }

// FileAdapter 从显式文件列表或约定目录提供缓存配置。
type FileAdapter struct {
	mu        sync.RWMutex
	files     []string
	directory string
	extension string
	cache     Tree
	signature string
	watchers  map[string]WatcherFunc
	cancel    context.CancelFunc
}

// NewFileAdapter 创建多文件适配器，文件按参数顺序深合并，后者优先。
func NewFileAdapter(files ...string) (*FileAdapter, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("tcfg: at least one config file is required")
	}
	adapter := &FileAdapter{files: append([]string(nil), files...), watchers: map[string]WatcherFunc{}}
	if _, err := adapter.reload(context.Background(), true); err != nil {
		return nil, err
	}
	return adapter, nil
}

// NewDirectoryAdapter 创建按文件名建立命名空间的目录适配器。
func NewDirectoryAdapter(directory, extension string) (*FileAdapter, error) {
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	if !supportedConfigExtension(extension) {
		return nil, fmt.Errorf("tcfg: unsupported config extension %q", extension)
	}
	adapter := &FileAdapter{directory: directory, extension: extension, watchers: map[string]WatcherFunc{}}
	if _, err := adapter.reload(context.Background(), true); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (a *FileAdapter) Available(ctx context.Context, resource ...string) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	if len(resource) == 0 || resource[0] == "" {
		a.mu.RLock()
		available := len(a.cache) > 0
		a.mu.RUnlock()
		return available
	}
	_, err := os.Stat(resource[0])
	return err == nil
}

func (a *FileAdapter) Get(ctx context.Context, path string) (any, error) {
	data, err := a.Data(ctx)
	if err != nil {
		return nil, err
	}
	value, _ := data.Lookup(path)
	return cloneValue(value), nil
}

func (a *FileAdapter) Data(ctx context.Context) (Tree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.RLock()
	data := a.cache.Clone()
	a.mu.RUnlock()
	return data, nil
}

func (a *FileAdapter) AddWatcher(name string, watcher WatcherFunc) error {
	if strings.TrimSpace(name) == "" || watcher == nil {
		return fmt.Errorf("tcfg: watcher name and callback are required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.watchers[name]; exists {
		return fmt.Errorf("tcfg: watcher %q already exists", name)
	}
	a.watchers[name] = watcher
	return nil
}

func (a *FileAdapter) RemoveWatcher(name string) {
	a.mu.Lock()
	delete(a.watchers, name)
	a.mu.Unlock()
}

func (a *FileAdapter) WatcherNames() []string {
	a.mu.RLock()
	names := make([]string, 0, len(a.watchers))
	for name := range a.watchers {
		names = append(names, name)
	}
	a.mu.RUnlock()
	sort.Strings(names)
	return names
}

func (a *FileAdapter) IsWatching(name string) bool {
	a.mu.RLock()
	_, ok := a.watchers[name]
	a.mu.RUnlock()
	return ok
}

func (a *FileAdapter) StartWatch(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return nil
	}
	watchContext, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.mu.Unlock()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-watchContext.Done():
				return
			case <-ticker.C:
				changed, err := a.reload(watchContext, false)
				if err != nil || changed {
					a.notify(watchContext, err)
				}
			}
		}
	}()
	return nil
}

func (a *FileAdapter) StopWatch() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.mu.Unlock()
}

func (a *FileAdapter) notify(ctx context.Context, reloadError error) {
	a.mu.RLock()
	watchers := make([]WatcherFunc, 0, len(a.watchers))
	for _, watcher := range a.watchers {
		watchers = append(watchers, watcher)
	}
	a.mu.RUnlock()
	for _, watcher := range watchers {
		watcher(ctx, reloadError)
	}
}

func (a *FileAdapter) reload(ctx context.Context, force bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	files, err := a.resolveFiles()
	if err != nil {
		return false, err
	}
	signature, err := fileSignature(files)
	if err != nil {
		return false, err
	}
	a.mu.RLock()
	unchanged := !force && signature == a.signature
	a.mu.RUnlock()
	if unchanged {
		return false, nil
	}
	data, err := loadFiles(files, a.directory != "")
	if err != nil {
		return false, err
	}
	a.mu.Lock()
	a.cache = data
	a.signature = signature
	a.mu.Unlock()
	return true, nil
}

func (a *FileAdapter) resolveFiles() ([]string, error) {
	if a.directory == "" {
		return append([]string(nil), a.files...), nil
	}
	entries, err := os.ReadDir(a.directory)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tcfg: read config directory %s: %w", a.directory, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), "."+a.extension) {
			files = append(files, filepath.Join(a.directory, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func loadFiles(files []string, namespaced bool) (Tree, error) {
	result := Tree{}
	for _, path := range files {
		data, err := parseFile(path)
		if err != nil {
			return nil, fmt.Errorf("tcfg: parse %s: %w", path, err)
		}
		if namespaced {
			namespace := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			current, _ := result[namespace].(map[string]any)
			if current == nil {
				current = map[string]any{}
			}
			mergeMap(current, data)
			result[namespace] = current
			continue
		}
		mergeMap(map[string]any(result), data)
	}
	return result, nil
}

func fileSignature(files []string) (string, error) {
	var builder strings.Builder
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&builder, "%s:%d:%d;", path, info.Size(), info.ModTime().UnixNano())
	}
	return builder.String(), nil
}

// ReadDir 读取目录中的全部受支持格式。仅用于显式工具场景。
func ReadDir(directory string) (Tree, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return Tree{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := Tree{}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !supportedConfigExtension(filepath.Ext(entry.Name())) {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := parseFile(path)
		if err != nil {
			return nil, fmt.Errorf("tcfg: parse %s: %w", path, err)
		}
		namespace := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		current, _ := result[namespace].(map[string]any)
		if current == nil {
			current = map[string]any{}
		}
		mergeMap(current, data)
		result[namespace] = current
	}
	return result, nil
}

func ReadDirWithExtension(directory, extension string) (Tree, error) {
	adapter, err := NewDirectoryAdapter(directory, extension)
	if err != nil {
		return nil, err
	}
	return adapter.Data(context.Background())
}
