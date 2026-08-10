package tvalid

// ──────────────── 便捷校验入口 ────────────────
//
// 除结构体校验 CheckStruct 外，提供基于 map 与单值的快速校验入口，
// 满足 API 参数、表单等非结构体场景。

// CheckMap 校验 data 中各字段，rules 为「字段 → 规则字符串」或「字段 → []string 序列规则」。
// msgs 可选，传入「字段 → 规则 → 自定义消息」覆盖默认提示。
func (v *Validator) CheckMap(data map[string]any, rules map[string]any, msgs ...map[string]map[string]string) error {
	if !v.frozen {
		v.freeze()
	}
	custom := map[string]map[string]string{}
	if len(msgs) > 0 && msgs[0] != nil {
		custom = msgs[0]
	}
	var errs Errors
	for field, spec := range rules {
		var ruleList []Rule
		switch r := spec.(type) {
		case string:
			ruleList = parseTagRules(r)
		case []string:
			for _, rs := range r {
				ruleList = append(ruleList, parseTagRules(rs)...)
			}
		case []Rule:
			ruleList = r
		default:
			continue
		}
		for _, rule := range ruleList {
			if err := v.checkRule(field, rule, data[field], data); err != nil {
				if m, ok := custom[field]; ok {
					if msg, ok := m[rule.Name]; ok && msg != "" {
						err.Message = msg
					}
				}
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// CheckValue 校验单个值。rule 为规则字符串（如 "required|min:1|max:10"）。
// msgs 可选，提供覆盖消息。
func (v *Validator) CheckValue(value any, rule string, msgs ...map[string]string) error {
	if !v.frozen {
		v.freeze()
	}
	custom := map[string]string{}
	if len(msgs) > 0 && msgs[0] != nil {
		custom = msgs[0]
	}
	var errs Errors
	for _, r := range parseTagRules(rule) {
		if err := v.checkRule("", r, value, nil); err != nil {
			if msg, ok := custom[r.Name]; ok && msg != "" {
				err.Message = msg
			}
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// CheckValueQuick 包级便捷方法，使用默认校验器校验单值。
func CheckValueQuick(value any, rule string, msgs ...map[string]string) error {
	return std.CheckValue(value, rule, msgs...)
}

// CheckMapQuick 包级便捷方法，使用默认校验器校验 map。
func CheckMapQuick(data map[string]any, rules map[string]any, msgs ...map[string]map[string]string) error {
	return std.CheckMap(data, rules, msgs...)
}
