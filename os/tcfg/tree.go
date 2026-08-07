package tcfg

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Reader 是配置消费者依赖的只读读取契约。
type Reader interface {
	Lookup(path string) (any, bool)
	Get(path string, defaults ...any) any
	Has(path string) bool
	String(path string, defaults ...string) string
	Bool(path string, defaults ...bool) bool
	Int(path string, defaults ...int) int
	Int64(path string, defaults ...int64) int64
	Float64(path string, defaults ...float64) float64
	Strings(path string, defaults ...[]string) []string
	DecodeAt(path string, target any) error
	Data() Tree
}

// Tree 是一棵只读配置快照。返回给调用方前应通过 Clone 复制。
type Tree map[string]any

// Data 返回不共享内部 map 或 slice 的快照。
func (t Tree) Data() Tree { return t.Clone() }

// Clone 深复制配置树。
func (t Tree) Clone() Tree {
	return Tree(cloneMap(map[string]any(t)))
}

// MergeTrees 深合并配置树，后面的树优先。
func MergeTrees(trees ...Tree) Tree {
	result := Tree{}
	for _, tree := range trees {
		mergeMap(map[string]any(result), cloneMap(map[string]any(tree)))
	}
	return result
}

// Lookup 按点路径读取 map 字段或 slice 索引。
func (t Tree) Lookup(path string) (any, bool) {
	if path == "" || path == "." {
		return t.Clone(), true
	}
	return lookup(any(map[string]any(t)), strings.Split(path, "."))
}

// Get 返回原始值；路径不存在时返回可选默认值。
func (t Tree) Get(path string, defaults ...any) any {
	if value, ok := t.Lookup(path); ok {
		return cloneValue(value)
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return nil
}

func (t Tree) Has(path string) bool {
	_, ok := t.Lookup(path)
	return ok
}

func (t Tree) String(path string, defaults ...string) string {
	return treeValue(t, path, firstOrZero(defaults))
}

func (t Tree) Bool(path string, defaults ...bool) bool {
	return treeValue(t, path, firstOrZero(defaults))
}

func (t Tree) Int(path string, defaults ...int) int {
	return treeValue(t, path, firstOrZero(defaults))
}

func (t Tree) Int64(path string, defaults ...int64) int64 {
	return treeValue(t, path, firstOrZero(defaults))
}

func (t Tree) Float64(path string, defaults ...float64) float64 {
	return treeValue(t, path, firstOrZero(defaults))
}

func (t Tree) Strings(path string, defaults ...[]string) []string {
	return treeValue(t, path, firstOrZero(defaults))
}

// DecodeAt 将指定路径绑定到目标值。路径不存在时不修改目标。
func (t Tree) DecodeAt(path string, target any) error {
	value, ok := t.Lookup(path)
	if !ok {
		return nil
	}
	if err := Decode(value, target); err != nil {
		return fmt.Errorf("tcfg: decode %q: %w", path, err)
	}
	return nil
}

// Value 读取并转换指定路径，转换失败时返回带路径的错误。
func Value[T any](reader Reader, path string) (T, error) {
	var result T
	value, ok := reader.Lookup(path)
	if !ok {
		return result, fmt.Errorf("tcfg: path %q not found", path)
	}
	var holder struct {
		Value T `json:"value"`
	}
	if err := Decode(map[string]any{"value": value}, &holder); err != nil {
		return result, fmt.Errorf("tcfg: convert %q: %w", path, err)
	}
	return holder.Value, nil
}

func lookup(current any, parts []string) (any, bool) {
	for _, part := range parts {
		switch value := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = value[part]
			if !ok {
				return nil, false
			}
		case Tree:
			var ok bool
			current, ok = value[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
		default:
			rv := reflect.ValueOf(current)
			if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
				return nil, false
			}
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= rv.Len() {
				return nil, false
			}
			current = rv.Index(index).Interface()
		}
	}
	return current, true
}

func firstOrZero[T any](values []T) T {
	if len(values) > 0 {
		return values[0]
	}
	var zero T
	return zero
}

func treeValue[T any](tree Tree, path string, fallback T) T {
	value, ok := tree.Lookup(path)
	if !ok {
		return fallback
	}
	if typed, ok := value.(T); ok {
		return typed
	}
	var holder struct {
		Value T `json:"value"`
	}
	if err := Decode(map[string]any{"value": value}, &holder); err != nil {
		return fallback
	}
	return holder.Value
}

func mergeMap(target, source map[string]any) {
	for key, value := range source {
		if sourceChild, ok := value.(map[string]any); ok {
			if targetChild, ok := target[key].(map[string]any); ok {
				mergeMap(targetChild, sourceChild)
				continue
			}
		}
		target[key] = cloneValue(value)
	}
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneValue(value)
	}
	return result
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case Tree:
		return typed.Clone()
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = cloneValue(item)
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}
