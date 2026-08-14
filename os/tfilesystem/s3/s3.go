// Package s3 提供与 AWS S3 兼容的对象存储适配器（S3 / 阿里云 OSS / 腾讯云 COS 通用）。
//
// 仅依赖标准库（net/http + crypto/hmac/sha256），实现 AWS Signature V4 签名，
// 零外部 SDK 依赖，符合 tingo「自有基础、零 gogf 依赖」的约束。
//
// 适配器实现 tfilesystem.Disk 接口，通过 tfilesystem.RegisterDriver 注册后，
// 即可在 filesystem 配置中通过 type="s3" 启用（OSS/COS 仅 endpoint/region 不同）：
//
//	tfilesystem.RegisterDriver("s3", s3.NewDisk)
//	fs, _ := tfilesystem.New(tfilesystem.Config{
//	    Default: "oss",
//	    Disks: map[string]tfilesystem.DiskConfig{
//	        "oss": {Type: "s3", Bucket: "my-bucket", Region: "oss-cn-hangzhou",
//	                Endpoint: "https://oss-cn-hangzhou.aliyuncs.com",
//	                Key: "...", Secret: "...", URL: "https://cdn.example.com"},
//	    },
//	})
package s3

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xmszy/tingo/os/tfilesystem"
)

// Disk 是 S3 兼容存储的 Disk 实现。
type Disk struct {
	bucket   string
	region   string
	endpoint string // 形如 https://s3.amazonaws.com（不含桶名）
	key      string
	secret   string
	url      string // 公开访问 URL 前缀（可选）
	client   *http.Client
}

