package validate

import (
	"strconv"
	"testing"
)

func TestRequireAndMax(t *testing.T) {
	data := map[string]any{"name": "ab", "email": "bad"}
	rules := map[string]string{
		"name":  "require|max:5",
		"email": "require|email",
	}
	if err := Check(data, rules); err == nil {
		t.Fatal("expected error")
	} else if err.Error() == "" {
		t.Fatal("empty error")
	}
	batch := CheckBatch(data, rules)
	if e, ok := batch.(Errors); !ok || len(e) == 0 {
		t.Fatal("batch should collect errors")
	}
}

func TestPass(t *testing.T) {
	data := map[string]any{"name": "alice", "email": "a@b.com"}
	rules := map[string]string{"name": "require|max:10", "email": "require|email"}
	if err := Check(data, rules); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInAndBetween(t *testing.T) {
	data := map[string]any{"role": "2", "age": "30"}
	rules := map[string]string{"role": "in:1,2,3", "age": "between:18,60"}
	if err := Check(data, rules); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestConfirm(t *testing.T) {
	data := map[string]any{"password": "123", "password_confirm": "456"}
	rules := map[string]string{"password": "require|confirm"}
	if err := Check(data, rules); err == nil {
		t.Fatal("confirm should fail")
	}
}

type loginForm struct {
	Name     string `json:"name" validate:"require|max:25"`
	Email    string `json:"email" validate:"require|email"`
	Password string `json:"password" validate:"require|min:6"`
}

func TestStructTag(t *testing.T) {
	f := loginForm{Name: "", Email: "x", Password: "123"}
	if err := Struct(f); err == nil {
		t.Fatal("expected struct error")
	}
	f2 := loginForm{Name: "alice", Email: "a@b.com", Password: "secret"}
	if err := Struct(f2); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCustomMsg(t *testing.T) {
	data := map[string]any{"name": ""}
	rules := map[string]string{"name": "require"}
	msgs := map[string]string{"name.require": "请填写姓名"}
	err := Check(data, rules, msgs)
	if err == nil || err.Error() != "请填写姓名" {
		t.Fatalf("custom msg failed: %v", err)
	}
}

func TestCustomRule(t *testing.T) {
	AddRule("even", func(val, _ string) bool {
		n, err := strconv.Atoi(val)
		return err == nil && n%2 == 0
	})
	data := map[string]any{"n": "4"}
	rules := map[string]string{"n": "even"}
	if err := Check(data, rules); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
