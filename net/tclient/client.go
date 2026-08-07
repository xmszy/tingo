// Package tclient 提供 HTTP 客户端封装。
// 设计要点：
//   - 基于标准库 net/http，零外部依赖。
//   - 支持链式配置、中间件链、重试。
//   - 提供 Response/Bytes/Content 三级 API，用户按需选择。
package tclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client HTTP 客户端。
type Client struct {
	hc              *http.Client
	header          http.Header
	retryCount      int
	retryInterval   time.Duration
	middlewareChain []Middleware
	baseURL         string
}

// Middleware 客户端中间件。
// 每层中间件接收 *Client 和 *http.Request，返回 *Response 和 error。
type Middleware func(c *Client, r *http.Request, next func(r *http.Request) (*Response, error)) (*Response, error)

// Response HTTP 响应封装。
type Response struct {
	*http.Response
	BodyBytes []byte
}

// Option 函数式配置选项。
type Option func(c *Client)

// ──────────────── 构造与配置 ────────────────

// New 创建 HTTP 客户端。
func New(opts ...Option) *Client {
	c := &Client{
		hc: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		header:        make(http.Header),
		retryInterval: time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Clone 克隆客户端（共享 http.Client，独立 header/middleware）。
func (c *Client) Clone() *Client {
	hdr := make(http.Header, len(c.header))
	for k, v := range c.header {
		hdr[k] = append([]string{}, v...)
	}
	mChain := make([]Middleware, len(c.middlewareChain))
	copy(mChain, c.middlewareChain)
	return &Client{
		hc:              c.hc,
		header:          hdr,
		retryCount:      c.retryCount,
		retryInterval:   c.retryInterval,
		middlewareChain: mChain,
		baseURL:         c.baseURL,
	}
}

func WithTimeout(d time.Duration) Option        { return func(c *Client) { c.hc.Timeout = d } }
func WithRetry(count int, interval time.Duration) Option {
	return func(c *Client) { c.retryCount = count; c.retryInterval = interval }
}
func WithHeader(key, value string) Option     { return func(c *Client) { c.header.Set(key, value) } }
func WithHeaders(h http.Header) Option        { return func(c *Client) { c.header = h.Clone() } }
func WithBaseURL(base string) Option          { return func(c *Client) { c.baseURL = strings.TrimRight(base, "/") } }
func WithTLSInsecure() Option                 { return func(c *Client) { c.setTLSInsecure() } }
func WithMiddleware(m ...Middleware) Option   { return func(c *Client) { c.middlewareChain = append(c.middlewareChain, m...) } }

func (c *Client) setTLSInsecure() {
	if t, ok := c.hc.Transport.(*http.Transport); ok {
		c2 := t.Clone()
		if c2.TLSClientConfig == nil {
			c2.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			c2.TLSClientConfig = c2.TLSClientConfig.Clone()
			c2.TLSClientConfig.InsecureSkipVerify = true
		}
		c.hc.Transport = c2
	}
}

// ──────────────── 链式配置 ────────────────

func (c *Client) SetTimeout(d time.Duration) *Client                 { c.hc.Timeout = d; return c }
func (c *Client) SetRetry(count int, interval time.Duration) *Client  { c.retryCount = count; c.retryInterval = interval; return c }
func (c *Client) SetHeader(k, v string) *Client                      { c.header.Set(k, v); return c }
func (c *Client) SetHeaders(h http.Header) *Client                   { c.header = h.Clone(); return c }
func (c *Client) SetBaseURL(base string) *Client                     { c.baseURL = strings.TrimRight(base, "/"); return c }
func (c *Client) Use(m ...Middleware) *Client                        { c.middlewareChain = append(c.middlewareChain, m...); return c }

// ──────────────── Response API ────────────────

// DoRequest 执行 HTTP 请求，返回完整 Response（需调用方释放 Body）。
func (c *Client) DoRequest(ctx context.Context, method, url string, body io.Reader, opts ...RequestOption) (*Response, error) {
	req, err := c.buildRequest(ctx, method, url, body, opts)
	if err != nil {
		return nil, err
	}
	return c.doWithMiddleware(req)
}

// Get / Post / Put / Delete / Patch / Head / Options 简洁方法。
func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodGet, url, nil, opts...)
}
func (c *Client) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodPost, url, body, opts...)
}
func (c *Client) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodPut, url, body, opts...)
}
func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodDelete, url, nil, opts...)
}
func (c *Client) Patch(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodPatch, url, body, opts...)
}
func (c *Client) Head(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodHead, url, nil, opts...)
}
func (c *Client) Options(ctx context.Context, url string, opts ...RequestOption) (*Response, error) {
	return c.DoRequest(ctx, http.MethodOptions, url, nil, opts...)
}

