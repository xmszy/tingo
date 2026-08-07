// Package tlang 提供零外部依赖的国际化（i18n）支持。
//
// 特性：
//   - 多语言包（map 结构），支持命名空间（"user.login" 用点分隔）；
//   - 占位符替换：支持 {name} 与位置序号 {0} 两种；
//   - 复数规则（PluralRule）：支持 Zero/One/Two/Few/Many/Other 六种 CLDR 形式，
//     内置中/英/法文规则，译文用 | 分隔复数形式；
//   - 回退语言（fallback）：缺失 key 时回退到默认语言；
//   - 上下文语言切换：TranslateCtx 从 context.Context 读取 locale；
//   - text/template 可选渲染：TranslateTpl 使用 Go 模板引擎，支持函数调用和条件；
//   - 线程安全，可在运行时热替换语言包。
//
// 基本用法：
//
//	tr := tlang.New("zh", "en")
//	tr.SetPluralRule(tlang.ChinesePluralRule)
//	tr.Add("zh", map[string]string{
//	    "messages": "{count} 条消息",              // 中文仅 Other，不用 | 
//	})
//	tr.Add("en", map[string]string{
//	    "messages": "no messages | 1 message | {count} messages", // zero|one|other
//	})
//	tr.TranslatePlural("messages", 0)   // en: "no messages"
//	tr.TranslatePlural("messages", 1)   // en: "1 message"
//	tr.TranslatePlural("messages", 5)   // en: "5 messages"
package tlang

import (
	"bytes"
	"context"
	"maps"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

// Translator 是翻译器。
type Translator struct {
	mu         sync.RWMutex
	messages   map[string]map[string]string // locale -> key -> text
	fallback   string
	locale     string
	pluralRule PluralRule
	tplCache   map[string]*template.Template // key+locale -> compiled template
}

// New 创建翻译器，locale 为默认语言，fallback 为回退语言。
func New(locale, fallback string) *Translator {
	return &Translator{
		messages: map[string]map[string]string{},
		fallback: fallback,
		locale:   locale,
		tplCache: map[string]*template.Template{},
	}
}

// Add 向某语言追加词条（可多次调用合并）。
func (t *Translator) Add(locale string, msgs map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.messages[locale] == nil {
		t.messages[locale] = map[string]string{}
	}
	maps.Copy(t.messages[locale], msgs)
	// 清空模板缓存（消息可能已变）
	t.tplCache = map[string]*template.Template{}
}

// SetLocale 切换默认语言。
func (t *Translator) SetLocale(locale string) { t.mu.Lock(); t.locale = locale; t.mu.Unlock() }

// Locale 返回当前语言。
func (t *Translator) Locale() string { t.mu.RLock(); defer t.mu.RUnlock(); return t.locale }

// SetPluralRule 设置复数规则。传 nil 禁用复数（所有 count 均为 Other）。
func (t *Translator) SetPluralRule(rule PluralRule) { t.mu.Lock(); t.pluralRule = rule; t.mu.Unlock() }

// PluralRule 返回当前复数规则。
func (t *Translator) PluralRule() PluralRule { t.mu.RLock(); defer t.mu.RUnlock(); return t.pluralRule }

// Translate 按当前语言翻译 key。
// params 可为：map[string]any（{name} 风格）或 []any（{0} 风格）。
func (t *Translator) Translate(key string, params ...any) string {
	return t.translateWithPlural(t.locale, key, 0, false, params...)
}

// TranslateFor 按指定语言翻译。
func (t *Translator) TranslateFor(locale, key string, params ...any) string {
	return t.translateWithPlural(locale, key, 0, false, params...)
}

// TranslatePlural 按当前语言翻译，count 用于选择复数形式。
// count 不会自动注入模板数据，需手动在 params 中传递，如：
//
//	tr.TranslatePlural("messages", 5, map[string]any{"count": 5})
func (t *Translator) TranslatePlural(key string, count int, params ...any) string {
	return t.translateWithPlural(t.locale, key, count, false, params...)
}

// TranslatePluralFor 按指定语言翻译复数消息。
func (t *Translator) TranslatePluralFor(locale, key string, count int, params ...any) string {
	return t.translateWithPlural(locale, key, count, false, params...)
}

// TranslateCtx 从 context.Context 读取 locale 翻译。
// 若 context 无 locale，回退到当前默认 locale。
func (t *Translator) TranslateCtx(ctx context.Context, key string, params ...any) string {
	return t.translateCtx(ctx, key, 0, false, params...)
}

// TranslatePluralCtx 从 context 读取 locale 翻译复数消息。
func (t *Translator) TranslatePluralCtx(ctx context.Context, key string, count int, params ...any) string {
	return t.translateCtx(ctx, key, count, false, params...)
}

// TranslateTpl 使用 Go text/template 引擎渲染译文。
// 模板数据从 params 提取：如果是 map[string]any 则直接作为模板数据；
// 如果是多个非 map 参数，则作为 .V0、.V1 … 注入模板。
// 模板缓存会自动编译并复用，线程安全。
func (t *Translator) TranslateTpl(key string, params ...any) string {
	return t.translateWithPlural(t.locale, key, 0, true, params...)
}

// TranslatePluralTpl 使用 Go text/template 引擎渲染复数译文。
func (t *Translator) TranslatePluralTpl(key string, count int, params ...any) string {
	return t.translateWithPlural(t.locale, key, count, true, params...)
}

// ---- context support ----

type localeCtxKey struct{}

var localeKey = localeCtxKey{}

// SetLocaleCtx 返回携带 locale 的新 context。
func SetLocaleCtx(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey, locale)
}

