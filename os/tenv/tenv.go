// Package tenv 提供类型安全的环境变量访问。
//
// 设计要点：
//   - 全部基于标准库 os 包，零外部依赖、零运行时分配成本；
//   - 提供泛型 Get[T]，免去手动类型转换与错误判断；
//   - 读取失败时回退到默认值，符合 12-Factor 配置习惯。
package tenv

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Get 读取环境变量 key，按目标类型解析。
// key 支持 `database.hostname`，会映射为大写下划线格式 DATABASE_HOSTNAME。
// 变量缺失或解析失败时返回 def。
func Get[T any](key string, def T) T {
	value, ok := Lookup(key)
	if !ok {
		return def
	}
	parsed, ok := parse(value, def)
	if !ok {
		return def
	}
	return parsed
}

// MustGet 读取环境变量 key，缺失或解析失败时 panic。
// 用于启动期必须有值的强约束配置项。
func MustGet[T any](key string) T {
	value, ok := Lookup(key)
	if !ok {
		panic("tenv: required environment variable not set: " + normalizeKey(key))
	}
	parsed, ok := parse(value, *new(T))
	if !ok {
		panic("tenv: invalid environment variable: " + normalizeKey(key))
	}
	return parsed
}

// Lookup 返回环境变量原始值。与 Get 不同，它能区分变量缺失和变量存在但为空。
func Lookup(key string) (string, bool) { return os.LookupEnv(normalizeKey(key)) }

// Value 读取动态值并处理特殊字面量。
func Value(key string, defaults ...any) any {
	value, ok := Lookup(key)
	if !ok {
		if len(defaults) > 0 {
			return defaults[0]
		}
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "(true)":
		return true
	case "false", "(false)":
		return false
	case "null", "(null)":
		return nil
	case "empty", "(empty)":
		return ""
	default:
		return value
	}
}

// Set 写入进程级环境变量。
func Set(key, value string) error { return os.Setenv(normalizeKey(key), value) }

// SetInt / SetBool 是带格式化的便捷写入。
func SetInt(key string, value int) error   { return Set(key, strconv.Itoa(value)) }
func SetBool(key string, value bool) error { return Set(key, strconv.FormatBool(value)) }

// Unset 删除进程级环境变量。
func Unset(key string) error { return os.Unsetenv(normalizeKey(key)) }

// Has 判断环境变量是否存在，包括值为空的变量。
func Has(key string) bool {
	_, ok := Lookup(key)
	return ok
}

// Getenv 返回原始字符串，缺失时返回 def。
func Getenv(key, def string) string {
	if value, ok := Lookup(key); ok {
		return value
	}
	return def
}

// Load 按顺序加载 dotenv 文件。已有系统环境变量不会被覆盖。
func Load(paths ...string) error {
	for _, path := range paths {
		if err := LoadFile(path); err != nil {
			return err
		}
	}
	return nil
}

// Expand 展开字符串中的 ${VAR} 与 $VAR 占位符。
func Expand(s string) string { return os.ExpandEnv(s) }

// All 返回环境变量副本；prefix 可使用点路径或下划线前缀过滤。
func All(prefix ...string) map[string]string {
	filter := ""
	if len(prefix) > 0 && strings.TrimSpace(prefix[0]) != "" {
		filter = normalizeKey(prefix[0])
	}
	result := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok && (filter == "" || strings.HasPrefix(key, filter)) {
			result[key] = value
		}
	}
	return result
}

// GetMap 解析 "a=1,b=2" 形式的逗号分隔键值串为 map。
func GetMap(key string) map[string]string {
	value, ok := Lookup(key)
	if !ok || value == "" {
		return map[string]string{}
	}
	out := make(map[string]string)
	for pair := range strings.SplitSeq(value, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return out
}

func normalizeKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.NewReplacer(".", "_", "-", "_").Replace(key)
	return strings.ToUpper(key)
}

// parse 将字符串按目标类型解析。
func parse[T any](value string, def T) (T, bool) {
	trimmed := strings.TrimSpace(value)
	special := strings.ToLower(trimmed)
	switch special {
	case "empty", "(empty)":
		trimmed = ""
	case "null", "(null)":
		return def, false
	case "true", "(true)":
		trimmed = "true"
	case "false", "(false)":
		trimmed = "false"
	}
	switch any(def).(type) {
	case string:
		return any(trimmed).(T), true
	case int:
		if n, err := strconv.Atoi(trimmed); err == nil {
			return any(n).(T), true
		}
	case int64:
		if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return any(n).(T), true
		}
	case int32:
		if n, err := strconv.ParseInt(trimmed, 10, 32); err == nil {
			return any(int32(n)).(T), true
		}
	case float64:
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return any(f).(T), true
		}
	case float32:
		if f, err := strconv.ParseFloat(trimmed, 32); err == nil {
			return any(float32(f)).(T), true
		}
	case bool:
		if b, err := strconv.ParseBool(trimmed); err == nil {
			return any(b).(T), true
		}
	}
	return def, false
}

// LoadFile 加载 dotenv 文件。系统中已存在的环境变量优先，不会被文件覆盖。
// 文件不存在视为未配置，返回 nil。
func LoadFile(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("tenv: open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	section := ""
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = normalizeKey(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "" {
				return fmt.Errorf("tenv: %s:%d: empty section name", path, lineNumber)
			}
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("tenv: %s:%d: expected KEY=VALUE", path, lineNumber)
		}
		key = normalizeKey(key)
		if key == "" {
			return fmt.Errorf("tenv: %s:%d: empty variable name", path, lineNumber)
		}
		if section != "" && !strings.HasPrefix(key, section+"_") {
			key = section + "_" + key
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if err := os.Setenv(key, os.ExpandEnv(value)); err != nil {
			return fmt.Errorf("tenv: set %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("tenv: read %s: %w", path, err)
	}
	return nil
}
