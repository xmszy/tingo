package tconvert

import (
	"reflect"
)

// ScanList 将源切片的值按指定字段映射到目标切片。
//
// 类似 GoFrame gconv.ScanList，将嵌套的关联数据填充到结构体中指定字段。
//
// 用法：
//
//	type User struct {
//	    Id      int
//	    Profile Profile `scan:"Profile"` // 将从 scanList 中按外键匹配填充
//	}
//	users := []User{{Id: 1}, {Id: 2}}
//	profiles := []Profile{{UserId: 1, Bio: "hello"}, {UserId: 2, Bio: "world"}}
//	tconvert.ScanList(&users, &profiles, "Profile", "Id", "UserId")
func ScanList(dst, src any, fieldName, primaryKey, foreignKey string) error {
	dstVal := reflect.ValueOf(dst)
	if dstVal.Kind() != reflect.Pointer || dstVal.Elem().Kind() != reflect.Slice {
		return errDstSlicePtr
	}
	dstSlice := dstVal.Elem()

	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Pointer || srcVal.Elem().Kind() != reflect.Slice {
		return errSrcSlicePtr
	}
	srcSlice := srcVal.Elem()

	dstLen := dstSlice.Len()
	srcLen := srcSlice.Len()
	if dstLen == 0 || srcLen == 0 {
		return nil
	}

	// 找到目标中的关联字段类型
	dstElemType := dstSlice.Type().Elem()
	if dstElemType.Kind() == reflect.Pointer {
		dstElemType = dstElemType.Elem()
	}

	fieldIdx, err := findFieldIndex(dstElemType, fieldName)
	if err != nil {
		return err
	}
	dstField := dstElemType.Field(fieldIdx)

	// 构建源数据索引：map[foreignValue]sourceValue
	srcMap := make(map[any]reflect.Value)
	for i := 0; i < srcLen; i++ {
		srcItem := srcSlice.Index(i)
		if srcItem.Kind() == reflect.Pointer {
			srcItem = srcItem.Elem()
		}
		fkVal := findFieldValue(srcItem, foreignKey)
		if fkVal.IsValid() {
			srcMap[fkVal.Interface()] = srcSlice.Index(i)
		}
	}

	// 填充目标
	for i := 0; i < dstLen; i++ {
		dstItem := dstSlice.Index(i)
		if dstItem.Kind() == reflect.Pointer {
			if dstItem.IsNil() {
				continue
			}
			dstItem = dstItem.Elem()
		}
		pkVal := findFieldValue(dstItem, primaryKey)
		if !pkVal.IsValid() {
			continue
		}

		if srcItem, ok := srcMap[pkVal.Interface()]; ok {
			relationField := dstItem.Field(fieldIdx)
			if !relationField.CanSet() {
				continue
			}

			// 设置关联字段
			if dstField.Type.Kind() == reflect.Pointer {
				// 字段是指针类型
				relationField.Set(srcItem)
			} else if dstField.Type.Kind() == reflect.Slice {
				// 字段是切片类型——这里的实现是针对 HasMany
				sliceVal := reflect.MakeSlice(dstField.Type, 1, 1)
				if srcItem.Kind() == reflect.Pointer {
					sliceVal.Index(0).Set(srcItem.Elem())
				} else {
					sliceVal.Index(0).Set(srcItem)
				}
				relationField.Set(sliceVal)
			} else {
				// 字段是值类型
				if srcItem.Kind() == reflect.Pointer {
					relationField.Set(srcItem.Elem())
				} else {
					relationField.Set(srcItem)
				}
			}
		}
	}

	return nil
}

func findFieldIndex(t reflect.Type, name string) (int, error) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Name == name {
			return i, nil
		}
		// 检查 scan 标签
		if scan := f.Tag.Get("scan"); scan == name {
			return i, nil
		}
	}
	return 0, errFieldNotFound
}

func findFieldValue(v reflect.Value, name string) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Name == name {
			return v.Field(i)
		}
		// 检查 json 标签
		if jsonTag := f.Tag.Get("json"); jsonTag == name {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

var (
	errDstSlicePtr  = newError("dst must be a pointer to slice")
	errSrcSlicePtr  = newError("src must be a pointer to slice")
	errFieldNotFound = newError("field not found")
)

func newError(msg string) error {
	return &scanError{msg: msg}
}

type scanError struct {
	msg string
}

func (e *scanError) Error() string { return e.msg }
