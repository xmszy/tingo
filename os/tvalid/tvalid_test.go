package tvalid

import (
	"sync"
	"testing"
)

type addr struct {
	City string `valid:"required" label:"城市"`
}

type userForm struct {
	Name     string `valid:"required|len-min:2|len-max:20" label:"用户名"`
	Email    string `valid:"required|email" label:"邮箱"`
	Password string `valid:"required|len-min:6|len-max:32|confirm" label:"密码" form:"password"`
	Confirm  string `valid:"required" label:"确认密码" form:"password_confirmation"`
	Age      int    `valid:"min:0|max:150" label:"年龄"`
	Addr     addr   `valid:"required"`
}

// 校验通过的数据
func validUser() *userForm {
	return &userForm{
		Name:     "张三abc",
		Email:    "a@b.com",
		Password: "secret123",
		Confirm:  "secret123",
		Age:      30,
		Addr:     addr{City: "BJ"},
	}
}

func TestCheckStruct_OK(t *testing.T) {
	if err := CheckStruct(validUser()); err != nil {
		t.Fatalf("期望通过, 实际: %v", err)
	}
}

func asErrors(t *testing.T, err error) Errors {
	t.Helper()
	es, ok := err.(Errors)
	if !ok {
		t.Fatalf("期望 Errors, 实际 %T: %v", err, err)
	}
	return es
}

func TestCheckStruct_Required(t *testing.T) {
	u := validUser()
	u.Name = ""
	err := CheckStruct(u)
	if err == nil {
		t.Fatal("期望校验失败")
	}
	if asErrors(t, err).First().Field != "用户名" {
		t.Fatalf("期望 label 用户名, 实际: %s", asErrors(t, err).First().Field)
	}
}

func TestCheckStruct_Email(t *testing.T) {
	u := validUser()
	u.Email = "not-an-email"
	err := CheckStruct(u)
	if err == nil {
		t.Fatal("期望校验失败")
	}
	if asErrors(t, err).First().Rule != "email" {
		t.Fatalf("期望 email 规则失败, 实际: %v", err)
	}
}

func TestCheckStruct_Confirm(t *testing.T) {
	u := validUser()
	u.Confirm = "mismatch"
	err := CheckStruct(u)
	if err == nil {
		t.Fatal("期望 confirm 失败")
	}
}

func TestCheckStruct_Nested(t *testing.T) {
	// 说明：当前实现仅校验顶层字段的 valid tag（嵌套结构体字段不参与校验）。
	// 这里保证含嵌套结构时校验不报错，且顶层字段仍正常工作。
	u := validUser()
	u.Addr.City = ""
	if err := CheckStruct(u); err != nil {
		t.Fatalf("含嵌套结构不应报错, 实际: %v", err)
	}
	u.Name = ""
	if err := CheckStruct(u); err == nil {
		t.Fatal("顶层字段仍应校验失败")
	}
}

func TestCheckStruct_CacheConsistency(t *testing.T) {
	// 相同类型多次校验结果必须一致（缓存不污染值）
	for i := 0; i < 100; i++ {
		if err := CheckStruct(validUser()); err != nil {
			t.Fatalf("第 %d 次校验异常: %v", i, err)
		}
		u := validUser()
		u.Name = ""
		if err := CheckStruct(u); err == nil {
			t.Fatalf("第 %d 次应失败", i)
		}
	}
}

// 无 valid tag / 无 confirm 的结构体应走 fast-path 直接返回 nil，不产生校验开销。
type plainForm struct {
	Name  string
	Email string
	Age   int
}

func TestCheckStruct_NoRules_FastPath(t *testing.T) {
	if err := CheckStruct(&plainForm{Name: "a", Email: "b", Age: 1}); err != nil {
		t.Fatalf("无规则结构体应直接通过, 实际: %v", err)
	}
	if err := CheckStruct(plainForm{Name: "a"}); err != nil {
		t.Fatalf("非指针无规则结构体也应直接通过, 实际: %v", err)
	}
}

func BenchmarkCheckStruct_NoRules(b *testing.B) {
	f := &plainForm{Name: "a", Email: "b", Age: 1}
	b.ReportAllocs()
	for b.Loop() {
		_ = CheckStruct(f)
	}
}

// 缓存带来性能收益：重复校验同一类型应显著快于首次解析。
func BenchmarkCheckStruct(b *testing.B) {
	u := validUser()
	b.ReportAllocs()

	for b.Loop() {
		_ = CheckStruct(u)
	}
}

// 并发安全：多 goroutine 同类型校验。
func TestCheckStruct_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				_ = CheckStruct(validUser())
			}
		}()
	}
	wg.Wait()
}

// ──────────────── 跨字段/区间规则 ────────────────

func TestCrossRules(t *testing.T) {
	// same 失败：pwd != confirm_pwd
	bad := &struct {
		Pwd    string `valid:"required|same:confirm_pwd" label:"pwd"`
		Confirm string `valid:"required" label:"confirm_pwd"`
	}{Pwd: "a", Confirm: "b"}
	if err := CheckStruct(bad); err == nil {
		t.Fatal("same 应当失败")
	}
	// same 通过
	good := &struct {
		Pwd    string `valid:"required|same:confirm_pwd" label:"pwd"`
		Confirm string `valid:"required" label:"confirm_pwd"`
	}{Pwd: "a", Confirm: "a"}
	if err := CheckStruct(good); err != nil {
		t.Fatalf("same 应通过, 实际: %v", err)
	}
	// gte 失败：age < min_age
	badGte := &struct {
		Age    int `valid:"gte:min_age" label:"age"`
		MinAge int `valid:"required" label:"min_age"`
	}{Age: 10, MinAge: 20}
	if err := CheckStruct(badGte); err == nil {
		t.Fatal("gte 应当失败")
	}
	// gte 通过
	goodGte := &struct {
		Age    int `valid:"gte:min_age" label:"age"`
		MinAge int `valid:"required" label:"min_age"`
	}{Age: 20, MinAge: 20}
	if err := CheckStruct(goodGte); err != nil {
		t.Fatalf("gte 应通过, 实际: %v", err)
	}
	// date-range 失败：超出区间
	badDate := &struct {
		Start string `valid:"date-range:2020-01-01,2030-12-31" label:"start"`
	}{Start: "2000-01-01"}
	if err := CheckStruct(badDate); err == nil {
		t.Fatal("date-range 应当失败")
	}
	// date-range 通过
	goodDate := &struct {
		Start string `valid:"date-range:2020-01-01,2030-12-31" label:"start"`
	}{Start: "2025-06-01"}
	if err := CheckStruct(goodDate); err != nil {
		t.Fatalf("date-range 应通过, 实际: %v", err)
	}
}