// LocaleFromCtx 从 context 提取 locale。
func LocaleFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(localeKey).(string)
	return v, ok
}

// ---- internal ----

func (t *Translator) translateCtx(ctx context.Context, key string, count int, tmpl bool, params ...any) string {
	locale, _ := LocaleFromCtx(ctx)
	if locale == "" {
		locale = t.locale
	}
	return t.translateWithPlural(locale, key, count, tmpl, params...)
}

func (t *Translator) translateWithPlural(locale, key string, count int, isTpl bool, params ...any) string {
	t.mu.RLock()
	rule := t.pluralRule
	msg, ok := t.messages[locale]
	t.mu.RUnlock()
	
	if !ok && locale != t.fallback {
		t.mu.RLock()
		msg, ok = t.messages[t.fallback]
		t.mu.RUnlock()
	}

	if ok {
		if raw, ok := msg[key]; ok {
			return t.render(raw, rule, count, isTpl, locale, params...)
		}
	}
	return key
}

func (t *Translator) render(raw string, rule PluralRule, count int, isTpl bool, locale string, params ...any) string {
	// 复数形式选择
	if rule != nil {
		raw = selectPluralForm(raw, rule(count))
	}

	if isTpl {
		return t.renderTemplate(raw, locale, params...)
	}
	return fill(raw, params...)
}

func (t *Translator) renderTemplate(tmpl, locale string, params ...any) string {
	cacheKey := locale + "\x00" + tmpl

	t.mu.RLock()
	tpl, ok := t.tplCache[cacheKey]
	t.mu.RUnlock()

	if !ok {
		var err error
		tpl, err = template.New("").Parse(tmpl)
		if err != nil {
			// 解析失败，回退到纯文本渲染
			return fill(tmpl, params...)
		}
		t.mu.Lock()
		// 双重检查
		if existing, ok := t.tplCache[cacheKey]; ok {
			tpl = existing
		} else {
			t.tplCache[cacheKey] = tpl
		}
		t.mu.Unlock()
	}

	data := buildTemplateData(params...)
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return fill(tmpl, params...)
	}
	return buf.String()
}

func buildTemplateData(params ...any) any {
	if len(params) == 0 {
		return nil
	}
	if len(params) == 1 {
		if m, ok := params[0].(map[string]any); ok {
			return m
		}
		return struct{ V0 any }{params[0]}
	}
	// 多个非 map 参数 → .V0, .V1, …
	m := make(map[string]any, len(params))
	for i, p := range params {
		m["V"+strconv.Itoa(i)] = p
	}
	return m
}

// fill 替换占位符。
// 若 params[0] 为 map[string]any，则替换 {key}；
// 否则按顺序替换 {0}{1} 位置占位符。
func fill(tmpl string, params ...any) string {
	if len(params) == 0 {
		return tmpl
	}
	switch p := params[0].(type) {
	case map[string]any:
		var b strings.Builder
		b.Grow(len(tmpl))
		for i := 0; i < len(tmpl); {
			if tmpl[i] == '{' {
				end := strings.IndexByte(tmpl[i:], '}')
				if end > 0 {
					name := tmpl[i+1 : i+end]
					if v, ok := p[name]; ok {
						b.WriteString(toStr(v))
						i += end + 1
						continue
					}
				}
			}
			b.WriteByte(tmpl[i])
			i++
		}
		return b.String()
	default:
		// 按顺序替换 {0},{1},… 位置占位符
		var b strings.Builder
		b.Grow(len(tmpl))
		for i := 0; i < len(tmpl); {
			if tmpl[i] == '{' {
				end := strings.IndexByte(tmpl[i:], '}')
				if end > 0 {
					idx := 0
					ok := true
					for k := i + 1; k < i+end; k++ {
						c := tmpl[k]
						if c < '0' || c > '9' {
							ok = false
							break
						}
						idx = idx*10 + int(c-'0')
					}
					if ok && idx < len(params) {
						b.WriteString(toStr(params[idx]))
						i += end + 1
						continue
					}
				}
			}
			b.WriteByte(tmpl[i])
			i++
		}
		return b.String()
	}
}

func toStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		return ""
	}
}
