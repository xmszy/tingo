// Package tfilesystem 提供多磁盘文件系统抽象。
//
// 支持多种存储后端（本地磁盘 / S3 / OSS / COS 等），
// 通过统一 Disk 接口操作文件，磁盘切换零 API 变化。
//
// 用法：
//
//	fs := tfilesystem.New(conf)
//	err := fs.Disk("public").Put(ctx, "avatar.png", data)
//	url := fs.Disk("public").URL(ctx, "avatar.png")
package tfilesystem

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Disk 存储磁盘接口。
//
// 每个磁盘对应一种存储后端（本地目录 / S3 / OSS 等），
// 对外暴露统一的文件 CRUD + 元数据方法。
type Disk interface {
	// Put 写入文件。若目录不存在则自动创建；若文件已存在则覆盖。
	Put(ctx context.Context, path string, contents []byte) error

	// Get 读取文件全部内容。
	Get(ctx context.Context, path string) ([]byte, error)

	// Delete 删除文件。
	Delete(ctx context.Context, path string) error

	// Exists 判断文件是否存在。
	Exists(ctx context.Context, path string) bool

	// Size 返回文件大小（字节）。
	Size(ctx context.Context, path string) (int64, error)

	// MimeType 返回文件的 MIME 类型。
	MimeType(ctx context.Context, path string) string

	// Copy 复制文件。
	Copy(ctx context.Context, src, dst string) error

	// Move 移动/重命名文件。
	Move(ctx context.Context, src, dst string) error

	// Path 返回文件的绝对路径（本地磁盘）或 URI（远程磁盘）。
	Path(ctx context.Context, path string) string

	// URL 返回文件的公开访问 URL。
	URL(ctx context.Context, path string) string

	// Append 在文件末尾追加内容。
	Append(ctx context.Context, path string, contents []byte) error

	// Reader 返回文件读取流。
	Reader(ctx context.Context, path string) (io.ReadCloser, error)

	// Writer 返回文件写入流。若文件已存在则覆盖。
	Writer(ctx context.Context, path string) (io.WriteCloser, error)
}

// DiskConfig 单个磁盘的配置。
type DiskConfig struct {
	Type       string `toml:"type"`       // 驱动类型：local / s3 / oss / cos
	Root       string `toml:"root"`       // 根目录（本地磁盘）
	URL        string `toml:"url"`        // 公开访问 URL 前缀
	Visibility string `toml:"visibility"` // 可见性：public / private
	Region     string `toml:"region"`     // 区域（S3/OSS/COS）
	Bucket     string `toml:"bucket"`     // 存储桶
	Endpoint   string `toml:"endpoint"`   // 自定义 endpoint
	Key        string `toml:"key"`        // Access Key
	Secret     string `toml:"secret"`     // Secret Key
}

// Config 文件系统配置（对应 config/filesystem.toml）。
type Config struct {
	Default string                `toml:"default"` // 默认磁盘名称
	Disks   map[string]DiskConfig `toml:"disks"`   // 磁盘列表
}

// Driver 磁盘驱动工厂。
//
// name 为磁盘名称，dc 为该磁盘的配置。
type Driver func(name string, dc DiskConfig) (Disk, error)

// registeredDrivers 已注册的驱动工厂。
var registeredDrivers = map[string]Driver{}

// RegisterDriver 注册磁盘驱动。
//
// 内置驱动（local）已自动注册；扩展驱动（s3/oss/cos）
// 可在 contrib 或用户代码中通过此函数注册。
func RegisterDriver(driverType string, fn Driver) {
	if driverType == "" || fn == nil {
		panic("tfilesystem: RegisterDriver 参数为空")
	}
	registeredDrivers[driverType] = fn
}

