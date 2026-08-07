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

func TestModelPascal(t *testing.T) {
	if got := pascalIdentifier("user_order"); got != "UserOrder" {
		t.Fatalf("got %s", got)
	}
}

func TestGeneratedModelTemplateRendersSingleLayerModel(t *testing.T) {
	columns := []tdb.Column{
		{Name: "id", Type: "bigint", Key: "PRI", Extra: "auto_increment", Comment: "主键"},
		{Name: "user_name", Type: "varchar(64)", Comment: "用户名"},
		{Name: "remark", Type: "varchar(255)", Nullable: true, Comment: "备注"},
	}
	var output strings.Builder
	if err := generatedModelTemplate.Execute(&output, map[string]any{
		"Package": "model", "Struct": "UserOrder", "Table": "user_order", "Columns": columns,
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
