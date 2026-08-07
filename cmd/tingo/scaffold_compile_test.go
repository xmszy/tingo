package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestScaffoldTemplatesAreValidGo 确保 tingo init 产出的每个 Go 文件都语法合法。
//
// render() 在 gofmt 失败时会静默保留原文，语法错误不会在生成阶段暴露，
// 因此必须由本测试守护，避免脚手架产出无法编译的项目。
func TestScaffoldTemplatesAreValidGo(t *testing.T) {
	data := map[string]string{
		"Module":     "example.com/demo",
		"Name":       "demo",
		"Replace":    "",
		"LeftDelim":  "{{",
		"RightDelim": "}}",
	}

	// 覆盖 initCmd 中登记的全部 Go 模板。
	goTemplates := map[string]string{
		"main.go":                 tplMain,
		"app/app.go":              tplApplication,
		"app/route/app.go":        tplRoute,
		"app/controller/index.go": tplController,
		"app/controller/base.go":  tplBaseController,
		"app/kernel.go":           tplKernel,
		"app/exception.go":        tplExceptionHandle,
		"app/provider.go":         tplAppService,
		"app/common.go":           tplCommon,
		"app/middleware/auth.go":  tplMiddlewareAuth,
	}

	fset := token.NewFileSet()
	for name, tpl := range goTemplates {
		src, err := render(tpl, data, true)
		if err != nil {
			t.Fatalf("%s 渲染失败: %v", name, err)
		}
		file, err := parser.ParseFile(fset, name, src, parser.AllErrors)
		if err != nil {
			t.Errorf("%s 不是合法的 Go 源码: %v\n%s", name, err, src)
			continue
		}
		// 模板不应残留未替换的占位符。
		if strings.Contains(string(src), "{{.") {
			t.Errorf("%s 残留未渲染的模板占位符:\n%s", name, src)
		}
		assertImportsUsed(t, name, file)
	}
}

// assertImportsUsed 检查文件没有未使用的 import。
// Go 编译器会因未使用的 import 直接报错，这是脚手架最易犯的错误。
//
// 通过遍历 AST 收集所有形如 pkg.Sym 的选择器来判断引用情况，
// 比文本匹配更准确（不会把注释和 import 行本身算作引用）。
func assertImportsUsed(t *testing.T, name string, file *ast.File) {
	t.Helper()

	used := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok {
			used[ident.Name] = true
		}
		return true
	})

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		// 空白导入用于副作用，无需被引用。
		if imp.Name != nil && imp.Name.Name == "_" {
			continue
		}
		pkg := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			pkg = path[i+1:]
		}
		if imp.Name != nil {
			pkg = imp.Name.Name
		}
		if !used[pkg] {
			t.Errorf("%s 导入了 %q 但未使用，将导致编译失败", name, path)
		}
	}
}
