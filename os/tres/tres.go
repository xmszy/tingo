// Package tres 提供资源管理。
// 设计要点：
//   - 基于标准库 embed，零外部依赖。
//   - 提供资源嵌入、列出、读取等工具。
package tres

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
)

// Manager 资源管理器。
type Manager struct {
	fs   embed.FS
	root string
}

// New 创建资源管理器。
func New(fs embed.FS, root ...string) *Manager {
	r := "."
	if len(root) > 0 {
		r = root[0]
	}
	return &Manager{fs: fs, root: strings.TrimRight(r, "/")}
}

// ReadFile 读取嵌入的文件内容。
func (m *Manager) ReadFile(name string) ([]byte, error) {
	return m.fs.ReadFile(filepath.Join(m.root, name))
}

// ReadString 读取嵌入的文件内容为字符串。
func (m *Manager) ReadString(name string) (string, error) {
	b, err := m.ReadFile(name)
	return string(b), err
}

// Exists 判断文件是否存在。
func (m *Manager) Exists(name string) bool {
	_, err := m.fs.Open(filepath.Join(m.root, name))
	return err == nil
}

// Walk 遍历所有嵌入文件。
func (m *Manager) Walk(fn func(path string, d fs.DirEntry) error) error {
	return fs.WalkDir(m.fs, m.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return fn(strings.TrimPrefix(path, m.root+"/"), d)
	})
}

// List 列出指定目录下的所有文件。
func (m *Manager) List(dir string) ([]string, error) {
	entries, err := m.fs.ReadDir(filepath.Join(m.root, dir))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
