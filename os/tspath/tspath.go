// Package tspath 提供路径搜索工具。
//
// 设计要点：
//   - 基于标准库 os/path/filepath，零外部依赖。
//   - Search 在多个目录中查找文件（类似 $PATH 搜索）。
//   - 常用于框架查找配置文件、模板文件、资源文件等。
package tspath

import (
	"os"
	"path/filepath"
	"strings"
)

// Search 在指定路径列表中搜索文件，返回第一个匹配的完整路径。
// paths 为搜索目录列表，name 为文件名（支持相对路径）。
// 搜索顺序：先匹配 name 原样，再逐前缀 path 拼接。
func Search(paths []string, name string) (string, error) {
	for _, p := range paths {
		full := filepath.Join(p, name)
		if _, err := os.Stat(full); err == nil {
			return full, nil
		}
	}
	return "", os.ErrNotExist
}

// SearchRealpath 同 Search，但返回绝对路径。
func SearchRealpath(paths []string, name string) (string, error) {
	p, err := Search(paths, name)
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

// SearchGlob 在指定路径列表中搜索匹配 glob 模式的第一个文件。
func SearchGlob(paths []string, pattern string) (string, error) {
	for _, p := range paths {
		full := filepath.Join(p, pattern)
		matches, err := filepath.Glob(full)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
	}
	return "", os.ErrNotExist
}

// IsAbs 判断路径是否为绝对路径。
func IsAbs(path string) bool { return filepath.IsAbs(path) }

// Split 拆分路径为目录和文件名两部分。
// 例: "/foo/bar.txt" → ("/foo", "bar.txt")
func Split(path string) (dir, name string) {
	return filepath.Split(path)
}

// Clean 清理路径中的冗余分隔符和 . ..
func Clean(path string) string { return filepath.Clean(path) }

// ExtName 返回文件扩展名（小写）。
func ExtName(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// ──────────────── 常用路径 ────────────────

// Home 返回用户主目录。
func Home() (string, error) { return os.UserHomeDir() }

// HomeOrPwd 返回用户主目录，失败返回当前工作目录。
func HomeOrPwd() string {
	h, err := os.UserHomeDir()
	if err != nil {
		h, _ = os.Getwd()
	}
	return h
}

// WorkDir 返回当前工作目录。
func WorkDir() (string, error) { return os.Getwd() }

// ExeDir 返回可执行文件所在目录。
func ExeDir() string {
	p, _ := os.Executable()
	return filepath.Dir(p)
}
