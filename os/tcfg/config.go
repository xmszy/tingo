package tcfg

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Config 是 Adapter 之上的只读配置门面。
type Config struct{ adapter Adapter }

func New(adapter Adapter) *Config {
	if adapter == nil {
		panic("tcfg: adapter must not be nil")
	}
	return &Config{adapter: adapter}
}

func NewFromTree(tree Tree) *Config { return New(NewContentAdapter(tree)) }

func NewFromBytes(format string, content []byte) (*Config, error) {
	adapter, err := NewContentAdapterBytes(format, content)
	if err != nil {
		return nil, err
	}
	return New(adapter), nil
}

func NewFromFiles(files ...string) (*Config, error) {
	adapter, err := NewFileAdapter(files...)
	if err != nil {
		return nil, err
	}
	return New(adapter), nil
}

func NewFromDirectory(directory, extension string) (*Config, error) {
	adapter, err := NewDirectoryAdapter(directory, extension)
	if err != nil {
		return nil, err
	}
	return New(adapter), nil
}

func (c *Config) Adapter() Adapter { return c.adapter }

func (c *Config) Available(ctx context.Context, resource ...string) bool {
	return c.adapter.Available(ctx, resource...)
}

func (c *Config) DataContext(ctx context.Context) (Tree, error) {
	return c.adapter.Data(ctx)
}

func (c *Config) Data() Tree {
	data, err := c.DataContext(context.Background())
	if err != nil {
		return Tree{}
	}
	return data
}

func (c *Config) LookupContext(ctx context.Context, path string) (any, bool, error) {
	value, err := c.adapter.Get(ctx, path)
	if err != nil {
		return nil, false, err
	}
	return value, value != nil, nil
}

func (c *Config) Lookup(path string) (any, bool) { return c.Data().Lookup(path) }

func (c *Config) Get(path string, defaults ...any) any {
	if value, ok := c.Lookup(path); ok {
		return cloneValue(value)
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return nil
}
func (c *Config) Has(path string) bool { _, ok := c.Lookup(path); return ok }
func (c *Config) String(path string, defaults ...string) string {
	return configValue(c, path, firstOrZero(defaults))
}
func (c *Config) Bool(path string, defaults ...bool) bool {
	return configValue(c, path, firstOrZero(defaults))
}
func (c *Config) Int(path string, defaults ...int) int {
	return configValue(c, path, firstOrZero(defaults))
}
func (c *Config) Int64(path string, defaults ...int64) int64 {
	return configValue(c, path, firstOrZero(defaults))
}
func (c *Config) Float64(path string, defaults ...float64) float64 {
	return configValue(c, path, firstOrZero(defaults))
}
func (c *Config) Strings(path string, defaults ...[]string) []string {
	return configValue(c, path, firstOrZero(defaults))
}
func (c *Config) DecodeAt(path string, target any) error {
	value, ok := c.Lookup(path)
	if !ok {
		return nil
	}
	if err := Decode(value, target); err != nil {
		return fmt.Errorf("tcfg: decode %q: %w", path, err)
	}
	return nil
}

func configValue[T any](config *Config, path string, fallback T) T {
	value, ok, err := config.LookupContext(context.Background(), path)
	if err != nil || !ok {
		return fallback
	}
	var holder struct {
		Value T `json:"value"`
	}
	if err := Decode(map[string]any{"value": value}, &holder); err != nil {
		return fallback
	}
	return holder.Value
}

// GetEffective 按“环境变量 > 配置 > 默认值”读取有效值。
func (c *Config) GetEffective(ctx context.Context, path string, defaults ...any) (any, error) {
	if value, exists := os.LookupEnv(environmentKey(path)); exists {
		return value, nil
	}
	value, err := c.adapter.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	if value != nil {
		return cloneValue(value), nil
	}
	if len(defaults) > 0 {
		return defaults[0], nil
	}
	return nil, nil
}

func (c *Config) EffectiveString(ctx context.Context, path string, fallback string) (string, error) {
	value, err := c.GetEffective(ctx, path, fallback)
	if err != nil {
		return "", err
	}
	var holder struct {
		Value string `json:"value"`
	}
	if err := Decode(map[string]any{"value": value}, &holder); err != nil {
		return "", err
	}
	return holder.Value, nil
}

func environmentKey(path string) string {
	path = strings.TrimSpace(path)
	return strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(path))
}
