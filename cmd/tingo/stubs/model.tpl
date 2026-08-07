// Package model 是{{if .App}} {{.App}} 应用的{{end}}数据模型（由 tingo make 生成）。
package model

import (
	"github.com/xmszy/tingo/database/tdb"
	t "github.com/xmszy/tingo/frame"
)

// {{.Name}} 是表 {{.Table}} 的模型实体。
type {{.Name}} struct {
	Id int64 `json:"id" tdb:"id,pk,ai"`
}

func ({{.Name}}) TableName() string { return "{{.Table}}" }

// New{{.Name}} 返回使用默认或命名连接的查询模型。
func New{{.Name}}(connection ...string) *tdb.Model[{{.Name}}] {
	return tdb.NewModel[{{.Name}}](t.Database(connection...))
}
