// Package compress 提供数据压缩/解压。
// 设计要点：
//   - 基于标准库 compress/gzip 和 compress/zlib，零外部依赖。
package compress

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
)

// Gzip 使用 gzip 压缩数据。
func Gzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnGzip 解压 gzip 数据。
func UnGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Zlib 使用 zlib 压缩数据。
func Zlib(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnZlib 解压 zlib 数据。
func UnZlib(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
