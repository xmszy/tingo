// Package tconv Map/Struct 互转。
// 设计要点：
//   - 基于反射的通用 Map-to-Struct 转换，支持 json/db/tdb 标签
//   - 零外部依赖，仅用标准库 reflect
//   - 保持 tingo 简洁风格
package tconv

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// ScanOption 控制 Map-to-Struct 转换行为。
type ScanOption struct {
	// OmitEmpty 跳过源数据中的 nil/零值（不覆盖目标已有值）。
	OmitEmpty bool
	// ContinueOnError 遇到错误继续处理下一个字段，而非立即返回。
	ContinueOnError bool
	// TagName 指定结构体标签名，默认 "json"。
	TagName string
}

// MapToStruct 将 map/struct 转换为目标结构体（通过反射）。
//
// 支持输入：
//   - map[string]any / map[string]string / map[string]int 等
//   - 另一个 struct（字段名匹配）
//
// 字段匹配优先级：tag > 字段名(Snake) > 字段名(小写)
// 默认标签名 "json"。
func MapToStruct(input any, output any) error {
	return mapToStruct(input, output, ScanOption{TagName: "json"})
}

// ScanStruct 带选项的 MapToStruct。
func ScanStruct(input any, output any, opts ScanOption) error {
	if opts.TagName == "" {
		opts.TagName = "json"
	}
	return mapToStruct(input, output, opts)
}

func mapToStruct(input any, output any, opts ScanOption) error {
	// 验证 output 是非 nil 结构体指针
	ov := reflect.ValueOf(output)
	if ov.Kind() != reflect.Ptr || ov.IsNil() {
		return fmt.Errorf("tconv: output must be a non-nil pointer to struct, got %T", output)
	}
	ov = ov.Elem()
	if ov.Kind() != reflect.Struct {
		return fmt.Errorf("tconv: output must be a pointer to struct, got %T", output)
	}
	ot := ov.Type()

	// 将 input 统一为 reflect.Value
	iv := reflect.ValueOf(input)
	if iv.Kind() == reflect.Ptr {
		iv = iv.Elem()
	}

	// map 输入
	if iv.Kind() == reflect.Map {
		return mapToStructFromMap(iv, ov, ot, opts)
	}

	// struct 输入
	if iv.Kind() == reflect.Struct {
		return structToStruct(iv, ov, ot, opts)
	}

	return fmt.Errorf("tconv: unsupported input type %T", input)
}

// fieldMapEntry 是缓存解析后的「目标字段 → map key」映射，避免每次反射遍历 + 解析 tag。
type fieldMapEntry struct {
	index int
	key   reflect.Value // reflect.ValueOf(name)，直接用于 mv.MapIndex
}

// structCacheKey 缓存键：目标结构体类型 + 使用的 tag 名。
type structCacheKey struct {
	typ     reflect.Type
	tagName string
}

// structFieldCache 按 (reflect.Type, tagName) 缓存字段映射（并发安全，只读不删）。
var structFieldCache sync.Map

// cachedMapFields 返回目标结构体按 tag 解析出的字段映射，第二次起直接命中缓存。
// 映射不依赖 opts（仅取字段名），可安全复用。
func cachedMapFields(ot reflect.Type, tagName string) []fieldMapEntry {
	key := structCacheKey{ot, tagName}
	if v, ok := structFieldCache.Load(key); ok {
		return v.([]fieldMapEntry)
	}
	var entries []fieldMapEntry
	for i := 0; i < ot.NumField(); i++ {
		sf := ot.Field(i)
		if !sf.IsExported() {
			continue
		}
		fieldName := fieldNameByTag(sf, tagName)
		if fieldName == "" || fieldName == "-" {
			continue
		}
		entries = append(entries, fieldMapEntry{index: i, key: reflect.ValueOf(fieldName)})
	}
	structFieldCache.Store(key, entries)
	return entries
}

// cachedStructByName 缓存「目标结构体按 name 检索的字段索引」，供 structToStruct 复用。
// 优先级：tag 名 > 原始字段名 > 小写字段名（命中第一个即可）。
var structByNameCache sync.Map

func cachedStructByName(ot reflect.Type, tagName string) map[string]int {
	key := structCacheKey{ot, tagName}
	if v, ok := structByNameCache.Load(key); ok {
		return v.(map[string]int)
	}
	idx := make(map[string]int)
	for i := 0; i < ot.NumField(); i++ {
		sf := ot.Field(i)
		if !sf.IsExported() {
			continue
		}
		if name := fieldNameByTag(sf, tagName); name != "" && name != "-" {
			if _, exists := idx[name]; !exists {
				idx[name] = i
			}
		}
		if _, exists := idx[sf.Name]; !exists {
			idx[sf.Name] = i
		}
		if lo := strings.ToLower(sf.Name); lo != "" {
			if _, exists := idx[lo]; !exists {
				idx[lo] = i
			}
		}
	}
	structByNameCache.Store(key, idx)
	return idx
}

