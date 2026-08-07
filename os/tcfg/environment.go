package tcfg

import (
	"fmt"
	"os"
	"strings"
)

// expandEnvironment 展开配置内容中的环境变量。
// ${VAR:-default} 在变量缺失或为空时使用默认值；无默认值的缺失变量会返回错误。
func expandEnvironment(content []byte, source string) ([]byte, error) {
	input := string(content)
	var output strings.Builder
	output.Grow(len(input))

	for index := 0; index < len(input); {
		if input[index] != '$' {
			output.WriteByte(input[index])
			index++
			continue
		}
		if index+1 >= len(input) {
			output.WriteByte('$')
			index++
			continue
		}
		if input[index+1] == '$' {
			output.WriteByte('$')
			index += 2
			continue
		}

		name, fallback, hasFallback, consumed, recognized, err := parseEnvironmentExpression(input[index:])
		if err != nil {
			return nil, fmt.Errorf("tcfg: expand %s: %w", source, err)
		}
		if !recognized {
			output.WriteByte('$')
			index++
			continue
		}
		value, exists := os.LookupEnv(name)
		if hasFallback && (!exists || value == "") {
			value = fallback
			exists = true
		}
		if !exists {
			return nil, fmt.Errorf("tcfg: expand %s: environment variable %s is not set", source, name)
		}
		output.WriteString(value)
		index += consumed
	}
	return []byte(output.String()), nil
}

func parseEnvironmentExpression(input string) (name, fallback string, hasFallback bool, consumed int, recognized bool, err error) {
	if len(input) < 2 || input[0] != '$' {
		return "", "", false, 0, false, nil
	}
	if input[1] == '{' {
		end := strings.IndexByte(input[2:], '}')
		if end < 0 {
			return "", "", false, 0, true, fmt.Errorf("unterminated environment expression")
		}
		end += 2
		expression := input[2:end]
		if variable, defaultValue, ok := strings.Cut(expression, ":-"); ok {
			name = variable
			fallback = defaultValue
			hasFallback = true
		} else {
			name = expression
		}
		if !validEnvironmentName(name) {
			return "", "", false, 0, true, fmt.Errorf("invalid environment expression %q", input[:end+1])
		}
		return name, fallback, hasFallback, end + 1, true, nil
	}
	end := 1
	for end < len(input) && isEnvironmentNameByte(input[end], end == 1) {
		end++
	}
	if end == 1 {
		return "", "", false, 0, false, nil
	}
	return input[1:end], "", false, end, true, nil
}

func validEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for index := range len(name) {
		if !isEnvironmentNameByte(name[index], index == 0) {
			return false
		}
	}
	return true
}

func isEnvironmentNameByte(value byte, first bool) bool {
	if value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' {
		return true
	}
	return !first && value >= '0' && value <= '9'
}
