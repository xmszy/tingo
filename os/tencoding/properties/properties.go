// Package properties 提供 Java Properties 格式解析（零外部依赖）。
//
// 支持：
//   - key=value / key:value / key value（空格分隔）三种形式。
//   - # 与 ! 开头的注释行。
//   - 行续接：行尾以 \ 结尾表示下一行继续。
//   - 转义：\n \t \r \\ \: \= \ 空格 以及 Unicode \uXXXX。
//   - 变量插值：${key} 引用已解析的键。
//
// 解析结果统一为 map[string]string；另提供 Unmarshal 映射到结构体（字段 tag `prop:"key"`）。
package properties

import (
	"fmt"
	"reflect"
	"strings"
)

// Parse 解析 Properties 文本为键值对。
func Parse(data string) (map[string]string, error) {
	out := map[string]string{}
	// 先按逻辑行（处理续接）切分。
	logical := joinContinuations(data)
	for _, raw := range logical {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		// 找到首个未被转义、未加引号的键结束符（= : 或空白）。
		key, val, ok := splitKeyVal(line)
		if !ok {
			return nil, fmt.Errorf("properties: no key-value separator in %q", raw)
		}
		key = strings.TrimSpace(unescape(key))
		val = strings.TrimSpace(unescape(val))
		out[key] = val
	}
	resolveInterpolation(out)
	return out, nil
}

// Unmarshal 解析 Properties 文本映射到结构体 v（指针），字段 tag `prop:"key"`。
func Unmarshal(data string, v any) error {
	m, err := Parse(data)
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("properties: Unmarshal requires non-nil pointer")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("properties: Unmarshal target must be struct")
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("prop")
		if tag == "" {
			continue
		}
		val, ok := m[tag]
		if !ok {
			continue
		}
		fv := rv.Field(i)
		if err := assignString(fv, val); err != nil {
			return err
		}
	}
	return nil
}

// joinContinuations 把以 \ 结尾的行与下一行合并（去掉续接符与换行）。
func joinContinuations(data string) []string {
	raw := strings.Split(data, "\n")
	var out []string
	var buf strings.Builder
	cont := false
	for _, line := range raw {
		trimmed := strings.TrimRight(line, "\r")
		if cont {
			buf.WriteString(strings.TrimSuffix(trimmed, "\\"))
			cont = strings.HasSuffix(trimmed, "\\") && len(trimmed) > 0
			if !cont {
				out = append(out, buf.String())
				buf.Reset()
			}
			continue
		}
		if strings.HasSuffix(trimmed, "\\") && len(trimmed) > 0 {
			buf.WriteString(strings.TrimSuffix(trimmed, "\\"))
			cont = true
			continue
		}
		out = append(out, trimmed)
	}
	if cont {
		out = append(out, buf.String())
	}
	return out
}

// splitKeyVal 找到键结束位置并返回 (key, value, true)。
func splitKeyVal(line string) (string, string, bool) {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '=', ':':
			if !inSingle && !inDouble {
				return line[:i], line[i+1:], true
			}
		case ' ', '\t':
			if !inSingle && !inDouble {
				// 空白分隔：键后需紧跟空白、再是值（key value 形式）。
				j := i
				for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
					j++
				}
				if j < len(line) && line[j] != '=' && line[j] != ':' {
					return line[:i], line[j:], true
				}
				// 否则继续（键内空白不合法，但保持稳健）
			}
		}
	}
	return "", "", false
}

// unescape 处理 Properties 转义：\n \t \r \\ \: \= \ 空格 以及 \uXXXX。
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 'u':
				if i+5 < len(s) {
					var r rune
					if _, err := fmt.Sscanf(s[i+2:i+6], "%X", &r); err == nil {
						b.WriteRune(r)
						i += 5
						continue
					}
				}
				b.WriteByte('\\')
			case ' ', ':', '=', '\\':
				b.WriteByte(next)
				i++
				continue
			default:
				b.WriteByte('\\')
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// resolveInterpolation 处理 ${key} 引用（已解析的键）。
func resolveInterpolation(m map[string]string) {
	for k, v := range m {
		for strings.Contains(v, "${") {
			start := strings.Index(v, "${")
			end := strings.Index(v[start:], "}")
			if end < 0 {
				break
			}
			ref := v[start+2 : start+end]
			if repl, ok := m[ref]; ok {
				v = v[:start] + repl + v[start+end+1:]
				m[k] = v
			} else {
				break
			}
		}
	}
}

// assignString 把字符串值按字段类型赋给 fv。
func assignString(fv reflect.Value, val string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n int64
		if _, err := fmt.Sscan(val, &n); err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var n uint64
		if _, err := fmt.Sscan(val, &n); err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		var f float64
		if _, err := fmt.Sscan(val, &f); err != nil {
			return err
		}
		fv.SetFloat(f)
	case reflect.Bool:
		fv.SetBool(strings.EqualFold(val, "true") || val == "1" || val == "yes" || val == "on")
	default:
		return fmt.Errorf("properties: unsupported field kind %s", fv.Kind())
	}
	return nil
}