// ──────────────── Bytes API（自动关闭 Body，返回 []byte） ────────────────

// RequestBytes 发送请求并返回响应体字节。
func (c *Client) RequestBytes(ctx context.Context, method, url string, body io.Reader, opts ...RequestOption) ([]byte, error) {
	resp, err := c.DoRequest(ctx, method, url, body, opts...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) GetBytes(ctx context.Context, url string, opts ...RequestOption) ([]byte, error) {
	return c.RequestBytes(ctx, http.MethodGet, url, nil, opts...)
}
func (c *Client) PostBytes(ctx context.Context, url string, body io.Reader, opts ...RequestOption) ([]byte, error) {
	return c.RequestBytes(ctx, http.MethodPost, url, body, opts...)
}

// ──────────────── Content API（自动关闭，返回 string） ────────────────

func (c *Client) RequestContent(ctx context.Context, method, url string, body io.Reader, opts ...RequestOption) (string, error) {
	bs, err := c.RequestBytes(ctx, method, url, body, opts...)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func (c *Client) GetContent(ctx context.Context, url string, opts ...RequestOption) (string, error) {
	return c.RequestContent(ctx, http.MethodGet, url, nil, opts...)
}
func (c *Client) PostContent(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (string, error) {
	return c.RequestContent(ctx, http.MethodPost, url, body, opts...)
}

// ──────────────── JSON API ────────────────

// PostJSON 以 JSON 格式发送请求并获取响应。
func (c *Client) PostJSON(ctx context.Context, url string, payload any, opts ...RequestOption) (*Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("tclient: json marshal: %w", err)
	}
	opts = append(opts, WithContentType("application/json"))
	return c.DoRequest(ctx, http.MethodPost, url, bytes.NewReader(data), opts...)
}

// PostJSONInto 发送 JSON 请求，将响应反序列化到 dest。
func (c *Client) PostJSONInto(ctx context.Context, url string, payload any, dest any, opts ...RequestOption) error {
	resp, err := c.PostJSON(ctx, url, payload, opts...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dest)
}

// GetJSON 发送 GET 请求并将响应反序列化到 dest。
func (c *Client) GetJSON(ctx context.Context, url string, dest any, opts ...RequestOption) error {
	resp, err := c.Get(ctx, url, opts...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dest)
}

// ──────────────── XML API ────────────────

// PostXML 以 XML 格式发送请求。
func (c *Client) PostXML(ctx context.Context, url string, payload any, opts ...RequestOption) (*Response, error) {
	data, err := xml.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("tclient: xml marshal: %w", err)
	}
	opts = append(opts, WithContentType("application/xml"))
	return c.DoRequest(ctx, http.MethodPost, url, bytes.NewReader(data), opts...)
}

// ──────────────── Form 表单 ────────────────

// PostForm 发送 x-www-form-urlencoded 表单。
func (c *Client) PostForm(ctx context.Context, url string, data map[string]string, opts ...RequestOption) (*Response, error) {
	opts = append(opts, WithContentType("application/x-www-form-urlencoded"))
	vals := make([]string, 0, len(data)*2)
	for k, v := range data {
		vals = append(vals, k, v)
	}
	rd := strings.NewReader(buildQuery(vals))
	return c.DoRequest(ctx, http.MethodPost, url, rd, opts...)
}

// PostMultipart 发送 multipart/form-data。
func (c *Client) PostMultipart(ctx context.Context, url string, fields map[string]string, files map[string]string, opts ...RequestOption) (*Response, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	for fieldName, filePath := range files {
		part, err := w.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			return nil, err
		}
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(part, f); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
	}
	w.Close()
	opts = append(opts, WithContentType(w.FormDataContentType()))
	return c.DoRequest(ctx, http.MethodPost, url, &buf, opts...)
}

// ──────────────── 请求构建 ────────────────

