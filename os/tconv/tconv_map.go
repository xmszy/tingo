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

func mapToStructFromMap(mv reflect.Value, ov reflect.Value, ot reflect.Type, opts ScanOption) error {
	tagName := opts.TagName

	var errs []string
	for i := 0; i < ot.NumField(); i++ {
		sf := ot.Field(i)
		if !sf.IsExported() {
			continue
		}

		// 获取字段名（tag 优先）
		fieldName := fieldNameByTag(sf, tagName)
		if fieldName == "" || fieldName == "-" {
			continue
		}

		// 从 map 取值
		mk := reflect.ValueOf(fieldName)
		val := mv.MapIndex(mk)
		if !val.IsValid() {
			continue
		}

		if opts.OmitEmpty && val.IsZero() {
			continue
		}

		fv := ov.Field(i)
		if !fv.CanSet() {
			continue
		}

		if err := setField(fv, val.Interface()); err != nil {
			if opts.ContinueOnError {
				errs = append(errs, fmt.Sprintf("%s: %v", sf.Name, err))
				continue
			}
			return fmt.Errorf("tconv: field %s: %w", sf.Name, err)
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
	svFields := make(map[string]reflect.Value)

	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if !f.IsExported() {
			continue
		}
		key := fieldNameByTag(f, tagName)
		if key == "" {
			key = f.Name
		}
		svFields[key] = sv.Field(i)
		svFields[strings.ToLower(f.Name)] = sv.Field(i)
	}

	var errs []string
	for i := 0; i < ot.NumField(); i++ {
		sf := ot.Field(i)
		if !sf.IsExported() {
			continue
		}

		fieldName := fieldNameByTag(sf, tagName)
		if fieldName == "" || fieldName == "-" {
			continue
		}

		svField, ok := svFields[fieldName]
		if !ok {
			svField, ok = svFields[sf.Name]
		}
		if !ok {
			svField, ok = svFields[strings.ToLower(sf.Name)]
		}
		if !ok {
			continue
		}

		if opts.OmitEmpty && svField.IsZero() {
			continue
		}

		fv := ov.Field(i)
		if !fv.CanSet() {
			continue
		}

		if err := setField(fv, svField.Interface()); err != nil {
			if opts.ContinueOnError {
				errs = append(errs, fmt.Sprintf("%s: %v", sf.Name, err))
				continue
			}
			return fmt.Errorf("tconv: field %s: %w", sf.Name, err)
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

// setField 将 value 写入目标 reflect.Value，自动类型转换。
func setField(fv reflect.Value, val any) error {
	if val == nil {
		return nil
	}

	vv := reflect.ValueOf(val)
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
		fv.SetString(String(val))
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fv.SetInt(Int64(val))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fv.SetUint(Uint64(val))
		return nil
	case reflect.Float32, reflect.Float64:
		fv.SetFloat(Float64(val))
		return nil
	case reflect.Bool:
		fv.SetBool(Bool(val))
		return nil
	case reflect.Slice:
		if ft.Elem().Kind() == reflect.Uint8 { // []byte
			fv.SetBytes(Bytes(val))
			return nil
		}
		return setSlice(fv, vv)
	case reflect.Struct:
		if ft == reflect.TypeOf(time.Time{}) {
			fv.Set(reflect.ValueOf(Time(val)))
			return nil
		}
		// 尝试递归 MapToStruct
		if vv.Kind() == reflect.Map || (vv.Kind() == reflect.Ptr && vv.Elem().Kind() == reflect.Struct) {
			nv := reflect.New(ft)
			if err := mapToStruct(val, nv.Interface(), ScanOption{}); err != nil {
				return err
			}
			fv.Set(nv.Elem())
			return nil
		}
	}

	return fmt.Errorf("cannot convert %T to %s", val, ft)
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