// NewDisk 实现 tfilesystem.Driver，供 RegisterDriver 注册。
// dc.Endpoint 指向服务域名（不含桶名），例如：
//   - S3:    https://s3.amazonaws.com
//   - OSS:   https://oss-cn-hangzhou.aliyuncs.com
//   - COS:   https://cos.ap-guangzhou.myqcloud.com
func NewDisk(name string, dc tfilesystem.DiskConfig) (tfilesystem.Disk, error) {
	if dc.Bucket == "" {
		return nil, fmt.Errorf("s3(%s): bucket 未配置", name)
	}
	if dc.Key == "" || dc.Secret == "" {
		return nil, fmt.Errorf("s3(%s): key/secret 未配置", name)
	}
	endpoint := strings.TrimRight(dc.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://s3.amazonaws.com"
	}
	return &Disk{
		bucket:   dc.Bucket,
		region:   dc.Region,
		endpoint: endpoint,
		key:      dc.Key,
		secret:   dc.Secret,
		url:      strings.TrimRight(dc.URL, "/"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// objectURL 返回对象的完整 URL（endpoint/bucket/key 形式）。
func (d *Disk) objectURL(path string) string {
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/%s/%s", d.endpoint, d.bucket, path)
}

// ── Disk 接口实现 ────────────────────────────────────────────────

func (d *Disk) Put(ctx context.Context, path string, contents []byte) error {
	return d.do(ctx, http.MethodPut, path, bytes.NewReader(contents), map[string]string{
		"Content-Type":   tfilesystem.MimeByExt(path),
		"Content-Length": fmt.Sprintf("%d", len(contents)),
	})
}

func (d *Disk) Get(ctx context.Context, path string) ([]byte, error) {
	status, body, err := d.request(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("s3: Get %q 返回 %d: %s", path, status, string(body))
	}
	return body, nil
}

func (d *Disk) Reader(ctx context.Context, path string) (io.ReadCloser, error) {
	req, err := d.buildRequest(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("s3: Reader %q 返回 %d: %s", path, resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (d *Disk) Delete(ctx context.Context, path string) error {
	status, body, err := d.request(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("s3: Delete %q 返回 %d: %s", path, status, string(body))
	}
	return nil
}

func (d *Disk) Exists(ctx context.Context, path string) bool {
	status, _, err := d.request(ctx, http.MethodHead, path, nil, nil)
	if err != nil {
		return false
	}
	return status == http.StatusOK
}

func (d *Disk) Size(ctx context.Context, path string) (int64, error) {
	req, err := d.buildRequest(ctx, http.MethodHead, path, nil, nil)
	if err != nil {
		return 0, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("s3: Size %q 返回 %d", path, resp.StatusCode)
	}
	return resp.ContentLength, nil
}

func (d *Disk) MimeType(ctx context.Context, path string) string {
	req, err := d.buildRequest(ctx, http.MethodHead, path, nil, nil)
	if err != nil {
		return ""
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		return ct
	}
	return tfilesystem.MimeByExt(path)
}

func (d *Disk) Copy(ctx context.Context, src, dst string) error {
	// S3 CopyObject：通过 x-amz-copy-source 头在服务端复制。
	copySrc := fmt.Sprintf("/%s/%s", d.bucket, strings.TrimLeft(src, "/"))
	status, body, err := d.request(ctx, http.MethodPut, dst, nil, map[string]string{
		"x-amz-copy-source": copySrc,
	})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("s3: Copy %q→%q 返回 %d: %s", src, dst, status, string(body))
	}
	return nil
}

func (d *Disk) Move(ctx context.Context, src, dst string) error {
	if err := d.Copy(ctx, src, dst); err != nil {
		return err
	}
	return d.Delete(ctx, src)
}

func (d *Disk) Path(ctx context.Context, path string) string {
	return d.objectURL(path)
}

func (d *Disk) URL(ctx context.Context, path string) string {
	if d.url != "" {
		return d.url + "/" + strings.TrimLeft(path, "/")
	}
	return d.objectURL(path)
}

func (d *Disk) Append(ctx context.Context, path string, contents []byte) error {
	// 对象存储不支持原地追加：读取原内容再合并重传。
	old, err := d.Get(ctx, path)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return err
	}
	merged := append(old, contents...)
	return d.Put(ctx, path, merged)
}

func (d *Disk) Writer(ctx context.Context, path string) (io.WriteCloser, error) {
	return &s3Writer{disk: d, ctx: ctx, path: path, buf: &bytes.Buffer{}}, nil
}

// s3Writer 是一个缓冲写入器，Close 时一次性 Put（对象存储不支持流式追加）。
type s3Writer struct {
	disk *Disk
	ctx  context.Context
	path string
	buf  *bytes.Buffer
}

func (w *s3Writer) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *s3Writer) Close() error {
	return w.disk.Put(w.ctx, w.path, w.buf.Bytes())
}

// ── 底层 HTTP + 签名 ─────────────────────────────────────────────

// do 执行一次带 body 的请求并返回错误（无 body 时 body 为 nil）。
func (d *Disk) do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) error {
	status, respBody, err := d.request(ctx, method, path, body, headers)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("s3: %s %q 返回 %d: %s", method, path, status, string(respBody))
}

// request 执行一次请求并返回 (status, body, err)。
func (d *Disk) request(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (int, []byte, error) {
	req, err := d.buildRequest(ctx, method, path, body, headers)
	if err != nil {
		return 0, nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// buildRequest 构造带 SigV4 签名的请求。
func (d *Disk) buildRequest(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Request, error) {
	url := d.objectURL(path)
	var payload []byte
	if body != nil {
		// 需要预先读取 body 以计算签名（对象存储通常体积小）。
		b, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		payload = b
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	host := hostOf(d.endpoint)

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", hashSHA256(payload))

	payloadHash := hashSHA256(payload)
	authorization := d.signV4(method, path, req.Header, payloadHash, amzDate, dateStamp)
	req.Header.Set("Authorization", authorization)

	return req, nil
}

// signV4 计算 AWS Signature V4 Authorization 头。
func (d *Disk) signV4(method string, path string, headers http.Header, payloadHash, amzDate, dateStamp string) string {
	// 1. Canonical Headers（按名排序）。
	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	canonicalHeaders := ""
	for _, h := range signedHeaders {
		canonicalHeaders += h + ":" + strings.TrimSpace(headers.Get(h)) + "\n"
	}

	// 2. Canonical Request。
	canonicalRequest := strings.Join([]string{
		method,
		"/" + d.bucket + "/" + strings.TrimLeft(path, "/"),
		"", // canonical query（本实现不传查询参数）
		canonicalHeaders,
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	// 3. String to Sign。
	scope := strings.Join([]string{dateStamp, d.region, "s3", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hashHex(canonicalRequest),
	}, "\n")

	// 4. Signing Key。
	signingKey := hmacSHA256([]byte("AWS4"+d.secret), dateStamp)
	signingKey = hmacSHA256(signingKey, d.region)
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")

	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		d.key, scope, strings.Join(signedHeaders, ";"), signature)
}

// ── 工具函数 ─────────────────────────────────────────────────────

func hostOf(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	if i := strings.Index(endpoint, "/"); i >= 0 {
		endpoint = endpoint[:i]
	}
	return endpoint
}

func hashSHA256(b []byte) string { return hashHex(string(b)) }

func hashHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}
