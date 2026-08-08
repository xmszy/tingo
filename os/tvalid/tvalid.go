// Package tvalid 提供数据校验框架。
//
// 设计要点：
//   - 零外部依赖，纯标准库实现。
//   - 基于 struct tag 声明校验规则（valid tag）。
//   - 支持链式校验器和自定义规则注册。
//   - 错误类型独立，可精确定位验证失败字段。
//
// Tag 格式：
//
//	type User struct {
//	    Name string `valid:"rule1:arg1|rule2:arg2" label:"用户名"`
//	    Age  int    `valid:"min:1|max:120"`
//	}
//
// 内置规则：required/ip/email/url/len/len-min/len-max/regex/in/not-in/
// numeric/alpha-num/between/min/max/eq/phone/date/json/uuid
//
// 用法：
//
//	rule := tvalid.New().Rules(tvalid.Map{
//	    "name": "required|len:3,20",
//	    "age":  "required|between:1,150",
//	})
//	err := rule.Check(ctx, data)
//	err := rule.CheckStruct(user)
package tvalid

import (
	"maps"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// ──────────────── 默认实例 ────────────────

var defaultValidator = New()

// ──────────────── 门面函数 ────────────────

// Check 使用默认校验器校验 data map（ruleSpec 是 map[string]string 格式）。
func Check(data map[string]any, ruleSpec RuleSpec) error {
	return defaultValidator.Check(data, ruleSpec)
}

// CheckStruct 使用默认校验器校验结构体。
func CheckStruct(v any) error {
	return defaultValidator.CheckStruct(v)
}

// ──────────────── 错误 ────────────────

// Error 校验错误。
type Error struct {
	Field   string `json:"field"`   // 字段名
	Rule    string `json:"rule"`    // 触碰的规则
	Message string `json:"message"` // 错误信息
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("%s validation failed: %s", e.Field, e.Rule)
}

// Errors 校验错误集合。
type Errors []*Error

func (e Errors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var b strings.Builder
	for i, err := range e {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

// First 返回第一个错误。
func (e Errors) First() *Error {
	if len(e) == 0 {
		return nil
	}
	return e[0]
}

// All 返回所有错误的 message map。
func (e Errors) All() map[string]string {
	m := make(map[string]string, len(e))
	for _, err := range e {
		m[err.Field] = err.Error()
	}
	return m
}

// HasError 是否有错误。
func (e Errors) HasError() bool { return len(e) > 0 }

// ──────────────── 规则 ────────────────

// Rule 一条校验规则。
type Rule struct {
	Name  string   // 规则名
	Args  []string // 参数列表
	Param string   // 原始参数字符串
}

// RuleSpec map[string]string 形式规则，如 {"name": "required|len:3,20"}。
type RuleSpec map[string]string

// Rules 结构体 tag 解析后的规则 map：字段名 → []Rule。
type Rules map[string][]Rule

// ──────────────── Validator ────────────────

// Validator 校验器。
type Validator struct {
	mu    sync.RWMutex
	rules map[string]RuleFunc
	msgs  map[string]string // 规则 → 默认错误提示模板

	// freeze 后保存只读快照，热路径读取不再加锁。
	frozen      bool
	frozenRules map[string]RuleFunc
	frozenMsgs  map[string]string
}

// freeze 在首次校验前把规则表冻结为只读快照；之后 Register 会重新冻结。
func (v *Validator) freeze() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.frozen {
		return
	}
	nr := make(map[string]RuleFunc, len(v.rules))
	maps.Copy(nr, v.rules)
	nm := make(map[string]string, len(v.msgs))
	maps.Copy(nm, v.msgs)
	v.frozenRules = nr
	v.frozenMsgs = nm
	v.frozen = true
}

// RuleFunc 规则函数：值→错误信息（空=nil 表示通过）。
type RuleFunc func(value any, args []string) error

// New 创建校验器。
func New() *Validator {
	v := &Validator{
		rules: make(map[string]RuleFunc),
		msgs:  make(map[string]string),
	}
	v.registerBuiltins()
	return v
}

// Register 注册自定义校验规则。
func (v *Validator) Register(name string, fn RuleFunc, defaultMsg string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules[name] = fn
	if defaultMsg != "" {
		v.msgs[name] = defaultMsg
	}
	v.frozen = false // 规则变更，下次校验重新冻结
}

// RegisterMsg 为内置规则注册自定义错误提示。
func (v *Validator) RegisterMsg(name, msg string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.msgs[name] = msg
	v.frozen = false
}

// ──────────────── 校验入口 ────────────────

// Check 校验 data map（key 对应对应 ruleSpec 中的 key）。
func (v *Validator) Check(data map[string]any, ruleSpec RuleSpec) error {
	if !v.frozen {
		v.freeze()
	}
	rules := parseRules(ruleSpec)
	var errs Errors
	for field, value := range data {
		fieldRules, ok := rules[field]
		if !ok {
			continue
		}
		for _, rule := range fieldRules {
			if err := v.checkRule(field, rule, value); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ──────────────── 结构体解析缓存 ────────────────
//
// 同一结构体类型的 tag 解析结果（字段规则、label、confirm 信息、form→字段索引）
// 在每次请求中是完全相同的。这里按 reflect.Type 缓存，避免每请求重复反射与建 map。
// 缓存只保存「类型信息」，不含任何 reflect.Value，因此天然并发安全。

type fieldMeta struct {
	index int      // 字段在结构体中的索引
	name  string   // 原始字段名
	label string   // label tag（为空则等于 name）
	rules []Rule    // 该字段的校验规则（已解析）
}

type structMeta struct {
	fields   []fieldMeta
	confirms []confirmInfo
	formIdx  map[string]int // form tag / snake_case → 字段索引，用于 confirm 查找

	// 标志位：类型是否包含任何需要运行的校验。
	// 两者皆否时 CheckStruct 可直接短路返回 nil（零开销 fast-path）。
	hasRule    bool
	hasConfirm bool
}

var structCache sync.Map // reflect.Type → *structMeta

func getStructMeta(rt reflect.Type) *structMeta {
	if m, ok := structCache.Load(rt); ok {
		return m.(*structMeta)
	}
	m := buildStructMeta(rt)
	structCache.Store(rt, m)
	return m
}

// buildStructMeta 一次性解析结构体类型（递归处理嵌套结构体）。
func buildStructMeta(rt reflect.Type) *structMeta {
	m := &structMeta{
		formIdx: make(map[string]int),
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		formTag := f.Tag.Get("form")
		if formTag != "" {
			m.formIdx[formTag] = i
		}
		m.formIdx[snakeCase(f.Name)] = i

		tag := f.Tag.Get("valid")
		if tag == "-" {
			continue
		}
		fm := fieldMeta{index: i, name: f.Name, label: f.Name}
		if label := f.Tag.Get("label"); label != "" {
			fm.label = label
		}
		if tag != "" {
			fm.rules = parseTagRules(tag)
		}
		if f.Type.Kind() == reflect.Struct && !isBasicType(f.Type) {
			nested := buildStructMeta(f.Type)
			m.hasRule = m.hasRule || nested.hasRule
			m.hasConfirm = m.hasConfirm || nested.hasConfirm
			for _, nf := range nested.fields {
				cf := nf
				cf.name = f.Name + "." + nf.name
				cf.label = f.Name + "." + nf.label
				m.fields = append(m.fields, cf)
			}
			m.confirms = append(m.confirms, nested.confirms...)
		} else {
			if len(fm.rules) > 0 {
				m.hasRule = true
			}
			m.fields = append(m.fields, fm)
		}

		// 提取 confirm 规则（目标字段为本字段）
		for _, r := range fm.rules {
			if r.Name != "confirm" {
				continue
			}
			m.hasConfirm = true
			confirmField := formTag
			if confirmField == "" {
				confirmField = snakeCase(f.Name)
			}
			confirmField = confirmField + "_confirmation"
			if len(r.Args) > 0 && r.Args[0] != "" {
				confirmField = r.Args[0]
			}
			m.confirms = append(m.confirms, confirmInfo{
				confirmField: confirmField,
				targetField:  f.Name,
				ruleName:     r.Name,
			})
		}
	}
	return m
}

// CheckStruct 校验结构体（基于 valid tag）。
// 自动处理 confirm 规则（值相等性校验）。
func (v *Validator) CheckStruct(obj any) error {
	if !v.frozen {
		v.freeze()
	}
	rv := reflect.ValueOf(obj)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("tvalid: CheckStruct requires struct, got %T", obj)
	}
	rt := rv.Type()
	meta := getStructMeta(rt)

	// fast-path：结构体既无校验规则也无 confirm 时直接返回，零开销。
	if !meta.hasRule && !meta.hasConfirm {
		return nil
	}

	var errs Errors
	for _, fm := range meta.fields {
		if len(fm.rules) == 0 {
			continue
		}
		value := rv.Field(fm.index).Interface()
		for _, rule := range fm.rules {
			if rule.Name == "confirm" {
				continue // confirm 统一处理
			}
			if err := v.checkRule(fm.label, rule, value); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// 统一验证 confirm 规则
	for _, ci := range meta.confirms {
		idx, ok := meta.formIdx[ci.confirmField]
		if !ok {
			errs = append(errs, &Error{
				Field:   ci.targetField,
				Rule:    "confirm",
				Message: fmt.Sprintf("%s 确认字段 %s 不存在", ci.targetField, ci.confirmField),
			})
			continue
		}
		targetVal := rv.FieldByName(ci.targetField)
		confirmVal := rv.Field(idx)
		if !equalValue(targetVal.Interface(), confirmVal.Interface()) {
			errs = append(errs, &Error{
				Field:   ci.targetField,
				Rule:    "confirm",
				Message: fmt.Sprintf("%s 与确认值不一致", ci.targetField),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// equalValue 比较两个值是否相等（支持基础类型）。
func equalValue(a, b any) bool {
	return toString(a) == toString(b)
}

func (v *Validator) checkRule(field string, rule Rule, value any) *Error {
	fn, ok := v.frozenRules[rule.Name]
	if !ok {
		return &Error{Field: field, Rule: rule.Name, Message: "unknown rule: " + rule.Name}
	}
	if err := fn(value, rule.Args); err != nil {
		msg := err.Error()
		if tpl, ex := v.frozenMsgs[rule.Name]; ex {
			msg = tpl
			// 简单模板替换
			msg = strings.ReplaceAll(msg, "{field}", field)
			msg = strings.ReplaceAll(msg, "{rule}", rule.Name)
			msg = strings.ReplaceAll(msg, "{args}", strings.Join(rule.Args, ","))
		}
		return &Error{Field: field, Rule: rule.Name, Message: msg}
	}
	return nil
}

// ──────────────── 内置规则 ────────────────

func (v *Validator) registerBuiltins() {
	reg := func(name, msg string, fn RuleFunc) { v.Register(name, fn, msg) }
	reg("required", "{field} 不能为空", ruleRequired)
	reg("ip", "{field} 格式不正确", ruleIP)
	reg("email", "{field} 邮箱格式不正确", ruleEmail)
	reg("url", "{field} URL 格式不正确", ruleURL)
	reg("len", "{field} 长度必须为 {args}", ruleLen)
	reg("len-min", "{field} 长度不能小于 {args}", ruleLenMin)
	reg("len-max", "{field} 长度不能大于 {args}", ruleLenMax)
	reg("regex", "{field} 格式不匹配", ruleRegex)
	reg("in", "{field} 值不在允许范围内", ruleIn)
	reg("not-in", "{field} 值不允许", ruleNotIn)
	reg("numeric", "{field} 必须为数字", ruleNumeric)
	reg("alpha-num", "{field} 只能为字母和数字", ruleAlphaNum)
	reg("between", "{field} 必须在 {args} 之间", ruleBetween)
	reg("min", "{field} 不能小于 {args}", ruleMin)
	reg("max", "{field} 不能大于 {args}", ruleMax)
	reg("eq", "{field} 值不匹配", ruleEq)
	reg("phone", "{field} 手机号格式不正确", rulePhone)
	reg("date", "{field} 日期格式不正确", ruleDate)
	reg("boolean", "{field} 必须为布尔值", ruleBool)
	reg("json", "{field} 不是合法 JSON", ruleJSON)
	reg("uuid", "{field} UUID 格式不正确", ruleUUID)

	// ──────── 新增规则 ────────
	reg("enum", "{field} 值不在允许的枚举范围内", ruleEnum)
	reg("alpha", "{field} 只能为字母", ruleAlpha)
	reg("alpha-dash", "{field} 只能为字母、数字、短划线和下划线", ruleAlphaDash)
	reg("chinese", "{field} 只能为中文", ruleChinese)
	reg("file-size", "{field} 文件大小超出限制", ruleFileSize)
	reg("file-ext", "{field} 文件扩展名不允许", ruleFileExt)
}

func ruleRequired(value any, _ []string) error {
	if value == nil {
		return fmt.Errorf("required")
	}
	switch val := value.(type) {
	case string:
		if val == "" {
			return fmt.Errorf("required")
		}
	case []byte:
		if len(val) == 0 {
			return fmt.Errorf("required")
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			if rv.Len() == 0 {
				return fmt.Errorf("required")
			}
		case reflect.Ptr, reflect.Interface:
			if rv.IsNil() {
				return fmt.Errorf("required")
			}
		}
	}
	return nil
}

func ruleIP(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if ip := net.ParseIP(s); ip == nil {
		return fmt.Errorf("invalid ip")
	}
	return nil
}

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ruleEmail(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !emailRe.MatchString(s) {
		return fmt.Errorf("invalid email")
	}
	return nil
}

func ruleURL(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if _, err := url.ParseRequestURI(s); err != nil {
		return fmt.Errorf("invalid url")
	}
	return nil
}

func ruleLen(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	l := stringLen(value)
	n, _ := strconv.Atoi(args[0])
	if l != n {
		return fmt.Errorf("len != %d", n)
	}
	return nil
}

func ruleLenMin(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	l := stringLen(value)
	n, _ := strconv.Atoi(args[0])
	if l < n {
		return fmt.Errorf("len < %d", n)
	}
	return nil
}

func ruleLenMax(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	l := stringLen(value)
	n, _ := strconv.Atoi(args[0])
	if l > n {
		return fmt.Errorf("len > %d", n)
	}
	return nil
}

func ruleRegex(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	s := toString(value)
	if s == "" {
		return nil
	}
	re, err := regexp.Compile(args[0])
	if err != nil {
		return fmt.Errorf("invalid regex: %v", err)
	}
	if !re.MatchString(s) {
		return fmt.Errorf("regex mismatch")
	}
	return nil
}

func ruleIn(value any, args []string) error {
	s := toString(value)
	for _, a := range args {
		if a == s {
			return nil
		}
	}
	return fmt.Errorf("value not in %v", args)
}

func ruleNotIn(value any, args []string) error {
	s := toString(value)
	for _, a := range args {
		if a == s {
			return fmt.Errorf("value in blacklist")
		}
	}
	return nil
}

var numericRe = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

func ruleNumeric(value any, args []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !numericRe.MatchString(s) {
		return fmt.Errorf("not numeric")
	}
	return nil
}

var alphaNumRe = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func ruleAlphaNum(value any, args []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !alphaNumRe.MatchString(s) {
		return fmt.Errorf("not alphanumeric")
	}
	return nil
}

func ruleBetween(value any, args []string) error {
	if len(args) < 2 {
		return nil
	}
	n := toFloat(value)
	lo, _ := strconv.ParseFloat(args[0], 64)
	hi, _ := strconv.ParseFloat(args[1], 64)
	if n < lo || n > hi {
		return fmt.Errorf("not between %v and %v", lo, hi)
	}
	return nil
}

func ruleMin(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	n := toFloat(value)
	lo, _ := strconv.ParseFloat(args[0], 64)
	if n < lo {
		return fmt.Errorf("min %v", lo)
	}
	return nil
}

func ruleMax(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	n := toFloat(value)
	hi, _ := strconv.ParseFloat(args[0], 64)
	if n > hi {
		return fmt.Errorf("max %v", hi)
	}
	return nil
}

func ruleEq(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if toString(value) != args[0] {
		return fmt.Errorf("not eq %v", args[0])
	}
	return nil
}

var phoneRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

func rulePhone(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !phoneRe.MatchString(s) {
		return fmt.Errorf("invalid phone")
	}
	return nil
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}( \d{2}:\d{2}:\d{2})?$`)

func ruleDate(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !dateRe.MatchString(s) {
		return fmt.Errorf("invalid date")
	}
	return nil
}

func ruleBool(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	switch strings.ToLower(s) {
	case "true", "false", "1", "0":
		return nil
	default:
		return fmt.Errorf("not boolean")
	}
}

func ruleJSON(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !json.Valid([]byte(s)) {
		return fmt.Errorf("invalid json")
	}
	return nil
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ruleUUID(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !uuidRe.MatchString(s) {
		return fmt.Errorf("invalid uuid")
	}
	return nil
}

// ──────────────── 规则解析 ────────────────

// parseRules 从 RuleSpec 解析 Rules。
func parseRules(spec RuleSpec) Rules {
	r := make(Rules, len(spec))
	for field, ruleStr := range spec {
		r[field] = parseTagRules(ruleStr)
	}
	return r
}

// parseTagRules 从 valid tag 字符串解析 []Rule。
func parseTagRules(tag string) []Rule {
	if tag == "" {
		return nil
	}
	parts := strings.Split(tag, "|")
	rules := make([]Rule, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		r := Rule{}
		// 分隔规则名和参数：required, len:3,20
		if idx := strings.IndexByte(part, ':'); idx >= 0 {
			r.Name = strings.TrimSpace(part[:idx])
			r.Param = part[idx+1:]
			r.Args = splitArgs(r.Param)
		} else {
			r.Name = part
		}
		rules = append(rules, r)
	}
	return rules
}

// splitArgs 分割参数：1,20 → ["1","20"]。
func splitArgs(s string) []string {
	return strings.Split(s, ",")
}

func isBasicType(t reflect.Type) bool {
	k := t.Kind()
	return k >= reflect.Bool && k <= reflect.Complex128 || k == reflect.String
}

// ──────────────── 新增规则实现 ────────────────

func ruleEnum(value any, args []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	for _, a := range args {
		if a == s {
			return nil
		}
	}
	return fmt.Errorf("value not in enum")
}

func ruleAlpha(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !IsAlpha(s) {
		return fmt.Errorf("not alpha")
	}
	return nil
}

var alphaDashRe = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)

func ruleAlphaDash(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !alphaDashRe.MatchString(s) {
		return fmt.Errorf("not alpha-dash")
	}
	return nil
}

var chineseRe = regexp.MustCompile(`^[\p{Han}]+$`)

func ruleChinese(value any, _ []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	if !chineseRe.MatchString(s) {
		return fmt.Errorf("not chinese")
	}
	return nil
}

// fileSizeArg 解析文件大小参数，如 "1MB" → 1048576。
// 支持 B, KB, MB, GB 后缀。
func parseFileSize(sizeStr string) int64 {
	s := strings.TrimSpace(strings.ToUpper(sizeStr))
	multiplier := int64(1)
	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n * multiplier
}

func ruleFileSize(value any, args []string) error {
	if len(args) == 0 {
		return nil
	}
	maxSize := parseFileSize(args[0])
	var fileSize int64

	switch v := value.(type) {
	case int:
		fileSize = int64(v)
	case int64:
		fileSize = v
	case string:
		// 尝试解析文件路径获取实际大小
		// 这里传的是文件大小数值，按数值处理
		v2, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil // 非数字字符串跳过
		}
		fileSize = v2
	case float64:
		fileSize = int64(v)
	default:
		return nil // 无法判断大小的类型跳过
	}

	if fileSize > maxSize {
		return fmt.Errorf("file size exceeds %d bytes", maxSize)
	}
	return nil
}

// 常见允许的文件扩展名
var allowedExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "bmp": true, "webp": true, "svg": true,
	"doc": true, "docx": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true,
	"pdf": true, "txt": true, "csv": true, "json": true, "xml": true,
	"zip": true, "rar": true, "7z": true, "tar": true, "gz": true,
	"mp4": true, "avi": true, "mov": true, "mkv": true,
	"mp3": true, "wav": true, "flac": true,
}

func ruleFileExt(value any, args []string) error {
	s := toString(value)
	if s == "" {
		return nil
	}
	ext := strings.ToLower(s)
	// 去掉 .
	ext = strings.TrimPrefix(ext, ".")

	if len(args) == 0 {
		// 无参数：检查是否在常见允许列表中
		if allowedExts[ext] {
			return nil
		}
		return fmt.Errorf("file extension not in allowed list")
	}
	// 有参数：检查是否在指定的扩展名列表中
	for _, a := range args {
		if strings.ToLower(strings.TrimSpace(a)) == ext {
			return nil
		}
	}
	return fmt.Errorf("file extension not allowed: %s", ext)
}

// ──────────────── 确认字段（confirm rule） ────────────────
// confirm 规则需要在 CheckStruct 中特殊处理，因为它需要访问其他字段。
// 支持两种 form tag 约定：
//   1. password 字段上有 confirm 规则 → 自动查找 password_confirm 字段
//   2. password 字段上 confirm:repassword → 查找 repassword 字段

// confirmField 存储确认规则信息。
type confirmInfo struct {
	confirmField string // 确认字段名（如 "password_confirm"）
	targetField  string // 被确认的目标字段名（如 "Password"）
	ruleName     string // 原始规则名（confirm）
}

// snakeCase 将驼峰转为 snake_case。
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}


func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

func stringLen(value any) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	default:
		return len(toString(value))
	}
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		f, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f
	}
}

// ──────────────── 类型约束 ────────────────

// IsNumber 判断是否为数字类型。
func IsNumeric(s string) bool { return numericRe.MatchString(s) }

// IsEmail 判断是否为邮箱。
func IsEmail(s string) bool { return emailRe.MatchString(s) }

// IsPhone 判断是否为手机号。
func IsPhone(s string) bool { return phoneRe.MatchString(s) }

// IsURL 判断是否为 URL。
func IsURL(s string) bool {
	_, err := url.ParseRequestURI(s)
	return err == nil
}

// IsIP 判断是否为 IP。
func IsIP(s string) bool { return net.ParseIP(s) != nil }

// IsAlpha 判断是否全为字母。
func IsAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return len(s) > 0
}

// IsAlphaNum 判断是否字母+数字。
func IsAlphaNum(s string) bool { return alphaNumRe.MatchString(s) }
