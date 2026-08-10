package tvalid

// ──────────────── 二维错误查询扩展 ────────────────
//
// 在现有 Error/Errors 一维模型之上，提供「字段 → 规则 → 消息」的二维查询能力，
// 便于 API 层按字段聚合错误信息。不破坏原有 First/All/HasError 等 API。

// Maps 返回「字段 → {规则 → 消息}」二维结构。
func (e Errors) Maps() map[string]map[string]string {
	m := make(map[string]map[string]string)
	for _, err := range e {
		if err == nil {
			continue
		}
		if _, ok := m[err.Field]; !ok {
			m[err.Field] = make(map[string]string)
		}
		m[err.Field][err.Rule] = err.Message
	}
	return m
}

// Items 为 Maps 的别名，语义更直观。
func (e Errors) Items() map[string]map[string]string { return e.Maps() }

// Map 返回「字段 → 该字段第一条错误信息」的一维结构（等价于 All）。
func (e Errors) Map() map[string]string {
	m := make(map[string]string, len(e))
	for _, err := range e {
		if err == nil {
			continue
		}
		if _, ok := m[err.Field]; !ok {
			m[err.Field] = err.Message
		}
	}
	return m
}

// FirstRule 返回第一个错误的规则名。
func (e Errors) FirstRule() string {
	if f := e.First(); f != nil {
		return f.Rule
	}
	return ""
}

// FirstError 返回第一个错误的消息文本。
func (e Errors) FirstError() string {
	if f := e.First(); f != nil {
		return f.Message
	}
	return ""
}

// FirstItem 返回 (字段名, 消息) 的第一个错误元组。
func (e Errors) FirstItem() (string, string) {
	if f := e.First(); f != nil {
		return f.Field, f.Message
	}
	return "", ""
}
