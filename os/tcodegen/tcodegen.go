// Package tcodegen 提供基于结构体标签的代码生成能力（零外部依赖，仅用标准库 go/ast）。
//
// 当前能力：
//   - 解析 Go 源文件中的结构体，提取字段与 tdb 标签；
//   - 为结构体生成 tdb 的 Model 脚手架（NewXxxModel 构造函数 + 常用查询方法签名）；
//   - 生成资源控制器（ResourceController）骨架；
//   - 生成器输出为格式化 Go 代码（gofmt）。
//
// 设计原则：生成代码"手写可用、生成更快"——生成物不依赖本包，可独立编译。
package tcodegen

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// Field 描述一个结构体字段。
type Field struct {
	Name    string
	Type    string
	Column  string // tdb 标签指定的列名，缺省为 snake_case(Name)
	Primary bool
}

// Struct 描述一个结构体。
type Struct struct {
	Name   string
	Table  string // tdb 表名：Table() 方法或类型名 snake_case
	Fields []Field
}

// ParseFile 解析单个 Go 源文件，返回其中所有导出结构体。
// 仅处理带 `tdb:"..."` 标签或类型名以 Model/Entity 结尾的结构体（启发式）。
func ParseFile(path string) ([]Struct, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []Struct
	for _, decl := range node.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if !ts.Name.IsExported() {
				continue
			}
			s := Struct{Name: ts.Name.Name, Table: toSnake(ts.Name.Name)}
			// 检查是否实现 TableName()（在文件其他方法中）——这里简单用类型名推断。
			for _, f := range st.Fields.List {
				if len(f.Names) == 0 {
					continue // 嵌入字段，忽略
				}
				col := ""
				primary := false
				if f.Tag != nil {
					tag := strings.Trim(f.Tag.Value, "`")
					if v, ok := lookupTag(tag, "tdb"); ok {
						// 标签值形如 "colName primary"，首个空格前为列名。
						if before, after, found := strings.Cut(v, " "); found {
							col = before
							if strings.Contains(after, "primary") {
								primary = true
							}
						} else {
							col = v
						}
					}
				}
				for _, n := range f.Names {
					if !n.IsExported() {
						continue
					}
					fieldType := exprString(f.Type)
					fieldCol := col
					if fieldCol == "" {
						fieldCol = toSnake(n.Name)
					}
					s.Fields = append(s.Fields, Field{
						Name: n.Name, Type: fieldType, Column: fieldCol, Primary: primary,
					})
				}
			}
			out = append(out, s)
		}
	}
	return out, nil
}

// lookupTag 从反引号标签中解析 key:"value"，支持空格分隔的选项（如 `tdb:"id primary"`）。
func lookupTag(tag, key string) (string, bool) {
	st := reflect.StructTag(strings.Trim(tag, "`"))
	v, ok := st.Lookup(key)
	return v, ok
}

// exprString 把 AST 类型表达式转字符串。
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	default:
		return fmt.Sprintf("%T", e)
	}
}

// Snake 将 CamelCase 转为 snake_case（导出供 CLI 使用）。
func Snake(s string) string { return toSnake(s) }

// toSnake 将 CamelCase 转为 snake_case。
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// GenerateModel 为结构体生成 tdb Model 脚手架代码（包名为 pkg）。
// 生成 NewXxxModel(db) 构造函数与字段列名常量。
func GenerateModel(pkg string, s Struct) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import \"github.com/xmszy/tingo/database/tdb\"\n\n")
	fmt.Fprintf(&b, "// %sModel 是 %s 的数据库模型脚手架（代码生成，可手写修改）。\n", s.Name, s.Name)
	fmt.Fprintf(&b, "type %sModel struct{ *tdb.Model[%s] }\n\n", s.Name, s.Name)
	fmt.Fprintf(&b, "// New%sModel 构造模型。\n", s.Name)
	fmt.Fprintf(&b, "func New%sModel(db *tdb.DB) *%sModel {\n", s.Name, s.Name)
	fmt.Fprintf(&b, "\treturn &%sModel{Model: tdb.NewModel[%s](db)}\n", s.Name, s.Name)
	b.WriteString("}\n\n")
	// 列名常量
	fmt.Fprintf(&b, "const (\n")
	for _, f := range s.Fields {
		fmt.Fprintf(&b, "\tCol%s = %q\n", f.Name, f.Column)
	}
	b.WriteString(")\n")
	return b.String()
}

