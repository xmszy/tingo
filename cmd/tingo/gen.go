package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xmszy/tingo/os/tcodegen"
)

// genCmd 处理 gen 子命令。
//
//	tingo gen model [--connection name] [--tables ...]
//	tingo gen controller <file>
func genCmd(args []string) {
	// 帮助优先：-h / --help / help 任意位置都打印用法。
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			usageGen()
			os.Exit(0)
		}
	}
	if len(args) < 1 {
		usageGen()
		os.Exit(2)
	}
	if args[0] == "model" {
		runModelGeneration(args[1:])
		return
	}
	if len(args) < 2 {
		fmt.Println("usage: tingo gen controller <file>")
		os.Exit(2)
	}
	kind, file := args[0], args[1]
	structs, err := tcodegen.ParseFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析失败: %v\n", err)
		os.Exit(1)
	}
	if len(structs) == 0 {
		fmt.Println("未发现可导出的结构体")
		return
	}
	pkg := pkgName(file)
	outDir := filepath.Dir(file)
	for _, s := range structs {
		var src, outFile string
		switch kind {
		case "controller":
			src = tcodegen.GenerateController(pkg, s)
			outFile = filepath.Join(outDir, fmt.Sprintf("%s_controller_gen.go", tcodegen.Snake(s.Name)))
		default:
			fmt.Printf("未知生成类型: %s\n", kind)
			os.Exit(2)
		}
		src, err = tcodegen.Format(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "格式化警告(%s): %v\n", s.Name, err)
		}
		if err := os.WriteFile(outFile, []byte(src), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已生成: %s\n", outFile)
	}
}

// usageGen 打印 gen 子命令用法。
func usageGen() {
	fmt.Print(`用法: tingo gen <类型> [选项]

类型:
  model       按 config/database.toml 反向生成 app/model 单层模型
  controller  <file>  从 Go 源文件中的结构体生成资源控制器骨架

示例:
  tingo gen model                 从默认数据库生成全部表
  tingo gen model --tables user,order
  tingo gen controller app/model/user.go

各类型详细选项请使用：tingo gen <类型> -h
`)
}

// pkgName 从文件推导包名（启发式：用目录名）。
func pkgName(file string) string {
	dir := filepath.Dir(file)
	if dir == "." || dir == "" {
		return "main"
	}
	return filepath.Base(dir)
}
