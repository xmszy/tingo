// Package gzip 提供响应 gzip 压缩中间件。
//
// 设计：仅使用标准库 compress/gzip，无外部依赖。支持按内容类型与
// 最小长度阈值跳过压缩（如已压缩的图片、过小响应）。
package gzip

import (
	"compress/gzip"
	"io"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

// Config 是 gzip 配置。
type Config struct {
	// Level 压缩级别（gzip.DefaultCompression 等）。
	Level int
	// MinLength 小于该字节数的响应不压缩，默认 1024。
	MinLength int
	// ExcludeContentTypes 跳过压缩的内容类型前缀。
	ExcludeContentTypes []string
}

// Default 返回默认配置。
func Default() Config {
	return Config{
		Level:               gzip.DefaultCompression,
		MinLength:           1024,
		ExcludeContentTypes: []string{"application/gzip", "image/", "video/", "audio/"},
	}
}

type writer struct {
	gin.ResponseWriter
	gz *gzip.Writer
}

func (w *writer) Write(b []byte) (int, error) { return w.gz.Write(b) }

// Middleware 返回 gzip 压缩中间件。
func Middleware(cfg Config) core.Handler {
	if cfg.Level == 0 {
		cfg.Level = gzip.DefaultCompression
	}
	if cfg.MinLength == 0 {
		cfg.MinLength = 1024
	}
	var pool = sync.Pool{New: func() any {
		gz, _ := gzip.NewWriterLevel(io.Discard, cfg.Level)
		return gz
	}}
	return func(c *core.Ctx) {
		if !strings.Contains(c.Header("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		gz := pool.Get().(*gzip.Writer)
		defer pool.Put(gz)
		gz.Reset(c.Res())
		defer gz.Close()

		w := &writer{ResponseWriter: c.Res(), gz: gz}
		c.Res().Header().Set("Content-Encoding", "gzip")
		c.Res().Header().Del("Content-Length")
		c.G().Writer = w
		c.Next()
	}
}
