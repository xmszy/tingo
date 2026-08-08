// Package validate 是{{if .App}} {{.App}} 应用的{{end}}校验器（由 tingo make 生成）。
package validate

import "github.com/xmszy/tingo-contrib/validate"

// {{.Name}} 校验规则。用法：validate.Check(data, {{.Name}})
var {{.Name}} = map[string]string{
	// "field": "require|max:25|email",
}
