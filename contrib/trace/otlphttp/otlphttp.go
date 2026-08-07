// Package otlphttp 提供通过 HTTP 导出 OTLP traces 的能力。
//
// 将 tingo HTTP 请求导出为 OpenTelemetry spans，
// 支持对接 Jaeger、Grafana Tempo 等标准 OTLP 后端。
//
// 依赖：go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
//
// 使用示例：
//
//	exporter, _ := otlphttp.New("http://localhost:4318", otlphttp.WithServiceName("myapp"))
//	defer exporter.Shutdown(context.Background())
//	router.Use(exporter.Middleware())
package otlphttp

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// semconv v1.26.0 中 HTTP 属性名。
const (
	serviceNameKey = "service.name"
	httpMethodKey  = "http.request.method"
	httpURLKey     = "url.full"
	httpTargetKey  = "url.path"
	httpSchemeKey  = "url.scheme"
	netHostNameKey = "server.address"
	httpStatusCode = "http.response.status_code"
)

// Exporter 将 HTTP 请求 span 导出到 OTLP collector。
type Exporter struct {
	endpoint    string
	serviceName string
	timeout     time.Duration
	sampleRatio float64
	provider    *sdktrace.TracerProvider
	tracer      oteltrace.Tracer
}

// Option 配置选项。
type Option func(*Exporter)

// WithServiceName 设置服务名（显示在 Jaeger 中）。默认 "tingo-app"。
func WithServiceName(name string) Option { return func(e *Exporter) { e.serviceName = name } }

// WithTimeout 设置导出超时，默认 10 秒。
func WithTimeout(d time.Duration) Option { return func(e *Exporter) { e.timeout = d } }

// WithSampleRatio 设置采样率 [0, 1]，默认 1（全采）。
func WithSampleRatio(r float64) Option { return func(e *Exporter) { e.sampleRatio = r } }

// New 创建 OTLP HTTP 导出器。
//
// endpoint 为 OTLP collector 地址，如 "http://localhost:4318"。
func New(endpoint string, opts ...Option) (*Exporter, error) {
	e := &Exporter{
		endpoint:    endpoint,
		serviceName: "tingo-app",
		timeout:     10 * time.Second,
		sampleRatio: 1.0,
	}
	for _, opt := range opts {
		opt(e)
	}

	exp, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithTimeout(e.timeout),
	)
	if err != nil {
		return nil, fmt.Errorf("otlphttp: create exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String(serviceNameKey, e.serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otlphttp: create resource: %w", err)
	}

	var sampler sdktrace.Sampler
	if e.sampleRatio >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if e.sampleRatio <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(e.sampleRatio))
	}

	e.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithExportTimeout(e.timeout)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(e.provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	e.tracer = e.provider.Tracer(e.serviceName)
	return e, nil
}

// TracerProvider 返回 SDKTracerProvider（用于高级配置）。
func (e *Exporter) TracerProvider() *sdktrace.TracerProvider {
	return e.provider
}

// Middleware 返回 gin-compatible 中间件，为每个请求创建 span 并导出。
func (e *Exporter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上游提取 trace context（支持 traceparent header）
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)

		spanCtx, span := e.tracer.Start(ctx,
			c.Request.Method+" "+c.FullPath(),
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				attribute.String(httpMethodKey, c.Request.Method),
				attribute.String(httpURLKey, c.Request.URL.String()),
				attribute.String(httpTargetKey, c.Request.URL.Path),
				attribute.String(httpSchemeKey, c.Request.URL.Scheme),
				attribute.String(netHostNameKey, c.Request.Host),
				attribute.String("http.client_ip", c.ClientIP()),
				attribute.String("http.user_agent", c.Request.UserAgent()),
			),
		)
		defer span.End()

		// 写回 trace id 到响应头
		if span.SpanContext().HasTraceID() {
			c.Header("X-Trace-Id", span.SpanContext().TraceID().String())
		}

		c.Request = c.Request.WithContext(spanCtx)
		c.Next()

		// 响应信息
		span.SetAttributes(
			attribute.Int(httpStatusCode, c.Writer.Status()),
		)
		if c.Writer.Status() >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", c.Writer.Status()))
			if len(c.Errors) > 0 {
				span.RecordError(c.Errors.Last().Err)
			}
		}
		if latency := time.Since(time.Now()); latency > 500*time.Millisecond {
			span.SetAttributes(
				attribute.String("slow", "true"),
				attribute.Float64("latency_ms", float64(latency.Milliseconds())),
			)
		}
	}
}

// Shutdown 优雅关闭，等待 pending span 导出完毕。
func (e *Exporter) Shutdown(ctx context.Context) error {
	return e.provider.Shutdown(ctx)
}

// ──────────────── 辅助 ────────────────

// NewTraceID 生成一个 TraceID，用于手动创建 span 关联。
func NewTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%032x", b)
}
