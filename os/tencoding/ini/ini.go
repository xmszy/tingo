// Package ini 提供轻量 INI 格式解析（零外部依赖）。
//
// 支持：
//   - [section] 分段；无段内容归入默认段 ""。
//   - key = value 与 key:value 两种分隔符，= 优先。
//   - # 与 ; 注释行、行尾注释（值内 # ; 保留）。
//   - 折叠引号（"..." / '...'）。
//   - 简单变量插值：%(otherkey) 引用同段或全局键。
//
// 解析为 map[string]map[string]string（段 → 键 → 值）；
// 另提供 Unmarshal 直接映射到结构体（字段 tag `ini:"key"` 或 `ini:"section.key"`）。
package ini

import (
	"fmt"
	"reflect"
	"strings"
)

// Sections 是 INI 解析结果：段名 → 键值对。默认段键为 ""。
type Sections map[string]map[string]string

// Parse 解析 INI 文本为分段键值对。
func Parse(data string) (Sections, error) {
	sec := ""
	out := Sections{}
	get := func() map[string]string {
		m, ok := out[sec]
		if !ok {
			m = map[string]string{}
			out[sec] = m
		}
		return m
	}

	lines := strings.Split(data, "\n")
	for n, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sec = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		// 分隔符：优先第一个未加引号的 '='，其次 ':'。
		sep := "="
		idx := firstUnquoted(line, '=')
		if idx < 0 {
			idx = firstUnquoted(line, ':')
			sep = ":"
		}
		if idx < 0 {
			return nil, fmt.Errorf("ini: line %d: missing '=' or ':' (%q)", n+1, raw)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+len(sep):])
		// 去行尾注释（未加引号时）。
		if i := firstUnquoted(val, ';'); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		val = unquote(val)
		get()[key] = val
	}

	// 变量插值：%(key) 引用同段或默认段。
	resolveInterpolation(out)
	return out, nil
}

// Unmarshal 解析 INI 文本并映射到结构体 v（指针）。
// 字段 tag 形如 `ini:"key"`（默认段）或 `ini:"section.key"`。
func Unmarshal(data string, v any) error {
	secs, err := Parse(data)
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("ini: Unmarshal requires non-nil pointer")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("ini: Unmarshal target must be struct")
	}

	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("ini")
		if tag == "" {
			continue
		}
		section, key := "", tag
		if i := strings.Index(tag, "."); i >= 0 {
			section, key = tag[:i], tag[i+1:]
		}
		m, ok := secs[section]
		if !ok {
			continue
		}
		val, ok := m[key]
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
		return fmt.Errorf("ini: unsupported field kind %s", fv.Kind())
	}
	return nil
}

// resolveInterpolation 处理 %(key) 与 %(section.key) 引用。
func resolveInterpolation(secs Sections) {
	for s, m := range secs {
		for k, v := range m {
			for strings.Contains(v, "%(") {
				start := strings.Index(v, "%(")
				end := strings.Index(v[start:], ")")
				if end < 0 {
					break
				}
				ref := v[start+2 : start+end]
				rs, rk := s, ref
				if i := strings.Index(ref, "."); i >= 0 {
					rs, rk = ref[:i], ref[i+1:]
				}
				lookup := secs[rs]
				if lookup != nil {
					if repl, ok := lookup[rk]; ok {
						v = v[:start] + repl + v[start+end+1:]
						m[k] = v
						continue
					}
				}
				break
			}
		}
	}
}

// firstUnquoted 返回字符 c 在 s 中首次出现且未被引号包裹的位置；未找到返回 -1。
func firstUnquoted(s string, c byte) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case c:
			if !inSingle && !inDouble {
				return i
			}
		}
	}
	return -1
}

// unquote 去掉首尾配对的引号。
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
