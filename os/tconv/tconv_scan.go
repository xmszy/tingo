package tconv

import "reflect"

// ──────────────── 通用扫描入口 ────────────────
//
// Scan 依据目标指针的底层类型，自动分派到 Struct / Structs / Map 转换。
// 适用于「未知来源数据 → 明确目标类型」的场景（配置解析、DB 行映射等）。

// Scan 将 v 扫描到 dst（dst 必须是指针）。
//   - *struct → 委托 MapToStruct
//   - *[]struct → 委托 Structs
//   - *map → 委托 MapToStruct
func Scan(dst any, v any) error {
	return ScanWithOptions(dst, v, ScanOption{TagName: "json"})
}

// ScanWithOptions 带选项的 Scan。
func ScanWithOptions(dst any, v any, opts ScanOption) error {
	if opts.TagName == "" {
		opts.TagName = "json"
	}
	dv := reflect.ValueOf(dst)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return errNeedPtr
	}
	elem := dv.Elem()
	switch elem.Kind() {
	case reflect.Struct:
		return MapToStruct(v, dst)
	case reflect.Slice:
		return Structs(v, dst)
	case reflect.Map:
		return MapToStruct(v, dst)
	default:
		return errUnsupportedScan(elem.Kind().String())
	}
}

var errNeedPtr = &scanError{"tconv: Scan dst must be non-nil pointer"}

type scanError struct{ msg string }

func (e *scanError) Error() string { return e.msg }

func errUnsupportedScan(kind string) error {
	return &scanError{"tconv: Scan unsupported dst kind: " + kind}
}
