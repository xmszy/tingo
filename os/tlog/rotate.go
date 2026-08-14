package tlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RotatingWriter 是基于文件大小的轮转写入器（实现 io.WriteCloser）。
// 当当前日志文件写入后会超过 MaxSize 时，将其重命名为 .1、.2... 备份，并新建文件。
// 仅控制大小轮转；配合外部 logrotate/time-based 亦可。备份文件保留 MaxBackups 份（0=不限制）。
//
// 注意：进程重启后若当前文件已接近上限，仍会在下一次写入超阈值时轮转，不会丢失已有内容。
type RotatingWriter struct {
	mu         sync.Mutex
	path       string
	maxSize    int64
	maxBackups int
	file       *os.File
	size       int64
}

// NewRotatingWriter 创建轮转写入器。maxSize 为单文件上限（字节，<=0 表示 100MB），
// maxBackups 为保留的备份数（<=0 表示不限制）。
func NewRotatingWriter(path string, maxSize int64, maxBackups int) (*RotatingWriter, error) {
	if maxSize <= 0 {
		maxSize = 100 << 20 // 100MB
	}
	w := &RotatingWriter{path: path, maxSize: maxSize, maxBackups: maxBackups}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotatingWriter) open() error {
	if dir := filepath.Dir(w.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}

// Write 实现 io.Writer，必要时先轮转。
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, fmt.Errorf("tlog: rotating writer not opened")
	}
	if w.maxSize > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotateLocked 在持有 mu 时执行轮转（关闭当前、重命名备份、新建）。
func (w *RotatingWriter) rotateLocked() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
	}
	// 清理最旧的备份（若超过 maxBackups）。
	if w.maxBackups > 0 {
		oldest := fmt.Sprintf("%s.%d", w.path, w.maxBackups)
		_ = os.Remove(oldest)
	}
	// 从最大备份号向 1 顺移：.1 -> .2 ...
	for i := w.maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		_ = os.Rename(src, dst)
	}
	// 当前文件 -> .1
	_ = os.Rename(w.path, w.path+".1")
	return w.open()
}

// Close 关闭底层文件。
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}
