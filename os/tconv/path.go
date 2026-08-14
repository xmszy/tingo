package tconv

import (
	"reflect"
	"strconv"
	"strings"
)

// GetByPath 按动态路径从嵌套结构（map / slice / struct）中取值，
// 路径支持点号分隔与数组下标，例如 "user.addr.0.city" 或 "a.b.2"。
//
// 支持的数据类型：
//   - map[string]any / map[string]T
//   - 任意切片/数组（下标用 .N）
//   - 结构体（字段名或 tag 名，如 json:"name"）
//
// 返回 (value, true)；路径不存在或类型不匹配时返回 (nil, false)。
//
// 例：
//
//	v, ok := tconv.GetByPath(data, "orders.0.amount")
func GetByPath(root any, path string) (any, bool) {
	if root == nil || path == "" {
		return root, root != nil
	}
	cur := reflect.ValueOf(root)
	segs := strings.Split(path, ".")
	for _, seg := range segs {
		if !cur.IsValid() {
			return nil, false
		}
		// 解引用指针与接口，得到可导航的底层值。
		for cur.Kind() == reflect.Pointer || cur.Kind() == reflect.Interface {
			if !cur.IsValid() {
				return nil, false
			}
			cur = cur.Elem()
		}
		if !cur.IsValid() {
			return nil, false
		}
		switch cur.Kind() {
		case reflect.Map:
			k := reflect.ValueOf(seg)
			if cur.Type().Key().Kind() != reflect.String {
				return nil, false
			}
			elem := cur.MapIndex(k)
			if !elem.IsValid() {
				return nil, false
			}
			cur = elem
		case reflect.Slice, reflect.Array:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= cur.Len() {
				return nil, false
			}
			cur = cur.Index(idx)
		case reflect.Struct:
			cur = structFieldValueByName(cur, seg)
			if !cur.IsValid() {
				return nil, false
			}
		default:
			// 标量后再遇到路径段即不可达。
			return nil, false
		}
	}
	if !cur.IsValid() {
		return nil, false
	}
	return cur.Interface(), true
}

// MustGetByPath 同 GetByPath，但路径不存在时返回零值而非 (nil, false)。
func MustGetByPath(root any, path string) any {
	v, _ := GetByPath(root, path)
	return v
}

// structFieldValueByName 按字段名或 tag（json/gconv/property 等）查找结构体字段值。
func structFieldValueByName(v reflect.Value, name string) reflect.Value {
	rt := v.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.Name == name {
			return v.Field(i)
		}
		// 常见 tag 名称匹配。
		for _, tag := range []string{"json", "gconv", "param", "property", "yaml", "ini", "prop"} {
			if tv, ok := sf.Tag.Lookup(tag); ok {
				tv = strings.Split(tv, ",")[0]
				if tv != "" && tv == name {
					return v.Field(i)
				}
			}
		}
	}
	return reflect.Value{}
}
