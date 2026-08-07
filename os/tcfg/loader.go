package tcfg

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Loader 将只读配置绑定为强类型快照，并可显式监听来源变化。
type Loader[T any] struct {
	mu           sync.RWMutex
	config       *Config
	path         string
	current      T
	converter    func(any, *T) error
	onChange     func(T) error
	onWatchError func(context.Context, error)
	watcherName  string
}

func NewLoader[T any](config *Config, path string, target ...*T) (*Loader[T], error) {
	if config == nil {
		return nil, fmt.Errorf("tcfg: loader config must not be nil")
	}
	loader := &Loader[T]{config: config, path: path}
	if len(target) > 0 && target[0] != nil {
		loader.current = *target[0]
	}
	if err := loader.Load(context.Background()); err != nil {
		return nil, err
	}
	return loader, nil
}

func NewLoaderWithAdapter[T any](adapter Adapter, path string, target ...*T) (*Loader[T], error) {
	return NewLoader(New(adapter), path, target...)
}

func (l *Loader[T]) SetConverter(converter func(any, *T) error) *Loader[T] {
	l.mu.Lock()
	l.converter = converter
	l.mu.Unlock()
	return l
}

func (l *Loader[T]) OnChange(callback func(T) error) *Loader[T] {
	l.mu.Lock()
	l.onChange = callback
	l.mu.Unlock()
	return l
}

func (l *Loader[T]) OnWatchError(callback func(context.Context, error)) *Loader[T] {
	l.mu.Lock()
	l.onWatchError = callback
	l.mu.Unlock()
	return l
}

func (l *Loader[T]) Load(ctx context.Context) error {
	data, err := l.config.DataContext(ctx)
	if err != nil {
		return err
	}
	var source any = data
	if l.path != "" && l.path != "." {
		var ok bool
		source, ok = data.Lookup(l.path)
		if !ok {
			return fmt.Errorf("tcfg: path %q not found", l.path)
		}
	}
	l.mu.RLock()
	converter := l.converter
	callback := l.onChange
	l.mu.RUnlock()
	var updated T
	if converter != nil {
		if err := converter(source, &updated); err != nil {
			return fmt.Errorf("tcfg: convert %q: %w", l.path, err)
		}
	} else if err := Decode(source, &updated); err != nil {
		return fmt.Errorf("tcfg: decode %q: %w", l.path, err)
	}
	l.mu.Lock()
	l.current = updated
	l.mu.Unlock()
	if callback != nil {
		return callback(updated)
	}
	return nil
}

func (l *Loader[T]) MustLoad(ctx context.Context) {
	if err := l.Load(ctx); err != nil {
		panic(err)
	}
}

func (l *Loader[T]) Get() T {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.current
}

func (l *Loader[T]) Watch(ctx context.Context, name string, interval ...time.Duration) error {
	watcher, ok := l.config.Adapter().(WatcherAdapter)
	if !ok {
		return fmt.Errorf("tcfg: adapter does not support watching")
	}
	if name == "" {
		return fmt.Errorf("tcfg: watcher name must not be empty")
	}
	if err := watcher.AddWatcher(name, func(eventContext context.Context, reloadError error) {
		if reloadError == nil {
			reloadError = l.Load(eventContext)
		}
		if reloadError != nil {
			l.mu.RLock()
			handler := l.onWatchError
			l.mu.RUnlock()
			if handler != nil {
				handler(eventContext, reloadError)
			}
		}
	}); err != nil {
		return err
	}
	l.mu.Lock()
	l.watcherName = name
	l.mu.Unlock()
	pollInterval := time.Second
	if len(interval) > 0 {
		pollInterval = interval[0]
	}
	return watcher.StartWatch(ctx, pollInterval)
}

func (l *Loader[T]) StopWatch() bool {
	l.mu.Lock()
	name := l.watcherName
	l.watcherName = ""
	l.mu.Unlock()
	if name == "" {
		return false
	}
	watcher, ok := l.config.Adapter().(WatcherAdapter)
	if !ok {
		return false
	}
	watcher.RemoveWatcher(name)
	return true
}

func (l *Loader[T]) IsWatching() bool {
	l.mu.RLock()
	name := l.watcherName
	l.mu.RUnlock()
	if name == "" {
		return false
	}
	watcher, ok := l.config.Adapter().(WatcherAdapter)
	return ok && watcher.IsWatching(name)
}
