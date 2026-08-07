package tres

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PackedResource 打包后的资源。
type PackedResource struct {
	name    string
	entries map[string][]byte
}

// Pack 将目录打包为内嵌资源。
//
// 可以将静态文件（HTML/CSS/JS/图片等）打包进二进制。
//
// 用法：
//
//	//go:embed static.zip
//	var staticZip []byte
//
//	res := tres.Unpack(staticZip)
//	content, ok := res.Get("index.html")
func Pack(dir string) (*PackedResource, error) {
	res := &PackedResource{
		name:    filepath.Base(dir),
		entries: make(map[string][]byte),
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// 统一使用正斜杠
		relPath = filepath.ToSlash(relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		res.entries[relPath] = data
		return nil
	})

	return res, err
}

// Unpack 从 zip 字节解析打包资源。
func Unpack(data []byte) (*PackedResource, error) {
	res := &PackedResource{
		entries: make(map[string][]byte),
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		res.entries[f.Name] = content
	}
	return res, nil
}

// PackToZip 将目录打包为 zip 字节。
func PackToZip(dir string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		f, err := w.Create(relPath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	})

	if err != nil {
		w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Get 获取资源内容。
func (r *PackedResource) Get(name string) ([]byte, bool) {
	name = strings.TrimPrefix(name, "/")
	data, ok := r.entries[name]
	return data, ok
}

// Has 检查资源是否存在。
func (r *PackedResource) Has(name string) bool {
	_, ok := r.entries[strings.TrimPrefix(name, "/")]
	return ok
}

// List 列出所有资源路径。
func (r *PackedResource) List() []string {
	paths := make([]string, 0, len(r.entries))
	for k := range r.entries {
		paths = append(paths, k)
	}
	return paths
}

// Size 返回打包资源的总大小。
func (r *PackedResource) Size() int {
	total := 0
	for _, data := range r.entries {
		total += len(data)
	}
	return total
}