func (c *Client) buildRequest(ctx context.Context, method, url string, body io.Reader, opts []RequestOption) (*http.Request, error) {
	fullURL := c.fullURL(url)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("tclient: build request: %w", err)
	}

	// 合并 header：先写 Client header，再写 request-level header。
	for k, vv := range c.header {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	ro := requestOptions{}
	for _, opt := range opts {
		opt(&ro)
	}
	for k, vv := range ro.headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	if ro.host != "" {
		req.Host = ro.host
	}
	return req, nil
}

func (c *Client) fullURL(path string) string {
	if c.baseURL == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return c.baseURL + "/" + strings.TrimLeft(path, "/")
}

// ──────────────── 中间件执行 ────────────────

func (c *Client) doWithMiddleware(req *http.Request) (*Response, error) {
	if len(c.middlewareChain) == 0 {
		return c.callRequest(req)
	}

	// 构建洋葱链：最后一层 = 真实请求。
	chain := func(r *http.Request) (*Response, error) {
		return c.callRequest(r)
	}
	for i := len(c.middlewareChain) - 1; i >= 0; i-- {
		m := c.middlewareChain[i]
		next := chain
		chain = func(r *http.Request) (*Response, error) {
			return m(c, r, next)
		}
	}
	return chain(req)
}

// callRequest 执行 HTTP 请求，含重试逻辑。
func (c *Client) callRequest(req *http.Request) (*Response, error) {
	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; ; attempt++ {
		resp, err = c.hc.Do(req)
		if err == nil {
			resp2 := &Response{Response: resp}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("tclient: read response body: %w", readErr)
			}
			resp2.BodyBytes = body
			resp2.Body = io.NopCloser(bytes.NewReader(body))
			return resp2, nil
		}
		if attempt < c.retryCount {
			time.Sleep(c.retryInterval)
		} else {
			break
		}
	}
	return nil, fmt.Errorf("tclient: %w", err)
}

// ──────────────── 默认全局客户端 ────────────────

var defaultClient = New()

// Default 返回默认客户端。
func Default() *Client { return defaultClient }

// ──────────────── 辅助 ────────────────

// RequestOption 单次请求级选项。
type RequestOption func(o *requestOptions)

type requestOptions struct {
	headers http.Header
	host    string
}

// WithContentType 设置 Content-Type header。
func WithContentType(ct string) RequestOption {
	return func(o *requestOptions) {
		if o.headers == nil {
			o.headers = make(http.Header)
		}
		o.headers.Set("Content-Type", ct)
	}
}

// WithReqHeader 设置请求级 header。
func WithReqHeader(k, v string) RequestOption {
	return func(o *requestOptions) {
		if o.headers == nil {
			o.headers = make(http.Header)
		}
		o.headers.Set(k, v)
	}
}

// WithHost 设置请求 Host。
func WithHost(host string) RequestOption {
	return func(o *requestOptions) { o.host = host }
}

// buildQuery 构建 query string（零分配版）。
func buildQuery(pairs []string) string {
	if len(pairs) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(urlEncode(pairs[i]))
		b.WriteByte('=')
		b.WriteString(urlEncode(pairs[i+1]))
	}
	return b.String()
}

func urlEncode(s string) string {
	// 避免引入 net/url 的分配，仅对常见特殊字符编码
	var b strings.Builder
	b.Grow(len(s))
	for _, ch := range s {
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			b.WriteByte(byte(ch))
		case ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(byte(ch))
		case ch == ' ':
			b.WriteByte('+')
		default:
			b.WriteString(fmt.Sprintf("%%%02X", ch))
		}
	}
	return b.String()
}

// ──────────────── Response 辅助 ────────────────

// String 返回响应体字符串。
func (r *Response) String() string { return string(r.BodyBytes) }

// JSON 反序列化响应体到 v。
func (r *Response) JSON(v any) error { return json.Unmarshal(r.BodyBytes, v) }

// XML 反序列化响应体到 v。
func (r *Response) XML(v any) error { return xml.Unmarshal(r.BodyBytes, v) }

// Close 关闭响应。
func (r *Response) Close() error {
	r.BodyBytes = nil
	if r.Response != nil && r.Response.Body != nil {
		return r.Response.Body.Close()
	}
	return nil
}
