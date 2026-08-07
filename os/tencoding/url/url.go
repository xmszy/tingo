// Package url 提供 URL 编解码。
// 设计要点：
//   - 基于标准库 net/url，零外部依赖。
//   - 提供 URL 编码/解码、Query 构建/解析等便捷函数。
package url

import (
	"net/url"
	"strings"
)

// Encode URL 编码字符串。
func Encode(s string) string { return url.QueryEscape(s) }

// Decode URL 解码字符串。
func Decode(s string) (string, error) { return url.QueryUnescape(s) }

// MustDecode URL 解码，失败返回原字符串。
func MustDecode(s string) string { v, _ := Decode(s); return v }

// RawEncode 使用 RawQuery 方式编码（空格编码为 %20 而非 +）。
func RawEncode(s string) string { return url.PathEscape(s) }

// RawDecode RawQuery 方式解码。
func RawDecode(s string) (string, error) { return url.PathUnescape(s) }

// BuildQuery 将 map 构建为 URL query string。
func BuildQuery(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(params))
	for k, v := range params {
		pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(pairs, "&")
}

// ParseQuery 将 query string 解析为 map。
func ParseQuery(query string) (map[string]string, error) {
	values, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(values))
	for k, v := range values {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result, nil
}

// ParseURL 解析完整 URL，返回各部分。
func ParseURL(rawURL string) (*URLInfo, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &URLInfo{
		Scheme:   u.Scheme,
		Host:     u.Host,
		Path:     u.Path,
		Query:    u.RawQuery,
		Fragment: u.Fragment,
	}, nil
}

// URLInfo URL 解析结果。
type URLInfo struct {
	Scheme   string
	Host     string
	Path     string
	Query    string
	Fragment string
}
