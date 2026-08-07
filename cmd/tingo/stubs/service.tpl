// Package service 是{{if .App}} {{.App}} 应用的{{end}}服务层（由 tingo make 生成）。
package service

// {{.Name}} 是业务服务聚合点。
type {{.Name}} struct{}

// New{{.Name}} 创建服务实例。
func New{{.Name}}() *{{.Name}} { return &{{.Name}}{} }
