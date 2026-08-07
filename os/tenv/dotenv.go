package tenv

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

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
