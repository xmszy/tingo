package tcodegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `package model

type User struct {
	ID   int64  ` + "`tdb:\"id primary\"`" + `
	Name string ` + "`tdb:\"name\"`" + `
	Age  int    ` + "`tdb:\"age\"`" + `
}
`

func TestParseAndGenerate(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "model.go")
	if err := os.WriteFile(f, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	structs, err := ParseFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(structs) != 1 {
		t.Fatalf("want 1 struct, got %d", len(structs))
	}
	s := structs[0]
	if s.Name != "User" {
		t.Fatalf("name=%s", s.Name)
	}
	if s.Table != "user" {
		t.Fatalf("table=%s", s.Table)
	}
	if len(s.Fields) != 3 {
		t.Fatalf("fields=%d", len(s.Fields))
	}
	if !s.Fields[0].Primary {
		t.Fatal("ID should be primary")
	}
	if s.Fields[0].Column != "id" {
		t.Fatalf("col=%s", s.Fields[0].Column)
	}

	modelSrc := GenerateModel("model", s)
	if !strings.Contains(modelSrc, "NewUserModel") {
		t.Fatal("model should contain NewUserModel")
	}
	if !strings.Contains(modelSrc, "ColID") {
		t.Fatal("model should contain ColID")
	}
	// 生成代码应能通过 gofmt。
	if _, err := Format(modelSrc); err != nil {
		t.Fatalf("model format error: %v", err)
	}

	ctrlSrc := GenerateController("controller", s)
	if !strings.Contains(ctrlSrc, "UserController") || !strings.Contains(ctrlSrc, "func (c *UserController) Index") {
		t.Fatal("controller generation wrong")
	}
	if _, err := Format(ctrlSrc); err != nil {
		t.Fatalf("controller format error: %v", err)
	}
}
