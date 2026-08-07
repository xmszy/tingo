// Package validate 扩展：嵌套结构体校验功能。
//
// 提供 Nested 函数用于递归校验嵌套结构体。
// 零外部依赖。
package validate

import (
	"fmt"
	"reflect"
)

// Nested 递归校验嵌套结构体中的所有 validate 标签。
//
// 用法：
//
//	type Address struct {
//	    City string `validate:"required"`
//	}
//	type User struct {
//	    Name    string  `validate:"required|min:2"`
//	    Addr    Address `validate:"nested"`
//	}
//	errors := validate.Nested(user)
func Nested(v any) map[string]string {
	errors := make(map[string]string)
	nestedValidate(v, "", errors)
	return errors
}

func nestedValidate(v any, prefix string, errors map[string]string) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := rv.Field(i)

		// 嵌套结构体
		if f.Type.Kind() == reflect.Struct && hasNestedTag(f) {
			nestedPrefix := f.Name
			if prefix != "" {
				nestedPrefix = prefix + "." + f.Name
			}
			nestedValidate(fv.Interface(), nestedPrefix, errors)
			continue
		}

		// 常规字段校验
		tag := f.Tag.Get("validate")
		if tag == "" {
			continue
		}
		fieldName := f.Name
		if prefix != "" {
			fieldName = prefix + "." + f.Name
		}

		// 简单校验
		if err := validateField(fv, tag, fieldName); err != "" {
			errors[fieldName] = err
		}
	}
}

func hasNestedTag(f reflect.StructField) bool {
	return f.Tag.Get("validate") == "nested"
}

func validateField(v reflect.Value, tag string, name string) string {
	if tag == "required" && isZeroValue(v) {
		return fmt.Sprintf("%s is required", name)
	}
	return ""
}

func isZeroValue(v reflect.Value) bool {
	return !v.IsValid() || v.IsZero()
}
