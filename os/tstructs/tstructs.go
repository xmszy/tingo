// Package tstructs 提供结构体反射工具。
// 设计要点：
//   - 基于标准库 reflect，零外部依赖。
//   - 提供 struct tag 解析、字段遍历、类型检查等工具。
package tstructs

import (
	"reflect"
	"strconv"
	"strings"
)

// ──────────────── Tag 标准常量 ────────────────

const (
	TagJson        = "json"        // JSON 序列化字段名
	TagValid       = "valid"       // 校验规则
	TagTdb         = "tdb"         // 数据库列名
	TagDB          = "db"          // 数据库列名（备用）
	TagDescription = "description" // 字段描述
	TagDefault     = "default"     // 默认值
	TagParam       = "param"       // 请求参数名
	TagExample     = "example"     // 示例值
	TagIn          = "in"          // 输入方向标记
	TagOut         = "out"         // 输出方向标记
	TagSummary     = "summary"     // 摘要
)

// DefaultTagPriority 默认 tag 查找优先级。
var DefaultTagPriority = []string{TagJson, TagTdb, TagDB, TagParam}

// ──────────────── Tag 解析 ────────────────

// ParseTag 将原始 tag 字符串解析为 key→value map。
// 支持两种格式：
//   - k:"v"  (Go struct tag 标准格式)
//   - k1:v1|k2:v2  (如 tvalid 的 valid tag)
//
// 例: "required|len:3,20|in:1,2,3" → {"required":"", "len":"3,20", "in":"1,2,3"}
func ParseTag(tag string) map[string]string {
	if tag == "" {
		return nil
	}
	m := make(map[string]string)
	for _, seg := range strings.Split(tag, "|") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		idx := strings.IndexByte(seg, ':')
		if idx < 0 {
			m[seg] = ""
		} else {
			m[seg[:idx]] = seg[idx+1:]
		}
	}
	return m
}

// ParseTagStruct 解析 struct tag 格式的字符串（k:"v" 形式）。
func ParseTagStruct(tag string) map[string]string {
	m := make(map[string]string)
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' {
			break
		}
		key := tag[:i]
		tag = tag[i+1:]
		// 跳过引号内的值
		if tag[0] == '"' {
			i = 1
			for i < len(tag) && tag[i] != '"' {
				i++
			}
			m[key] = tag[1:i]
			if i+1 < len(tag) {
				tag = tag[i+1:]
			} else {
				break
			}
		} else {
			i = 0
			for i < len(tag) && tag[i] > ' ' && tag[i] != ':' {
				i++
			}
			m[key] = tag[:i]
			tag = tag[i:]
		}
	}
	return m
}

// ──────────────── Field 结构体 ────────────────

// Field 封装 struct 字段信息，提供便捷的 tag 读取方法。
type Field struct {
	Name       string        // Go 字段名
	Index      int           // 字段索引
	Tag        reflect.StructTag
	Type       reflect.Type
	Value      reflect.Value
	IsEmbedded bool // 是否为匿名嵌入字段
	IsExported bool
}

// TagMap 返回该字段所有 tag 的 key→value map（使用 ParseTagStruct 解析）。
func (f *Field) TagMap() map[string]string {
	return ParseTagStruct(string(f.Tag))
}

// TagLookup 查找 tag 键的值，返回 (value, 是否存在)。
func (f *Field) TagLookup(key string) (string, bool) {
	v, ok := f.Tag.Lookup(key)
	return v, ok
}

// TagDefault 查找 tag 键的值，不存在则返回默认值。
func (f *Field) TagDefault(key, def string) string {
	if v, ok := f.Tag.Lookup(key); ok {
		return v
	}
	return def
}

// TagJsonName 返回 json tag 中的字段名（去除 omitempty 等选项）。
func (f *Field) TagJsonName() string {
	v := f.Tag.Get(TagJson)
	if idx := strings.IndexByte(v, ','); idx >= 0 {
		return v[:idx]
	}
	return v
}

// TagDbName 按优先级查找数据库列名（tdb → db → json）。
func (f *Field) TagDbName() string {
	for _, p := range DefaultTagPriority {
		if v := f.Tag.Get(p); v != "" {
			if idx := strings.IndexByte(v, ','); idx >= 0 {
				return v[:idx]
			}
			return v
		}
	}
	return f.Name
}

// HasValidRule 判断是否定义了指定校验规则。
func (f *Field) HasValidRule(rule string) bool {
	rules := ParseTag(f.Tag.Get(TagValid))
	_, ok := rules[rule]
	return ok
}

