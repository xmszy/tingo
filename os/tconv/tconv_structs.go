package tconv

import (
	"fmt"
	"reflect"
)

// ──────────────── 多行 / 切片结构体转换 ────────────────

// Structs 将多行数据转换为结构体切片指针。
//
// params 支持：
//   - []map[string]any / []map[string]string / [][]any
//   - []struct / []*struct
//
// ptr 必须是「结构体切片/指针切片」的指针，如 *[]*User 或 *[]User。
func Structs(params any, ptr any) error {
	pv := reflect.ValueOf(params)
	if pv.Kind() == reflect.Ptr {
		pv = pv.Elem()
	}
	if pv.Kind() != reflect.Slice && pv.Kind() != reflect.Array {
		return fmt.Errorf("tconv: Structs requires slice input, got %T", params)
	}

	// 解析目标切片元素类型（*User 或 User）
	dstv := reflect.ValueOf(ptr)
	if dstv.Kind() != reflect.Ptr || dstv.IsNil() {
		return fmt.Errorf("tconv: Structs requires non-nil slice pointer, got %T", ptr)
	}
	slicev := dstv.Elem()
	if slicev.Kind() != reflect.Slice {
		return fmt.Errorf("tconv: Structs requires slice pointer target, got %T", ptr)
	}
	elemType := slicev.Type().Elem()

	var errs []string
	result := reflect.MakeSlice(slicev.Type(), 0, pv.Len())
	for i := 0; i < pv.Len(); i++ {
		elemPtr := reflect.New(elemType)
		if err := MapToStruct(pv.Index(i).Interface(), elemPtr.Interface()); err != nil {
			errs = append(errs, fmt.Sprintf("[%d]: %v", i, err))
			continue
		}
		result = reflect.Append(result, elemPtr.Elem())
	}
	slicev.Set(result)
	if len(errs) > 0 {
		return fmt.Errorf("tconv: Structs: %v", errs)
	}
	return nil
}
