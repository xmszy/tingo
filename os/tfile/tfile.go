// Package tfile 提供文件操作工具。
// 设计要点：
//   - 基于标准库 os/io/path/filepath，零外部依赖。
//   - 提供常用文件操作封装。
package tfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ──────────────── 路径 ────────────────

// Exists 判断文件/目录是否存在。
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

// IsDir 判断是否为目录。
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsFile 判断是否为普通文件。
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Basename 返回路径的文件名部分。
func Basename(path string) string { return filepath.Base(path) }

// Dirname 返回路径的目录部分。
func Dirname(path string) string { return filepath.Dir(path) }

// Ext 返回文件扩展名。
func Ext(path string) string { return filepath.Ext(path) }

// Name 返回不带扩展名的文件名。
func Name(path string) string {
	full := filepath.Base(path)
	ext := filepath.Ext(full)
	return strings.TrimSuffix(full, ext)
}

// Abs 返回绝对路径。
func Abs(path string) (string, error) { return filepath.Abs(path) }

// Realpath 返回规范化的绝对路径（解析符号链接）。
func Realpath(path string) (string, error) { return filepath.EvalSymlinks(path) }

// Join 拼接路径。
func Join(elem ...string) string { return filepath.Join(elem...) }

// ──────────────── 读取 ────────────────

// ReadFile 读取整个文件内容为字符串。
func ReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// ReadBytes 读取整个文件内容为字节数组。
func ReadBytes(path string) ([]byte, error) { return os.ReadFile(path) }

// ReadLines 读取文件所有行的字符串数组。
func ReadLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), nil
}

// ──────────────── 写入 ────────────────

// PutContents 写入字符串到文件（覆盖）。
func PutContents(path, data string) error {
	return os.WriteFile(path, []byte(data), 0o644)
}

// PutBytes 写入字节数组到文件（覆盖）。
func PutBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// AppendContents 追加字符串到文件。
func AppendContents(path, data string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(data)
	return err
}

// ──────────────── 文件属性 ────────────────

// Size 返回文件大小（字节）。
func Size(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// MTime 返回文件修改时间。
func MTime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

// Perm 返回文件权限。
func Perm(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(info.Mode().Perm())
}

// ──────────────── 操作 ────────────────

// Mkdir 创建目录（含父目录）。
func Mkdir(path string) error { return os.MkdirAll(path, 0o755) }

// Remove 删除文件。
func Remove(path string) error { return os.Remove(path) }

// RemoveAll 递归删除目录树。
func RemoveAll(path string) error { return os.RemoveAll(path) }

// Rename 重命名/移动文件。
func Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

// Copy 复制文件。
func Copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	// 确保目标目录存在
	if err := Mkdir(filepath.Dir(dst)); err != nil {
		return err
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// Chmod 修改文件权限。
func Chmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }

// ──────────────── 扫描 ────────────────

// Glob 返回匹配模式的文件列表。
func Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }

// ScanDir 扫描目录下的文件和子目录。
func ScanDir(path string, pattern ...string) ([]string, error) {
	pat := "*"
	if len(pattern) > 0 {
		pat = pattern[0]
	}
	return filepath.Glob(filepath.Join(path, pat))
}

// ScanDirRecursive 递归扫描目录下的所有文件。
func ScanDirRecursive(path string, pattern ...string) ([]string, error) {
	pat := "*"
	if len(pattern) > 0 {
		pat = pattern[0]
	}
	var files []string
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if match, _ := filepath.Match(pat, filepath.Base(p)); match {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

// ──────────────── 临时文件 ────────────────

// TempDir 创建临时目录。
func TempDir(dir, prefix string) (string, error) { return os.MkdirTemp(dir, prefix) }

// TempFile 创建临时文件。
func TempFile(dir, prefix string) (*os.File, error) { return os.CreateTemp(dir, prefix) }

// ──────────────── 用户目录 ────────────────

// Home 返回当前用户的主目录。
func Home() (string, error) { return os.UserHomeDir() }

// HomeDir 返回用户主目录，获取失败返回工作目录。
func HomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		h, _ = os.Getwd()
	}
	return h
}

// ──────────────── 文件大小格式化 ────────────────

// FormatSize 将字节数格式化为人类可读的大小字符串。
func FormatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	unit := []string{"KB", "MB", "GB", "TB", "PB"}
	val := float64(size)
	for _, u := range unit {
		val /= 1024
		if val < 1024 || u == "PB" {
			if val < 10 {
				return fmt.Sprintf("%.1f %s", val, u)
			}
			return fmt.Sprintf("%.0f %s", val, u)
		}
	}
	return fmt.Sprintf("%d B", size)
}

// ReadableSize 读取文件并返回人类可读的大小。
func ReadableSize(path string) string {
	return FormatSize(Size(path))
}

// ──────────────── 内容替换 ────────────────

// ReplaceInFile 在文件中执行正则替换。
// 读取文件 → 替换 → 写回（原地）。
func ReplaceInFile(path, old, new string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	re, err := regexp.Compile(old)
	if err != nil {
		return err
	}
	result := re.ReplaceAll(content, []byte(new))
	if string(result) == string(content) {
		return nil
	}
	return os.WriteFile(path, result, 0o644)
}

// ReplaceStrInFile 在文件中执行纯字符串替换。
func ReplaceStrInFile(path, old, new string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	result := strings.ReplaceAll(string(content), old, new)
	if result == string(content) {
		return nil
	}
	return os.WriteFile(path, []byte(result), 0o644)
}

// ──────────────── 文件排序 ────────────────

// FileSortBy 文件排序方式。
type FileSortBy int

const (
	SortByName    FileSortBy = iota // 按名称升序
	SortByTime                      // 按修改时间升序
	SortBySize                      // 按文件大小升序
)

// SortFiles 对文件列表排序。
// desc: true 表示降序。
func SortFiles(files []string, by FileSortBy, desc bool) {
	switch by {
	case SortByTime:
		sort.Slice(files, func(i, j int) bool {
			t1 := MTime(files[i])
			t2 := MTime(files[j])
			if desc {
				return t1 > t2
			}
			return t1 < t2
		})
	case SortBySize:
		sort.Slice(files, func(i, j int) bool {
			s1 := Size(files[i])
			s2 := Size(files[j])
			if desc {
				return s1 > s2
			}
			return s1 < s2
		})
	default: // SortByName
		sort.Slice(files, func(i, j int) bool {
			if desc {
				return files[i] > files[j]
			}
			return files[i] < files[j]
		})
	}
}
