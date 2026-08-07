package tlang

import (
	"context"
	"testing"
)

func TestTranslateBasic(t *testing.T) {
	tr := New("zh", "en")
	tr.Add("zh", map[string]string{"hello": "你好 {name}", "bye": "再见"})
	tr.Add("en", map[string]string{"hello": "Hello {name}"})
	got := tr.Translate("hello", map[string]any{"name": "tingo"})
	if got != "你好 tingo" {
		t.Fatalf("got %q", got)
	}
}

func TestFallback(t *testing.T) {
	tr := New("fr", "en")
	tr.Add("en", map[string]string{"only": "English only"})
	got := tr.Translate("only")
	if got != "English only" {
		t.Fatalf("got %q", got)
	}
	// 缺失 key 回退也没有 -> 返回 key
	tr.SetLocale("fr")
	got = tr.Translate("missing")
	if got != "missing" {
		t.Fatalf("got %q", got)
	}
}

func TestPositional(t *testing.T) {
	tr := New("en", "en")
	tr.Add("en", map[string]string{"fmt": "name={0} age={1}"})
	got := tr.Translate("fmt", "tom", 18)
	if got != "name=tom age=18" {
		t.Fatalf("got %q", got)
	}
}

// ---- Plural ----

func TestEnglishPluralRule(t *testing.T) {
	if EnglishPluralRule(0) != PluralOther {
		t.Fatal("0 should be other")
	}
	if EnglishPluralRule(1) != PluralOne {
		t.Fatal("1 should be one")
	}
	if EnglishPluralRule(2) != PluralOther {
		t.Fatal("2 should be other")
	}
}

func TestChinesePluralRule(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 100} {
		if ChinesePluralRule(n) != PluralOther {
			t.Fatalf("%d should be other for Chinese", n)
		}
	}
}

func TestSelectPluralForm(t *testing.T) {
	tests := []struct {
		msg  string
		form PluralForm
		want string
	}{
		// 含 | 的复数消息
		{"zero|one|two|few|many|other", PluralZero, "zero"},
		{"zero|one|two|few|many|other", PluralOne, "one"},
		{"zero|one|two|few|many|other", PluralTwo, "two"},
		{"zero|one|two|few|many|other", PluralOther, "other"},
		// 部分缺失 → 回退到 other
		{"zero|one|other", PluralTwo, "other"},
		{"zero|one|other", PluralFew, "other"},
		// 单元素 → 不含 |，原样返回
		{"hello world", PluralZero, "hello world"},
		// other 为空 → 最后一个非空
		{"zero|one||", PluralOther, "one"},
		// 空字符串
		{"||other", PluralZero, "other"},
	}
	for _, tt := range tests {
		got := selectPluralForm(tt.msg, tt.form)
		if got != tt.want {
			t.Errorf("selectPluralForm(%q, %q) = %q, want %q", tt.msg, tt.form, got, tt.want)
		}
	}
}

func TestTranslatePlural(t *testing.T) {
	tr := New("en", "en")
	tr.SetPluralRule(EnglishPluralRule)
	// CLDR: English one=i=1&v=0, other=everything else (包括 0)
	// zero 位置仅供需要特殊 0 形式的语言使用（如阿拉伯语）
	tr.Add("en", map[string]string{
		"messages": " | 1 message | {count} messages", // zero空|one|other
	})

	// 0 → other（英文 0 属 other）
	if got := tr.TranslatePlural("messages", 0, map[string]any{"count": 0}); got != "0 messages" {
		t.Fatalf("plural 0: got %q", got)
	}
	// 1 → one
	if got := tr.TranslatePlural("messages", 1, map[string]any{"count": 1}); got != "1 message" {
		t.Fatalf("plural 1: got %q", got)
	}
	// 5 → other
	if got := tr.TranslatePlural("messages", 5, map[string]any{"count": 5}); got != "5 messages" {
		t.Fatalf("plural 5: got %q", got)
	}
}

func TestTranslatePluralChinese(t *testing.T) {
	tr := New("zh", "zh")
	tr.SetPluralRule(ChinesePluralRule)
	// 中文只用一个 other 形式
	tr.Add("zh", map[string]string{
		"messages": "{count} 条消息",
	})

	for _, n := range []int{0, 1, 5, 100} {
		got := tr.TranslatePlural("messages", n, map[string]any{"count": n})
		want := toStr(n) + " 条消息"
		if got != want {
			t.Fatalf("plural %d: got %q, want %q", n, got, want)
		}
	}
}

