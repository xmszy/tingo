// Package upload 提供文件上传辅助。
//
// 设计：零外部依赖，基于标准库 mime/multipart。在 core.Ctx 已有 File/SaveFile
// 之上补充：大小上限、允许扩展名校验、批量保存。
package upload

import (
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/xmszy/tingo/core"
)

// 错误定义。
var (
	ErrTooLarge      = errors.New("file too large")
	ErrExtNotAllowed = errors.New("file extension not allowed")
)

// Config 是上传校验配置。
type Config struct {
	// MaxSize 单文件最大字节数，0 表示不限制。
	MaxSize int64
	// AllowExts 允许的扩展名（含点，如 ".png"）。空表示不限制。
	AllowExts []string
}

// Save 校验并保存单个上传文件到 dst，返回错误。
// 校验顺序：大小、扩展名。
func Save(c *core.Ctx, name, dst string, cfg Config) error {
	f, err := c.File(name)
	if err != nil {
		return err
	}
	if err := validate(f, cfg); err != nil {
		return err
	}
	return c.G().SaveUploadedFile(f, dst)
}

// SaveAll 保存同名的多个上传文件到 dir 目录（保留原名）。
// 任一文件校验失败会立即停止并返回已保存列表。
func SaveAll(c *core.Ctx, name, dir string, cfg Config) ([]string, error) {
	headers, err := c.Files(name)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(headers))
	for _, h := range headers {
		if err := validate(h, cfg); err != nil {
			return out, err
		}
		dst := filepath.Join(dir, h.Filename)
		if err := c.G().SaveUploadedFile(h, dst); err != nil {
			return out, err
		}
		out = append(out, dst)
	}
	return out, nil
}

func validate(f *multipart.FileHeader, cfg Config) error {
	if cfg.MaxSize > 0 && f.Size > cfg.MaxSize {
		return ErrTooLarge
	}
	if len(cfg.AllowExts) > 0 {
		ext := strings.ToLower(filepath.Ext(f.Filename))
		ok := false
		for _, e := range cfg.AllowExts {
			if strings.EqualFold(e, ext) {
				ok = true
				break
			}
		}
		if !ok {
			return ErrExtNotAllowed
		}
	}
	return nil
}

// 保留 net/http 引用。
var _ = http.StatusOK
