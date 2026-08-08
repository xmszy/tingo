package main

import (
	"go/format"
	"strings"
	"testing"

	"github.com/xmszy/tingo/database/tdb"
)

func TestModelGoTypeMapping(t *testing.T) {
	cases := []struct {
		col  tdb.Column
		want string
	}{
		{tdb.Column{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"}, "int64"},
		{tdb.Column{Name: "age", Type: "int", Nullable: true}, "*int"},
		{tdb.Column{Name: "name", Type: "varchar(255)"}, "string"},
		{tdb.Column{Name: "price", Type: "decimal(10,2)"}, "float64"},
		{tdb.Column{Name: "active", Type: "tinyint(1)"}, "bool"},
		{tdb.Column{Name: "avatar", Type: "blob", Nullable: true}, "*[]byte"},
	}
	for _, test := range cases {
		if got := modelColumnGoType(test.col); got != test.want {
			t.Errorf("modelColumnGoType(%q) = %q, want %q", test.col.Type, got, test.want)
		}
	}
}

func TestGeneratedModelRenamesTableNameFieldOnConflict(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"},
		{		Name: "table_name", Type: "varchar(64)", Comment: "数据表名"},
	}
	views, _ := buildFieldViews(columns, "ba_crud_log")
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "BaCrudLog", "Table": "ba_crud_log",
		"Fields": views,
	}); err != nil {
		t.Fatal(err)
	}
	src := output.String()
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("model not gofmt: %v\n%s", err, src)
	}
	if !strings.Contains(src, "func (BaCrudLog) TableName() string { return \"ba_crud_log\" }") {
		t.Fatalf("TableName() method must be emitted so the model can report its table:\n%s", src)
	}
	if !strings.Contains(src, "TableNameField string `json:\"table_name\" tdb:\"table_name\" valid:\"required|max:64\" label:\"数据表名\"`") {
		t.Fatalf("table_name column should map to field TableNameField with validation:\n%s", src)
	}
	if strings.Contains(src, "TableName string `json:\"table_name\"") {
		t.Fatalf("field must not be named TableName (would clash with method):\n%s", src)
	}
}

func TestGeneratedModelEmitsTableNameMethodWithoutConflict(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"},
	}
	views, _ := buildFieldViews(columns, "user")
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "User", "Table": "user",
		"Fields": views,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "func (User) TableName() string { return \"user\" }") {
		t.Fatalf("TableName() method missing:\n%s", output.String())
	}
}

func TestModelPascal(t *testing.T) {
	if got := pascalIdentifier("user_order"); got != "UserOrder" {
		t.Fatalf("got %s", got)
	}
}

func TestGeneratedModelTemplateRendersSingleLayerModel(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment", Comment: "主键"},
		{Name: "user_name", Type: "varchar(64)", Comment: "用户名"},
		{		Name: "remark", Type: "varchar(255)", Nullable: true, Comment: "备注"},
	}
	views, _ := buildFieldViews(columns, "user_order")
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "UserOrder", "Table": "user_order", "Fields": views,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := format.Source([]byte(output.String())); err != nil {
		t.Fatalf("model not gofmt: %v\n%s", err, output.String())
	}
	for _, expected := range []string{
		"type UserOrder struct", "tdb:\"id,pk,ai\"", "func NewUserOrder(connection ...string)", "t.Database(connection...)",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q:\n%s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "Dao") || strings.Contains(output.String(), "entity.") {
		t.Fatalf("legacy dao/entity leaked into model:\n%s", output.String())
	}
}

// TestGeneratedModelEmitsTimestampSoftDeleteAndValidation 验证 P0 三件套：
// 自动时间戳 tag + AutoTimestamp 调用、软删除类型、tvalid 校验规则、Validate 入口。
func TestGeneratedModelEmitsTimestampSoftDeleteAndValidation(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"},
		{Name: "email", Type: "varchar(120)", Comment: "邮箱"},
		{Name: "status", Type: "tinyint(1)", Comment: "状态"},
		{Name: "create_time", Type: "datetime", Comment: "创建时间"},
		{Name: "update_time", Type: "datetime", Comment: "更新时间"},
		{Name: "deleted_at", Type: "datetime", Comment: "删除时间"},
	}
	views, hasTimestamp := buildFieldViews(columns, "member")
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "Member", "Table": "member",
		"Fields":        views,
		"HasTimestamp":  hasTimestamp,
		"CreateColName": "create_time",
		"UpdateColName": "update_time",
	}); err != nil {
		t.Fatal(err)
	}
	src := output.String()
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("model not gofmt: %v\n%s", err, src)
	}
	for _, expected := range []string{
		`timestamp:"create"`, `timestamp:"update"`, `timestamp:"delete"`,
		`mm = mm.AutoTimestamp("create_time", "update_time")`,
		`DeletedAt tdb.SoftDelete ` + "`",
		`valid:"required|max:120|email" label:"邮箱"`,
		`valid:"required|in:0,1" label:"状态"`,
		`func (x *Member) Validate() error { return tvalid.CheckStruct(x) }`,
		`"time"`, `"github.com/xmszy/tingo/os/tvalid"`,
	} {
		if !strings.Contains(src, expected) {
			t.Fatalf("missing %q:\n%s", expected, src)
		}
	}
}