func mapToStructFromMap(mv reflect.Value, ov reflect.Value, ot reflect.Type, opts ScanOption) error {
	entries := cachedMapFields(ot, opts.TagName)

	var errs []string
	for _, e := range entries {
		val := mv.MapIndex(e.key)
		if !val.IsValid() {
			continue
		}

		if opts.OmitEmpty && val.IsZero() {
			continue
		}

		fv := ov.Field(e.index)
		if !fv.CanSet() {
			continue
		}

		if err := setFieldValue(fv, val); err != nil {
			if opts.ContinueOnError {
				sf := ot.Field(e.index)
				errs = append(errs, fmt.Sprintf("%s: %v", sf.Name, err))
				continue
			}
			return fmt.Errorf("tconv: field %s: %w", ot.Field(e.index).Name, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("tconv: %s", strings.Join(errs, "; "))
	}
	return nil
}

func structToStruct(sv reflect.Value, ov reflect.Value, ot reflect.Type, opts ScanOption) error {
	tagName := opts.TagName
	st := sv.Type()
	// 源字段名→索引也走缓存，避免每次反射遍历源结构体（与目标字段缓存对称）。
	srcByName := cachedStructByName(st, tagName)
	dstByName := cachedStructByName(ot, tagName)

	var errs []string
	for name, di := range dstByName {
		si, ok := srcByName[name]
		if !ok {
			continue
		}
		svField := sv.Field(si)

		if opts.OmitEmpty && svField.IsZero() {
			continue
		}

		fv := ov.Field(di)
		if !fv.CanSet() {
			continue
		}

		if err := setFieldValue(fv, svField); err != nil {
			if opts.ContinueOnError {
				sf := ot.Field(di)
				errs = append(errs, fmt.Sprintf("%s: %v", sf.Name, err))
				continue
			}
			return fmt.Errorf("tconv: field %s: %w", ot.Field(di).Name, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("tconv: %s", strings.Join(errs, "; "))
	}
	return nil
}

// fieldNameByTag 从 struct tag 中提取字段名。
func fieldNameByTag(sf reflect.StructField, tagName string) string {
	tag := sf.Tag.Get(tagName)
	if tag == "" {
		return ""
	}
	// 处理 json:"name,omitempty" 格式
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		tag = tag[:idx]
	}
	return tag
}

// setFieldValue 直接基于 reflect.Value 赋值，避免调用方先 .Interface() 再 reflect.ValueOf 的来回装箱。
// 这是热路径（ORM 扫描 / 参数绑定）的核心优化点，与 GoFrame 直接反射操作的思路一致。
func setFieldValue(fv reflect.Value, vv reflect.Value) error {
	if !vv.IsValid() {
		return nil
	}
	// 处理 map 中的 nil interface 零值
	if vv.Kind() == reflect.Interface && vv.IsNil() {
		return nil
	}

	ft := fv.Type()
	vt := vv.Type()

	// 同类型直接赋值
	if vt == ft {
		fv.Set(vv)
		return nil
	}

	// 类型转换
	if vt.ConvertibleTo(ft) {
		fv.Set(vv.Convert(ft))
		return nil
	}

	// 特殊类型处理
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(String(vv.Interface()))
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fv.SetInt(Int64(vv.Interface()))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fv.SetUint(Uint64(vv.Interface()))
		return nil
	case reflect.Float32, reflect.Float64:
		fv.SetFloat(Float64(vv.Interface()))
		return nil
	case reflect.Bool:
		fv.SetBool(Bool(vv.Interface()))
		return nil
	case reflect.Slice:
		if ft.Elem().Kind() == reflect.Uint8 { // []byte
			fv.SetBytes(Bytes(vv.Interface()))
			return nil
		}
		return setSlice(fv, vv)
	case reflect.Struct:
		if ft == reflect.TypeOf(time.Time{}) {
			fv.Set(reflect.ValueOf(Time(vv.Interface())))
			return nil
		}
		// 尝试递归 MapToStruct
		if vv.Kind() == reflect.Map || (vv.Kind() == reflect.Ptr && vv.Elem().Kind() == reflect.Struct) {
			nv := reflect.New(ft)
			if err := mapToStruct(vv.Interface(), nv.Interface(), ScanOption{}); err != nil {
				return err
			}
			fv.Set(nv.Elem())
			return nil
		}
	}

	return fmt.Errorf("cannot convert %s to %s", vt, ft)
}

func setSlice(fv reflect.Value, vv reflect.Value) error {
	if vv.Kind() != reflect.Slice {
		return fmt.Errorf("cannot convert %T to slice", vv.Interface())
	}
	elemType := fv.Type().Elem()
	n := vv.Len()
	ns := reflect.MakeSlice(fv.Type(), n, n)
	for i := 0; i < n; i++ {
		ev := ns.Index(i)
		sv := vv.Index(i)
		if sv.Type() == elemType {
			ev.Set(sv)
		} else if sv.Type().ConvertibleTo(elemType) {
			ev.Set(sv.Convert(elemType))
		} else {
			// 递归嵌套 struct
			if elemType.Kind() == reflect.Struct {
				if sv.Kind() == reflect.Map || sv.Kind() == reflect.Struct {
					tmp := reflect.New(elemType)
					if err := mapToStruct(sv.Interface(), tmp.Interface(), ScanOption{}); err != nil {
						return err
					}
					ev.Set(tmp.Elem())
					continue
				}
			}
			return fmt.Errorf("cannot convert slice element %T to %s", sv.Interface(), elemType)
		}
	}
	fv.Set(ns)
	return nil
}
