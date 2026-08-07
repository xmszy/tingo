// Package command 是{{if .App}} {{.App}} 应用的{{end}}控制台指令（由 tingo make 生成）。
package command

import (
	"context"
	"fmt"
)

// {{.Name}} 指令。实现 tconsole.Command 接口后注册到控制台。
type {{.Name}} struct{}

// Name 返回指令名称。
func (c *{{.Name}}) Name() string { return "{{.Command}}" }

// Description 返回指令描述。
func (c *{{.Name}}) Description() string { return "{{.Name}} 指令" }

// Run 执行指令逻辑。
func (c *{{.Name}}) Run(ctx context.Context, args []string) error {
	fmt.Println("{{.Command}} 已执行")
	return nil
}