// TestGeneratedModelJsonAndEnum 验证 JSON 便捷方法、enum 真实 Go 类型与常量生成。
func TestGeneratedModelJsonAndEnum(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"},
		{Name: "ext", Type: "json", Nullable: true, Comment: "扩展"},
		{Name: "gender", Type: "enum('male','female')", Comment: "性别"},
	}
	views, _ := buildFieldViews(columns, "profile")
	enums := buildEnums(views)
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "Profile", "Table": "profile", "Fields": views, "Enums": enums,
	}); err != nil {
		t.Fatal(err)
	}
	src := output.String()
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("model not gofmt: %v\n%s", err, src)
	}
	for _, expected := range []string{
		"Ext *string `json:\"ext\"",                       // JSON 列仍用 *string
		"Gender ProfileGenderEnum `json:\"gender\"",      // enum 列换用表前缀枚举类型
		"type ProfileGenderEnum string",                  // 枚举类型定义
		"ProfileGenderEnumMale ProfileGenderEnum = \"male\"",   // 枚举常量
		"ProfileGenderEnumFemale ProfileGenderEnum = \"female\"", // 枚举常量
		"func (x *Profile) GetExt() (map[string]any, error)",   // JSON 获取
		"func (x *Profile) SetExt(v any) error",                // JSON 设置
		"func (x *Profile) BeforeInsert() error { return nil }", // 钩子
		"func (x *Profile) AfterQuery() error { return nil }",   // 钩子
		"\"encoding/json\"",
	} {
		if !strings.Contains(src, expected) {
			t.Fatalf("missing %q:\n%s", expected, src)
		}
	}
	if strings.Contains(src, "enum") && !strings.Contains(src, "ProfileGenderEnum") {
		t.Fatalf("enum column must use generated Go type, not raw string:\n%s", src)
	}
}

// TestGeneratedModelNewAdminPermBeforeHooks 验证 NewRoleAdminPerm 构造函数（按表名命名）生成在钩子方法之前。
func TestGeneratedModelNewAdminPermBeforeHooks(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"},
	}
	views, _ := buildFieldViews(columns, "role")
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "Role", "Table": "role", "Fields": views,
	}); err != nil {
		t.Fatal(err)
	}
	src := output.String()
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("model not gofmt: %v\n%s", err, src)
	}
	if !strings.Contains(src, "func NewRoleAdminPerm() *tdb.Model[Role]") {
		t.Fatalf("NewRoleAdminPerm constructor missing:\n%s", src)
	}
	if !strings.Contains(src, "t.DatabaseFor(\"admin\")") {
		t.Fatalf("NewRoleAdminPerm should use admin application scoped connection:\n%s", src)
	}
	// 顺序：NewRoleAdminPerm 必须出现在第一个钩子 BeforeInsert 之前。
	idxCon := strings.Index(src, "func NewRoleAdminPerm(")
	if idxCon < 0 {
		t.Fatal("NewRoleAdminPerm not found")
	}
	idxHook := strings.Index(src, "func (x *Role) BeforeInsert()")
	if idxHook < 0 {
		t.Fatal("BeforeInsert hook not found")
	}
	if idxCon > idxHook {
		t.Fatalf("NewRoleAdminPerm must appear before hooks:\n%s", src)
	}
}

// TestGeneratedModelEnumNullableUsesPointer 验证可空 enum 列生成 *XxxEnum。
func TestGeneratedModelEnumNullableUsesPointer(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment"},
		{Name: "gender", Type: "enum('male','female')", Nullable: true, Comment: "性别"},
	}
	views, _ := buildFieldViews(columns, "profile")
	var found bool
	for _, v := range views {
		if v.Name == "gender" {
			if v.GoType != "*ProfileGenderEnum" {
				t.Fatalf("nullable enum should be *GenderEnum, got %q", v.GoType)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("gender column not found")
	}
}
