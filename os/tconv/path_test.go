package tconv

import "testing"

func TestGetByPath(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name": "alice",
			"orders": []any{
				map[string]any{"amount": 10},
				map[string]any{"amount": 20},
			},
		},
	}
	if v, ok := GetByPath(data, "user.name"); !ok || v != "alice" {
		t.Errorf("user.name = %v %v", v, ok)
	}
	if v, ok := GetByPath(data, "user.orders.1.amount"); !ok || v != 20 {
		t.Errorf("user.orders.1.amount = %v %v", v, ok)
	}
	if _, ok := GetByPath(data, "user.orders.9.amount"); ok {
		t.Error("expected miss for out-of-range index")
	}
}

func TestMustGetByPath(t *testing.T) {
	data := map[string]any{"a": map[string]any{"b": 5}}
	if MustGetByPath(data, "a.b") != 5 {
		t.Error("a.b should be 5")
	}
	if MustGetByPath(data, "a.missing") != nil {
		t.Error("missing path should be nil")
	}
}
