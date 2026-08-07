// Package controller 是{{if .App}} {{.App}} 应用的{{end}}控制器（由 tingo make 生成）。
package controller

import t "github.com/xmszy/tingo/frame"

// {{.Name}} 控制器。
type {{.Name}} struct{}

// Index 列表页。
func (c *{{.Name}}) Index(ctx *t.Ctx) {
	ctx.JSON(t.Map{"action": "index"})
}

// Show 详情。
func (c *{{.Name}}) Show(ctx *t.Ctx) {
	id := ctx.Param("id")
	ctx.JSON(t.Map{"action": "show", "id": id})
}

// Store 保存。
func (c *{{.Name}}) Store(ctx *t.Ctx) {
	ctx.JSON(t.Map{"action": "store"})
}

// Update 更新。
func (c *{{.Name}}) Update(ctx *t.Ctx) {
	id := ctx.Param("id")
	ctx.JSON(t.Map{"action": "update", "id": id})
}

// Delete 删除。
func (c *{{.Name}}) Delete(ctx *t.Ctx) {
	id := ctx.Param("id")
	ctx.JSON(t.Map{"action": "delete", "id": id})
}