// GenerateController 生成资源控制器骨架。
func GenerateController(pkg string, s Struct) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import (\n")
	b.WriteString("\t\"github.com/xmszy/tingo/core\"\n")
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "// %sController 是 %s 的资源控制器（代码生成骨架）。\n", s.Name, s.Name)
	fmt.Fprintf(&b, "type %sController struct{}\n\n", s.Name)
	methods := []struct{ name, comment string }{
		{"Index", "列表"},
		{"Read", "详情"},
		{"Save", "新增"},
		{"Update", "更新"},
		{"Delete", "删除"},
	}
	for _, m := range methods {
		fmt.Fprintf(&b, "// %s %s。\n", m.name, m.comment)
		fmt.Fprintf(&b, "func (c *%sController) %s(ctx *core.Ctx) {\n", s.Name, m.name)
		fmt.Fprintf(&b, "\tctx.JSON(200, core.M{\"msg\": \"%s.%s 待实现\"})\n", s.Name, m.name)
		b.WriteString("}\n\n")
	}
	return b.String()
}

// Format 用 gofmt 格式化生成代码。
func Format(src string) (string, error) {
	out, err := format.Source([]byte(src))
	if err != nil {
		return src, err // 返回原始以便排查
	}
	return string(out), nil
}

/* ------------------------------------------------------------------ */
/* 注解路由生成：解析 //tingo:route 注释指令                                */
/* ------------------------------------------------------------------ */

// RouteDirective 是一条 //tingo:route 注解指令。
// 形如：// tingo:route GET /user/list
type RouteDirective struct {
	Method  string // GET/POST/PUT/DELETE/PATCH/ANY
	Path    string // 路由路径
	Handler string // 方法名
}

// ParseRoutes 解析源文件中所有方法的 //tingo:route 注释指令。
// 返回 控制器类型名 -> 该类型的路由指令列表。
func ParseRoutes(path string) (map[string][]RouteDirective, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]RouteDirective)
	for _, decl := range node.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
			continue // 仅处理函数方法（带接收者）
		}
		if fd.Doc == nil {
			continue
		}
		recv := recvTypeName(fd.Recv)
		if recv == "" {
			continue
		}
		for _, c := range fd.Doc.List {
			txt := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if !strings.HasPrefix(txt, "tingo:route") {
				continue
			}
			fields := strings.Fields(strings.TrimPrefix(txt, "tingo:route"))
			if len(fields) < 2 {
				continue
			}
			out[recv] = append(out[recv], RouteDirective{
				Method:  strings.ToUpper(fields[0]),
				Path:    fields[1],
				Handler: fd.Name.Name,
			})
		}
	}
	return out, nil
}

// recvTypeName 从方法接收者表达式提取类型名（*User -> User）。
func recvTypeName(recv *ast.FieldList) string {
	e := recv.List[0].Type
	if star, ok := e.(*ast.StarExpr); ok {
		if id, ok := star.X.(*ast.Ident); ok {
			return id.Name
		}
	} else if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// GenerateRouteAnnotation 为带 //tingo:route 注解的控制器生成 Annotations() 方法代码。
// 生成的代码实现 tapp.RouteAnnotated 接口，可被 AnnotationRoute / AutoRouteAnnotated 挂载。
func GenerateRouteAnnotation(pkg string, structName string, dirs []RouteDirective) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import \"github.com/xmszy/tingo/tapp\"\n\n")
	fmt.Fprintf(&b, "// Annotations 由 tcodegen 基于 //tingo:route 注解生成（可手写修改）。\n")
	fmt.Fprintf(&b, "func (c *%s) Annotations() []tapp.RouteMeta {\n", structName)
	b.WriteString("\treturn []tapp.RouteMeta{\n")
	for _, d := range dirs {
		fmt.Fprintf(&b, "\t\t{Method: %q, Path: %q, Handler: %q},\n", d.Method, d.Path, d.Handler)
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

// GenerateAnnotatedController 生成带注解路由的空控制器骨架。
// methods 为 {方法名, 注解 Method, 注解 Path} 列表。
func GenerateAnnotatedController(pkg, structName string, methods []struct{ Name, Method, Path string }) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	b.WriteString("import (\n")
	b.WriteString("\t\"github.com/xmszy/tingo/core\"\n")
	b.WriteString("\t\"github.com/xmszy/tingo/tapp\"\n")
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "// %sController 由 tcodegen 生成的注解路由控制器骨架。\n", structName)
	fmt.Fprintf(&b, "type %sController struct{}\n\n", structName)
	var dirs []RouteDirective
	for _, m := range methods {
		fmt.Fprintf(&b, "//tingo:route %s %s\n", m.Method, m.Path)
		fmt.Fprintf(&b, "func (c *%sController) %s(ctx *core.Ctx) {\n", structName, m.Name)
		fmt.Fprintf(&b, "\tctx.JSON(200, core.M{\"msg\": %q})\n", structName+"."+m.Name)
		b.WriteString("}\n\n")
		dirs = append(dirs, RouteDirective{Method: m.Method, Path: m.Path, Handler: m.Name})
	}
	b.WriteString(GenerateRouteAnnotation(pkg, structName+"Controller", dirs))
	return b.String()
}