// Managers 返回所有已注册的驱动类型名称。
func Drivers() []string {
	names := make([]string, 0, len(registeredDrivers))
	for k := range registeredDrivers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Manager 多磁盘文件系统管理器。
//
// 通过配置实例化后，用 Disk(name) 选择要操作的磁盘。
// Manager 实例是并发安全的（创建后不可变）。
type Manager struct {
	defaultDisk string
	disks       map[string]Disk
}

// New 根据配置创建 Manager。
//
// 内部调用每个磁盘对应的 RegisterDriver 工厂。
func New(cfg Config) (*Manager, error) {
	if cfg.Disks == nil {
		return nil, fmt.Errorf("tfilesystem: 未配置磁盘")
	}
	m := &Manager{
		defaultDisk: cfg.Default,
		disks:       make(map[string]Disk, len(cfg.Disks)),
	}
	if m.defaultDisk == "" {
		// 取字典序第一个作为默认磁盘。
		names := make([]string, 0, len(cfg.Disks))
		for n := range cfg.Disks {
			names = append(names, n)
		}
		sort.Strings(names)
		m.defaultDisk = names[0]
	}
	for name, dc := range cfg.Disks {
		driver, ok := registeredDrivers[dc.Type]
		if !ok {
			return nil, fmt.Errorf("tfilesystem: 未知驱动类型 %q（磁盘 %q）", dc.Type, name)
		}
		disk, err := driver(name, dc)
		if err != nil {
			return nil, fmt.Errorf("tfilesystem: 创建磁盘 %q 失败: %w", name, err)
		}
		m.disks[name] = disk
	}
	if _, ok := m.disks[m.defaultDisk]; !ok {
		return nil, fmt.Errorf("tfilesystem: 默认磁盘 %q 未定义", m.defaultDisk)
	}
	return m, nil
}

// Disk 获取磁盘实例。name 为空时返回默认磁盘。
func (m *Manager) Disk(name string) Disk {
	if name == "" {
		name = m.defaultDisk
	}
	return m.disks[name]
}

// DefaultDisk 返回默认磁盘名称。
func (m *Manager) DefaultDisk() string {
	return m.defaultDisk
}

// ── 便捷方法（直接操作默认磁盘）───────────────────────────────────

func (m *Manager) Put(ctx context.Context, path string, contents []byte) error {
	return m.Disk("").Put(ctx, path, contents)
}
func (m *Manager) Get(ctx context.Context, path string) ([]byte, error) {
	return m.Disk("").Get(ctx, path)
}
func (m *Manager) Delete(ctx context.Context, path string) error {
	return m.Disk("").Delete(ctx, path)
}
func (m *Manager) Exists(ctx context.Context, path string) bool {
	return m.Disk("").Exists(ctx, path)
}
func (m *Manager) Size(ctx context.Context, path string) (int64, error) {
	return m.Disk("").Size(ctx, path)
}
func (m *Manager) MimeType(ctx context.Context, path string) string {
	return m.Disk("").MimeType(ctx, path)
}
func (m *Manager) Copy(ctx context.Context, src, dst string) error {
	return m.Disk("").Copy(ctx, src, dst)
}
func (m *Manager) Move(ctx context.Context, src, dst string) error {
	return m.Disk("").Move(ctx, src, dst)
}
func (m *Manager) Path(ctx context.Context, path string) string {
	return m.Disk("").Path(ctx, path)
}
func (m *Manager) URL(ctx context.Context, path string) string {
	return m.Disk("").URL(ctx, path)
}
func (m *Manager) Append(ctx context.Context, path string, contents []byte) error {
	return m.Disk("").Append(ctx, path, contents)
}
func (m *Manager) Reader(ctx context.Context, path string) (io.ReadCloser, error) {
	return m.Disk("").Reader(ctx, path)
}
func (m *Manager) Writer(ctx context.Context, path string) (io.WriteCloser, error) {
	return m.Disk("").Writer(ctx, path)
}

// ── MIME 类型推断 ─────────────────────────────────────────────────

// mimeTypes 是扩展名到 MIME 类型的映射（覆盖常见类型）。
var mimeTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".htm":   "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "application/javascript; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".xml":   "application/xml; charset=utf-8",
	".txt":   "text/plain; charset=utf-8",
	".csv":   "text/csv; charset=utf-8",
	".md":    "text/markdown; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".bmp":   "image/bmp",
	".tiff":  "image/tiff",
	".mp3":   "audio/mpeg",
	".wav":   "audio/wav",
	".ogg":   "audio/ogg",
	".mp4":   "video/mp4",
	".webm":  "video/webm",
	".avi":   "video/x-msvideo",
	".pdf":   "application/pdf",
	".zip":   "application/zip",
	".gz":    "application/gzip",
	".tar":   "application/x-tar",
	".7z":    "application/x-7z-compressed",
	".doc":   "application/msword",
	".docx":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":   "application/vnd.ms-excel",
	".xlsx":  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":   "application/vnd.ms-powerpoint",
	".pptx":  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".ttf":   "font/ttf",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".eot":   "application/vnd.ms-fontobject",
}

// MimeByExt 根据文件扩展名返回 MIME 类型。
//
// 未知扩展名返回 "application/octet-stream"。
func MimeByExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}

// detectContentType 通过文件内容嗅探 MIME 类型。
func detectContentType(data []byte) string {
	return http.DetectContentType(data)
}

// ── 安全检查 ──────────────────────────────────────────────────────

// SandboxWrite 检查目标路径是否在根目录内，防止路径穿越。
//
// 返回安全的绝对路径；若穿越/空路径/绝对路径则返回 error。
func SandboxWrite(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("tfilesystem: 文件路径为空")
	}
	// 清理路径。
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." {
		return "", fmt.Errorf("tfilesystem: 非法文件路径 %q", rel)
	}
	// 将根目录也清理后拼接。
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("tfilesystem: 根目录解析失败: %w", err)
	}
	absPath, err := filepath.Abs(filepath.Join(absRoot, clean))
	if err != nil {
		return "", fmt.Errorf("tfilesystem: 文件路径解析失败: %w", err)
	}
	if !strings.HasPrefix(absPath, absRoot+string(os.PathSeparator)) && absPath != absRoot {
		return "", fmt.Errorf("tfilesystem: 非法文件路径 %q（超出根目录）", rel)
	}
	return absPath, nil
}
