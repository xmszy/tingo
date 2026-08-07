// Package tconv Map/Struct 转换测试。
package tconv_test

import (
	"testing"

	"github.com/xmszy/tingo/os/tconv"
)

type SimpleUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

type ComplexUser struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
}

func TestMapToStruct(t *testing.T) {
	m := map[string]any{
		"id":    float64(42),
		"name":  "Alice",
		"age":   float64(30),
		"email": "alice@example.com",
	}
	var u SimpleUser
	if err := tconv.MapToStruct(m, &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != 42 {
		t.Errorf("id = %d, want 42", u.ID)
	}
	if u.Name != "Alice" {
		t.Errorf("name = %s, want Alice", u.Name)
	}
	if u.Age != 30 {
		t.Errorf("age = %d, want 30", u.Age)
	}
}

func TestMapToStruct_ComplexTypes(t *testing.T) {
	m := map[string]any{
		"id":        1,
		"name":      "Bob",
		"score":     99.5,
		"is_active": true,
		"created_at": "2025-06-15",
	}
	var u ComplexUser
	if err := tconv.MapToStruct(m, &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != 1 || u.Name != "Bob" || u.Score != 99.5 || !u.IsActive || u.CreatedAt != "2025-06-15" {
		t.Errorf("unexpected result: %+v", u)
	}
}

func TestMapToStruct_StringMap(t *testing.T) {
	m := map[string]string{
		"name":  "Charlie",
		"email": "charlie@example.com",
	}
	var u SimpleUser
	if err := tconv.MapToStruct(m, &u); err != nil {
		t.Fatal(err)
	}
	if u.Name != "Charlie" || u.Email != "charlie@example.com" {
		t.Errorf("unexpected result: %+v", u)
	}
}

func TestScanStruct(t *testing.T) {
	m := map[string]any{
		"id":    100,
		"name":  "Diana",
		"score": 88.8,
	}
	var u ComplexUser
	if err := tconv.ScanStruct(m, &u, tconv.ScanOption{TagName: "json"}); err != nil {
		t.Fatal(err)
	}
	if u.ID != 100 || u.Name != "Diana" {
		t.Errorf("unexpected result: %+v", u)
	}
}

func TestScanStruct_ContinueOnError(t *testing.T) {
	m := map[string]any{
		"id":        "not_an_int", // best-effort: string→int → 0
		"name":      "Eve",
		"is_active": true,
	}
	var u ComplexUser
	err := tconv.ScanStruct(m, &u, tconv.ScanOption{ContinueOnError: true, TagName: "json"})
	// continue-on-error 不会因类型转换失败而报错（tconv 尽力而为），
	// 但 name 字段应该被正确设置
	if err != nil {
		t.Errorf("unexpected error (best-effort should continue): %v", err)
	}
	if u.Name != "Eve" {
		t.Errorf("name = %s, want Eve", u.Name)
	}
}

func TestMapToStruct_StructInput(t *testing.T) {
	src := struct {
		Name  string
		Email string
	}{Name: "Frank", Email: "frank@test.com"}
	var dst SimpleUser
	if err := tconv.MapToStruct(src, &dst); err != nil {
		t.Fatal(err)
	}
	if dst.Name != "Frank" || dst.Email != "frank@test.com" {
		t.Errorf("unexpected result: %+v", dst)
	}
}

func TestMapToStruct_NilPointer(t *testing.T) {
	err := tconv.MapToStruct(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil output")
	}
}
