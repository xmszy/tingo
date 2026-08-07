package tfilesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func init() {
	RegisterDriver("local", newLocalDisk)
}

// localDisk 本地文件系统磁盘。
type localDisk struct {
	name       string
	root       string // 绝对路径的根目录
	url        string // 公开 URL 前缀
	visibility string // public / private
}

func newLocalDisk(name string, dc DiskConfig) (Disk, error) {
	root, err := filepath.Abs(dc.Root)
	if err != nil {
		return nil, fmt.Errorf("local: 根目录 %q 解析失败: %w", dc.Root, err)
	}
	if fi, err := os.Stat(root); os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0755); err != nil {
			return nil, fmt.Errorf("local: 创建根目录 %q 失败: %w", root, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("local: 根目录 %q 访问失败: %w", root, err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("local: 根目录 %q 不是目录", root)
	}
	vis := dc.Visibility
	if vis == "" {
		vis = "public"
	}
	return &localDisk{
		name:       name,
		root:       root,
		url:        dc.URL,
		visibility: vis,
	}, nil
}

func (d *localDisk) mkdir(filePath string) error {
	dir := filepath.Dir(filePath)
	return os.MkdirAll(dir, 0755)
}

func (d *localDisk) fullPath(path string) (string, error) {
	return SandboxWrite(d.root, path)
}

// ── Disk 接口实现 ─────────────────────────────────────────────────

func (d *localDisk) Put(ctx context.Context, path string, contents []byte) error {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return err
	}
	if err := d.mkdir(fullPath); err != nil {
		return fmt.Errorf("local: 创建目录失败: %w", err)
	}
	perm := os.FileMode(0644)
	if d.visibility == "private" {
		perm = 0600
	}
	return os.WriteFile(fullPath, contents, perm)
}

func (d *localDisk) Get(ctx context.Context, path string) ([]byte, error) {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (d *localDisk) Delete(ctx context.Context, path string) error {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return err
	}
	// 忽略文件不存在错误。
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("local: 删除失败: %w", err)
	}
	return nil
}

func (d *localDisk) Exists(ctx context.Context, path string) bool {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(fullPath)
	return err == nil
}

func (d *localDisk) Size(ctx context.Context, path string) (int64, error) {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(fullPath)
	if err != nil {
		return 0, fmt.Errorf("local: stat 失败: %w", err)
	}
	return fi.Size(), nil
}

func (d *localDisk) MimeType(ctx context.Context, path string) string {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return "application/octet-stream"
	}
	buf := make([]byte, 512)
	f, err := os.Open(fullPath)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	n, _ := f.Read(buf)
	mt := "application/octet-stream"
	if n > 0 {
		mt = detectContentType(buf[:n])
	}
	// 如果嗅探仍为通用二进制，回退为扩展名映射。
	if mt == "application/octet-stream" {
		mt = MimeByExt(path)
	}
	return mt
}

func (d *localDisk) Copy(ctx context.Context, src, dst string) error {
	srcPath, err := d.fullPath(src)
	if err != nil {
		return err
	}
	dstPath, err := d.fullPath(dst)
	if err != nil {
		return err
	}
	return copyFile(srcPath, dstPath)
}

func (d *localDisk) Move(ctx context.Context, src, dst string) error {
	srcPath, err := d.fullPath(src)
	if err != nil {
		return err
	}
	dstPath, err := d.fullPath(dst)
	if err != nil {
		return err
	}
	if err := d.mkdir(dstPath); err != nil {
		return fmt.Errorf("local: 创建目标目录失败: %w", err)
	}
	return os.Rename(srcPath, dstPath)
}

func (d *localDisk) Path(ctx context.Context, path string) string {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return ""
	}
	return fullPath
}

func (d *localDisk) URL(ctx context.Context, path string) string {
	if d.url == "" {
		return ""
	}
	return d.url + "/" + path
}

func (d *localDisk) Append(ctx context.Context, path string, contents []byte) error {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("local: 追加写入失败: %w", err)
	}
	defer f.Close()
	_, err = f.Write(contents)
	return err
}

func (d *localDisk) Reader(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("local: 打开文件失败: %w", err)
	}
	return f, nil
}

type localWriter struct {
	*os.File
}

func (w *localWriter) Close() error {
	return w.File.Close()
}

func (d *localDisk) Writer(ctx context.Context, path string) (io.WriteCloser, error) {
	fullPath, err := d.fullPath(path)
	if err != nil {
		return nil, err
	}
	if err := d.mkdir(fullPath); err != nil {
		return nil, fmt.Errorf("local: 创建目录失败: %w", err)
	}
	perm := os.FileMode(0644)
	if d.visibility == "private" {
		perm = 0600
	}
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return nil, fmt.Errorf("local: 写入流创建失败: %w", err)
	}
	return &localWriter{f}, nil
}

// ── 辅助 ──────────────────────────────────────────────────────────

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("local: 打开源文件失败: %w", err)
	}
	defer s.Close()
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("local: 创建目标目录失败: %w", err)
	}
	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("local: 创建目标文件失败: %w", err)
	}
	defer d.Close()
	if _, err := io.Copy(d, s); err != nil {
		return fmt.Errorf("local: 复制失败: %w", err)
	}
	si, err := s.Stat()
	if err == nil {
		_ = os.Chmod(dst, si.Mode())
		_ = os.Chtimes(dst, time.Now(), si.ModTime())
	}
	return nil
}
