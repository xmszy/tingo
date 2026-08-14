package tapp

import (
	"testing"

	"github.com/xmszy/tingo/os/tvalid"
)

// TestDefaultValidatorCheckStruct 验证默认校验器按 struct tag 校验。
func TestDefaultValidatorCheckStruct(t *testing.T) {
	v := DefaultValidator()
	if v == nil {
		t.Fatal("default validator is nil")
	}
	r := struct {
		Name string `valid:"required"`
	}{Name: "ok"}
	if err := v.CheckStruct(&r); err != nil {
		t.Fatalf("non-empty required field should pass: %v", err)
	}
	empty := struct {
		Name string `valid:"required"`
	}{Name: ""}
	if err := v.CheckStruct(&empty); err == nil {
		t.Fatal("empty required field should fail")
	}
}

// TestValidatorValidateWithRuleSpec 验证 Map 规则表校验走 tvalid.RuleSpec。
func TestValidatorValidateWithRuleSpec(t *testing.T) {
	v := DefaultValidator()
	err := v.Validate(map[string]any{"name": ""}, tvalid.RuleSpec{"name": "required"})
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}
	err = v.Validate(map[string]any{"name": "ok"}, tvalid.RuleSpec{"name": "required"})
	if err != nil {
		t.Fatalf("valid data should pass: %v", err)
	}
}

// TestSetDefaultValidator 验证可替换默认校验器。
func TestSetDefaultValidator(t *testing.T) {
	orig := DefaultValidator()
	defer SetDefaultValidator(orig)

	mv := &mockValidator{}
	SetDefaultValidator(mv)
	if DefaultValidator() != mv {
		t.Fatal("SetDefaultValidator did not take effect")
	}
	// nil 应回退到内置实现。
	SetDefaultValidator(nil)
	if _, ok := DefaultValidator().(*tvalidValidator); !ok {
		t.Fatalf("nil should fall back to tvalidValidator, got %T", DefaultValidator())
	}
}

// TestNoopValidator 验证 noopValidator 恒放行。
func TestNoopValidator(t *testing.T) {
	var nv noopValidator
	if err := nv.Validate(map[string]any{}, tvalid.RuleSpec{"x": "required"}); err != nil {
		t.Fatalf("noop Validate should pass: %v", err)
	}
	if err := nv.CheckStruct(struct{}{}); err != nil {
		t.Fatalf("noop CheckStruct should pass: %v", err)
	}
}

// TestValidatorFunc 验证函数适配器满足接口并透传。
func TestValidatorFunc(t *testing.T) {
	called := false
	f := ValidatorFunc(func(data any, rules tvalid.RuleSpec) error {
		called = true
		return nil
	})
	if err := f.Validate(map[string]any{}, tvalid.RuleSpec{}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("ValidatorFunc.Validate not called")
	}
}