func TestTranslatePluralNoRule(t *testing.T) {
	// 不设 PluralRule → 不触发复数选择 → 管道消息原样传给 fill
	tr := New("en", "en")
	tr.Add("en", map[string]string{
		"messages": "no messages|1 message|{count} messages",
	})

	got := tr.TranslatePlural("messages", 1, map[string]any{"count": 1})
	// 无规则时 {count} 被替换，但 | 分隔仍保留
	if got != "no messages|1 message|1 messages" {
		t.Fatalf("no rule should keep pipe: got %q", got)
	}
}

func TestTranslatePluralFor(t *testing.T) {
	tr := New("zh", "en")
	tr.SetPluralRule(EnglishPluralRule)
	tr.Add("en", map[string]string{
		"messages": "no messages | 1 message | {count} messages",
	})
	tr.Add("zh", map[string]string{
		"messages": "{count} 条消息",
	})

	got := tr.TranslatePluralFor("en", "messages", 1, map[string]any{"count": 1})
	if got != "1 message" {
		t.Fatalf("got %q", got)
	}
}

// ---- Context ----

func TestTranslateCtx(t *testing.T) {
	tr := New("zh", "en")
	tr.Add("en", map[string]string{"hello": "Hello {name}"})
	tr.Add("zh", map[string]string{"hello": "你好 {name}"})

	ctx := SetLocaleCtx(context.Background(), "en")
	got := tr.TranslateCtx(ctx, "hello", map[string]any{"name": "tingo"})
	if got != "Hello tingo" {
		t.Fatalf("got %q", got)
	}
}

func TestTranslateCtxFallback(t *testing.T) {
	tr := New("zh", "en")
	tr.Add("zh", map[string]string{"hello": "你好 {name}"})

	// context 无 locale → 回退到默认 locale
	ctx := context.Background()
	got := tr.TranslateCtx(ctx, "hello", map[string]any{"name": "tingo"})
	if got != "你好 tingo" {
		t.Fatalf("got %q", got)
	}
}

func TestTranslatePluralCtx(t *testing.T) {
	tr := New("zh", "en")
	tr.SetPluralRule(EnglishPluralRule)
	tr.Add("en", map[string]string{
		"messages": "no messages | 1 message | {count} messages",
	})

	ctx := SetLocaleCtx(context.Background(), "en")
	got := tr.TranslatePluralCtx(ctx, "messages", 1, map[string]any{"count": 1})
	if got != "1 message" {
		t.Fatalf("got %q", got)
	}
}

func TestLocaleFromCtxEmpty(t *testing.T) {
	_, ok := LocaleFromCtx(context.Background())
	if ok {
		t.Fatal("should be false for empty context")
	}
}

// ---- Template ----

func TestTranslateTpl(t *testing.T) {
	tr := New("en", "en")
	tr.Add("en", map[string]string{
		"greet": "Hello {{.Name}}!",
	})

	got := tr.TranslateTpl("greet", map[string]any{"Name": "tingo"})
	if got != "Hello tingo!" {
		t.Fatalf("got %q", got)
	}
}

func TestTranslateTplPlural(t *testing.T) {
	tr := New("en", "en")
	tr.SetPluralRule(EnglishPluralRule)
	// CLDR English: 0→other, 1→one
	tr.Add("en", map[string]string{
		"items": " | 1 item | {{.Count}} items", // zero空|one|other
	})

	got := tr.TranslatePluralTpl("items", 0, map[string]any{"Count": 0})
	if got != "0 items" {
		t.Fatalf("count=0: got %q", got)
	}

	got = tr.TranslatePluralTpl("items", 5, map[string]any{"Count": 5})
	if got != "5 items" {
		t.Fatalf("count=5: got %q", got)
	}
}

func TestTranslateTplCache(t *testing.T) {
	tr := New("en", "en")
	tr.Add("en", map[string]string{
		"greet": "Hello {{.Name}}!",
	})

	// 多次调用应命中缓存
	for i := 0; i < 10; i++ {
		got := tr.TranslateTpl("greet", map[string]any{"Name": "tingo"})
		if got != "Hello tingo!" {
			t.Fatalf("got %q", got)
		}
	}
}

func TestTranslateTplFallback(t *testing.T) {
	tr := New("en", "en")
	tr.Add("en", map[string]string{
		"broken": "Hello {{.Name}!", // 无效模板
	})

	// 无效模板 → 回退到 fill 渲染
	got := tr.TranslateTpl("broken", map[string]any{"Name": "tingo"})
	// 回退后按 {key} 替换
	if got == "" {
		t.Fatal("fallback should produce output")
	}
}
