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
	if len(args) < 1 {
		fmt.Println("usage: tingo gen <model|controller> [args]")
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

// pkgName 从文件推导包名（启发式：用目录名）。
func pkgName(file string) string {
	dir := filepath.Dir(file)
	if dir == "." || dir == "" {
		return "main"
	}
	return filepath.Base(dir)
}
