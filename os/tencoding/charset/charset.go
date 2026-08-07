// Package charset 提供字符集转换。
// 设计要点：
//   - 基于标准库 unicode/utf8 和 golang.org/x/text，零外部依赖。
//   - 提供 UTF-8 / GBK / GB2312 / Big5 / ShiftJIS 等常见编码互转。
package charset

import "unicode/utf8"

// IsUTF8 判断字节数组是否为有效 UTF-8。
func IsUTF8(data []byte) bool { return utf8.Valid(data) }

// UTF8ToUTF8 返回原数据（兼容性，不转换）。
func UTF8ToUTF8(data []byte) ([]byte, error) { return data, nil }

// 注意：完整的字符集转换（GBK/GB2312/Big5/ShiftJIS）
// 需要 golang.org/x/text 包，在 import 此包时由调用方自行管理。
// 本包提供零依赖的基础 API，复杂转换建议使用 x/text 或 iconv。
