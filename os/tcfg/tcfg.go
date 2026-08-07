// Package tcfg 提供只读、可适配的配置读取与强类型绑定。
package tcfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
)

// Decode 使用统一弱类型规则把配置数据绑定到目标值。
func Decode(data any, target any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.TextUnmarshallerHookFunc(),
		ErrorUnused:      false,
		MatchName:        func(mapKey, fieldName string) bool { return strings.EqualFold(mapKey, fieldName) },
		Metadata:         nil,
		Result:           target,
		Squash:           true,
		TagName:          "json",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(data)
}

// ReadFile 读取一个配置文件。文件不存在时返回 loaded=false。
func ReadFile(path string) (tree Tree, loaded bool, err error) {
	data, err := parseFile(path)
	if os.IsNotExist(err) {
		return Tree{}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("tcfg: parse %s: %w", path, err)
	}
	return Tree(data), true, nil
}

// LoadFileInto 将单个配置文件绑定到目标值。
func LoadFileInto(path string, target any) (bool, error) {
	tree, loaded, err := ReadFile(path)
	if err != nil || !loaded {
		return loaded, err
	}
	if err := Decode(map[string]any(tree), target); err != nil {
		return true, fmt.Errorf("tcfg: decode %s: %w", path, err)
	}
	return true, nil
}

func parseFile(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded, err := expandEnvironment(raw, path)
	if err != nil {
		return nil, err
	}
	return parseBytes(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."), expanded)
}

func parseBytes(format string, raw []byte) (map[string]any, error) {
	format = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(format)), ".")
	data := map[string]any{}
	switch format {
	case "toml":
		if err := toml.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	case "json":
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	case "ini":
		return parseINI(raw)
	default:
		return nil, fmt.Errorf("tcfg: unsupported config format %q", format)
	}
	return data, nil
}

func parseINI(raw []byte) (map[string]any, error) {
	file, err := ini.LoadSources(ini.LoadOptions{Insensitive: false}, raw)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	for _, section := range file.Sections() {
		target := result
		if section.Name() != ini.DefaultSection {
			target = ensureMapPath(result, strings.Split(section.Name(), "."))
		}
		for _, key := range section.Keys() {
			setMapPath(target, strings.Split(key.Name(), "."), parseINIValue(key.Value()))
		}
	}
	return result, nil
}

func ensureMapPath(root map[string]any, parts []string) map[string]any {
	current := root
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	return current
}

func setMapPath(root map[string]any, parts []string, value any) {
	if len(parts) == 0 {
		return
	}
	target := ensureMapPath(root, parts[:len(parts)-1])
	target[strings.TrimSpace(parts[len(parts)-1])] = value
}

func parseINIValue(value string) any {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "true", "(true)":
		return true
	case "false", "(false)":
		return false
	case "null", "(null)":
		return nil
	case "empty", "(empty)":
		return ""
	}
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
		var decoded any
		if json.Unmarshal([]byte(value), &decoded) == nil {
			return decoded
		}
	}
	if integer, err := strconv.ParseInt(value, 10, 64); err == nil {
		return integer
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number
	}
	return value
}

func supportedConfigExtension(extension string) bool {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".") {
	case "toml", "yaml", "yml", "json", "ini":
		return true
	default:
		return false
	}
}

// configFileNames 是自动发现时尝试的文件名（不含扩展名），按优先级排列。
var configFileNames = []string{
	"config",    // config.toml / config.yaml / config.json
	"app",       // app.toml 等
	"setting",   // setting.toml
	".config",   // dotfile 风格
}

// Discover 在指定目录中查找配置文件（自动按 supportedConfigExtension 匹配第一个存在的文件）。
// dir 为空时默认为当前目录。
// 返回 Tree、文件已加载标志、错误。
//
// 自动发现，按优先级尝试多种文件。
func Discover(dir string) (Tree, bool, error) {
	if dir == "" {
		dir = "."
	}
	for _, name := range configFileNames {
		for _, ext := range supportedExtensions {
			path := filepath.Join(dir, name+"."+ext)
			if tree, loaded, err := ReadFile(path); err != nil {
				return nil, false, err
			} else if loaded {
				return tree, true, nil
			}
		}
	}
	return nil, false, nil
}

// supportedExtensions 是支持的配置文件扩展名，按优先级排列。
var supportedExtensions = []string{"toml", "yaml", "yml", "json", "ini"}

// LoadInto 自动发现配置并绑定到目标值。等效于 Discover + Decode。
// target 需为指针。
func LoadInto(dir string, target any) (bool, error) {
	tree, loaded, err := Discover(dir)
	if err != nil || !loaded {
		return loaded, err
	}
	if err := Decode(map[string]any(tree), target); err != nil {
		return true, fmt.Errorf("tcfg: decode auto-discovered config: %w", err)
	}
	return true, nil
}
