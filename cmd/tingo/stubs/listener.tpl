// Package listener 是{{if .App}} {{.App}} 应用的{{end}}事件监听器（由 tingo make 生成）。
package listener

import (
	"context"
	"log"
)

// {{.Name}} 事件监听器。通过 tevent.Subscribe 注册到事件总线。
type {{.Name}} struct{}

// Handle 处理事件。ctx 为请求上下文，payload 为事件载荷。
func (l *{{.Name}}) Handle(ctx context.Context, payload any) error {
	log.Printf("[{{.Name}}] 事件触发, payload: %+v", payload)
	return nil
}
