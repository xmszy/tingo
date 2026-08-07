// Package subscribe 是{{if .App}} {{.App}} 应用的{{end}}事件订阅者（由 tingo make 生成）。
package subscribe

// {{.Name}} 事件订阅者。
// 在 kernel.go 中将 subscribe.Events() 注册到 tevent.Bus。
type {{.Name}} struct{}

// Events 返回事件名 → 处理函数的映射。
func (s *{{.Name}}) Events() map[string]func(payload any) error {
	return map[string]func(payload any) error{
		// "app.init": s.onAppInit,
	}
}