// ──────────────── Fields 遍历 ────────────────

// FieldsInput 控制 FieldsInfo 遍历行为。
type FieldsInput struct {
	Object    any  // 目标 struct/指针
	Recursive bool // 是否递归展开嵌入字段
}

// FieldsInfo 返回 struct 的所有字段信息（[]Field）。
func FieldsInfo(in FieldsInput) []Field {
	t := reflect.TypeOf(in.Object)
	v := reflect.ValueOf(in.Object)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return collectFields(t, v, in.Recursive)
}

func collectFields(t reflect.Type, v reflect.Value, recursive bool) []Field {
	n := t.NumField()
	fields := make([]Field, 0, n)
	for i := 0; i < n; i++ {
		sf := t.Field(i)
		embedded := sf.Anonymous
		exported := sf.IsExported()

		if recursive && embedded {
			sub := collectFields(sf.Type, v.Field(i), true)
			fields = append(fields, sub...)
			continue
		}
		if !exported && !embedded {
			continue
		}
		fields = append(fields, Field{
			Name:       sf.Name,
			Index:      i,
			Tag:        sf.Tag,
			Type:       sf.Type,
			Value:      v.Field(i),
			IsEmbedded: embedded && !recursive,
			IsExported: exported,
		})
	}
	return fields
}

// ──────────────── 工具函数 ────────────────

// TagMapByName 按 tag 优先级返回 tag值→字段名 的 map。
func TagMapByName(v any, priority []string) map[string]string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	result := make(map[string]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		for _, p := range priority {
			if tv := f.Tag.Get(p); tv != "" {
				name := tv
				if idx := strings.IndexByte(name, ','); idx >= 0 {
					name = name[:idx]
				}
				if name != "" && name != "-" {
					result[name] = f.Name
				}
				break
			}
		}
	}
	return result
}

// TagValue 获取 struct 字段的 tag 值。
func TagValue(v any, fieldName, tagKey string) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	field, ok := t.FieldByName(fieldName)
	if !ok {
		return ""
	}
	return field.Tag.Get(tagKey)
}

// TagMap 返回指定 tag 的 map[string]string（字段名 → tag 值）。
func TagMap(v any, tagKey string) map[string]string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	result := make(map[string]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		result[f.Name] = f.Tag.Get(tagKey)
	}
	return result
}

// Fields 返回 struct 的所有导出字段名。
func Fields(v any) []string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() {
			names = append(names, f.Name)
		}
	}
	return names
}

// FieldMap 返回字段名 → 字段值的 map（仅导出字段）。
func FieldMap(v any) map[string]any {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()
	if typ.Kind() != reflect.Struct {
		return nil
	}
	result := make(map[string]any, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		result[f.Name] = val.Field(i).Interface()
	}
	return result
}

// SetField 设置 struct 字段值（指针传入）。
func SetField(v any, fieldName string, value any) {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return
	}
	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return
	}
	f := val.FieldByName(fieldName)
	if !f.IsValid() || !f.CanSet() {
		return
	}
	fv := reflect.ValueOf(value)
	if fv.Type().AssignableTo(f.Type()) {
		f.Set(fv)
		return
	}
	// 尝试数值类型转换
	if f.Kind() >= reflect.Int && f.Kind() <= reflect.Int64 {
		f.SetInt(toInt64(fv))
		return
	}
	if f.Kind() >= reflect.Uint && f.Kind() <= reflect.Uint64 {
		f.SetUint(toUint64(fv))
		return
	}
	if f.Kind() >= reflect.Float32 && f.Kind() <= reflect.Float64 {
		f.SetFloat(toFloat64(fv))
		return
	}
}

func toInt64(v reflect.Value) int64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Float32, reflect.Float64:
		return int64(v.Float())
	case reflect.String:
		n, _ := strconv.ParseInt(v.String(), 10, 64)
		return n
	}
	return 0
}

func toUint64(v reflect.Value) uint64 {
	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int())
	case reflect.Float32, reflect.Float64:
		return uint64(v.Float())
	case reflect.String:
		n, _ := strconv.ParseUint(v.String(), 10, 64)
		return n
	}
	return 0
}

func toFloat64(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.String:
		n, _ := strconv.ParseFloat(v.String(), 64)
		return n
	}
	return 0
}

// IsStruct 判断是否为结构体类型。
func IsStruct(v any) bool {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

// TypeName 返回类型的短名称（不含包路径）。
func TypeName(v any) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := t.String()
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}
