// Package otel 提供 OpenTelemetry 追踪集成。
//
// 需要 go get go.opentelemetry.io/otel
//
// 将 OpenTelemetry Span 与 tingo thttp 请求上下文绑定，
// 自动在中间件和请求处理间隙注入 TraceID 和 Span 信息。
package otel

// 占位——OpenTelemetry 集成需要 otel SDK 依赖。
const Version = "0.0.1"
