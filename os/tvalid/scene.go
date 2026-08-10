package tvalid

import (
	"fmt"
	"reflect"
	"strings"
)

// ──────────────── 验证场景（Scene）支持 ────────────────
//
// 同一结构体不同操作可以有不同的验证规则。
//
// Tag 约定：
//   - valid      标签 → 所有场景通用规则
//   - valid-xxx  标签 → 仅 xxx 场景使用
//   - valid="-"  标签 → 该字段不参与任何校验（包括场景）
//
// 示例：
//
//	type User struct {
//	    Name  string `valid:"required|len:3,20" label:"用户名"`
//	    Email string `valid:"email" label:"邮箱"`
//	    Age   int    `valid-update:"min:18" valid-create:"required|min:1" label:"年龄"`
//	}
//
//校验时指定场景：
//
//	err := tvalid.CheckStructWithScene(user, "create")
//	err := tvalid.CheckStructWithScene(user, "update")

// ──────────────── 场景校验 ────────────────

// CheckStructWithScene 使用默认校验器按场景校验结构体。
// scene 为空时退化为 CheckStruct（仅使用 valid tag 规则）。
func CheckStructWithScene(v any, scene string) error {
	return defaultValidator.CheckStructWithScene(v, scene)
}

// CheckStructWithScene 按场景校验结构体。
func (v *Validator) CheckStructWithScene(obj any, scene string) error {
	if !v.frozen {
		v.freeze()
	}
	rv := reflect.ValueOf(obj)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("tvalid: CheckStructWithScene requires struct, got %T", obj)
	}
	rt := rv.Type()

	var errs Errors
	data := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).IsExported() {
			name := rt.Field(i).Name
			if l := rt.Field(i).Tag.Get("label"); l != "" {
				name = l
			}
			data[name] = rv.Field(i).Interface()
		}
	}
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		// 获取场景规则：valid-{scene} 优先，valid 作为兜底
		tag, skip := resolveSceneTag(field, scene)
		if skip {
			continue
		}
		if tag == "" {
			continue
		}

		fieldRules := parseTagRules(tag)
		value := rv.Field(i).Interface()
		fieldName := field.Name
		if label := field.Tag.Get("label"); label != "" {
			fieldName = label
		}

		for _, rule := range fieldRules {
			if err := v.checkRule(fieldName, rule, value, data); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// resolveSceneTag 解析场景 tag。
// 返回 (生效的规则字符串, 是否跳过该字段)。
func resolveSceneTag(field reflect.StructField, scene string) (string, bool) {
	// valid="-" 表示不参与任何校验
	if tag := field.Tag.Get("valid"); tag == "-" {
		return "", true
	}

	if scene == "" {
		// 无场景：只用 valid tag
		return field.Tag.Get("valid"), false
	}

	// 优先使用 scene 专属规则
	sceneTagKey := "valid-" + scene
	if sceneTag := field.Tag.Get(sceneTagKey); sceneTag != "" {
		// scene 专属规则存在
		if sceneTag == "-" {
			// 场景中明确排除该字段
			return "", true
		}
		// 合并通用规则 + 场景专属规则
		commonTag := field.Tag.Get("valid")
		if commonTag == "" {
			return sceneTag, false
		}
		return mergeTags(commonTag, sceneTag), false
	}

	// 无场景专属规则：使用 valid tag
	return field.Tag.Get("valid"), false
}

// mergeTags 合并两个 tag 规则字符串。
// 同名字段场景规则覆盖通用规则（场景优先）。
func mergeTags(common, scene string) string {
	// 解析通用规则
	commonRules := parseTagRules(common)
	sceneRules := parseTagRules(scene)

	// 构建场景规则名集合，避免重复
	sceneNames := make(map[string]struct{}, len(sceneRules))
	for _, r := range sceneRules {
		sceneNames[r.Name] = struct{}{}
	}

	// 合并：通用规则中不在场景中的保留
	merged := make([]Rule, 0, len(commonRules)+len(sceneRules))
	for _, r := range commonRules {
		if _, exists := sceneNames[r.Name]; !exists {
			merged = append(merged, r)
		}
	}
	merged = append(merged, sceneRules...)

	// 序列化回 tag 字符串
	parts := make([]string, len(merged))
	for i, r := range merged {
		if r.Param != "" {
			parts[i] = r.Name + ":" + r.Param
		} else {
			parts[i] = r.Name
		}
	}
	return strings.Join(parts, "|")
}

// ──────────────── 批量场景 ────────────────

// CheckStructWithScenes 使用一组场景来校验（所有场景均需通过）。
// 适用场景：同时属于 create 和 admin 场景的校验规则。
func (v *Validator) CheckStructWithScenes(obj any, scenes ...string) error {
	for _, scene := range scenes {
		if err := v.CheckStructWithScene(obj, scene); err != nil {
			return err
		}
	}
	return nil
}

