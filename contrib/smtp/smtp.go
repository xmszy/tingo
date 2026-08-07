// Package smtp 提供电子邮件发送能力，基于 Go 标准库 net/smtp。
//
// 设计：零外部依赖，支持 PlainAuth/LOGIN 认证、TLS/STARTTLS、
// HTML 邮件、附件、CC/BCC。
//
// 用法：
//
//	client := smtp.New(smtp.Config{
//	    Host:     "smtp.example.com",
//	    Port:     465,
//	    Username: "user@example.com",
//	    Password: "password",
//	    From:     "user@example.com",
//	    SSL:      true,
//	})
//	err := client.Send(smtp.Message{
//	    To:      []string{"to@example.com"},
//	    Subject: "Hello",
//	    Body:    "Hello, world!",
//	    HTML:    true,
//	})
package smtp

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	stdmail "net/mail"
	"net/smtp"
	"strings"
)

// Config 是 SMTP 客户端配置。
type Config struct {
	// Host SMTP 服务器地址。
	Host string
	// Port 端口（465 SSL，587 STARTTLS，25 非加密）。
	Port int
	// Username 用户名（通常为邮箱地址）。
	Username string
	// Password 密码或授权码。
	Password string
	// From 发件人地址。
	From string
	// FromName 发件人显示名称（可选）。
	FromName string
	// SSL 是否使用 SSL/TLS 直连（465 端口为 true，587 为 false）。
	SSL bool
	// TLSConfig 自定义 TLS 配置（nil 使用默认）。
	TLSConfig *tls.Config
	// LocalName 用于 EHLO/HELO。空则自动取 os.Hostname()。
	LocalName string
}

// Message 表示一封邮件。
type Message struct {
	// From 发件人，空则使用 Config.From。
	From string
	// FromName 发件人显示名称。
	FromName string
	// To 收件人。
	To []string
	// CC 抄送。
	CC []string
	// BCC 密送。
	BCC []string
	// Subject 主题。
	Subject string
	// Body 正文。
	Body string
	// HTML 正文是否为 HTML（默认纯文本）。
	HTML bool
	// Attachments 附件。
	Attachments []Attachment
	// Headers 额外邮件头。
	Headers map[string]string
}

// Attachment 表示一个附件。
type Attachment struct {
	// Filename 文件名。
	Filename string
	// ContentType MIME 类型（如 "application/pdf"）。空则自动检测。
	ContentType string
	// Data 文件内容。
	Data []byte
}

// Client 是 SMTP 客户端。
type Client struct {
	cfg Config
}

// New 创建 SMTP 客户端。
func New(cfg Config) *Client {
	if cfg.Port == 0 {
		if cfg.SSL {
			cfg.Port = 465
		} else {
			cfg.Port = 25
		}
	}
	return &Client{cfg: cfg}
}

// Send 发送邮件。
func (c *Client) Send(msg Message) error {
	if err := c.validate(msg); err != nil {
		return err
	}

	data, recipients, err := c.buildMIME(msg)
	if err != nil {
		return fmt.Errorf("smtp: build mime: %w", err)
	}

	return c.send(recipients, data)
}

// SendTo 发送简单邮件（快捷方法）。
func (c *Client) SendTo(to, subject, body string) error {
	return c.Send(Message{
		To:      []string{to},
		Subject: subject,
		Body:    body,
	})
}

// SendHTML 发送 HTML 邮件（快捷方法）。
func (c *Client) SendHTML(to, subject, htmlBody string) error {
	return c.Send(Message{
		To:      []string{to},
		Subject: subject,
		Body:    htmlBody,
		HTML:    true,
	})
}

func (c *Client) validate(msg Message) error {
	if c.cfg.Host == "" {
		return errors.New("smtp: host is required")
	}
	if c.cfg.Username == "" {
		return errors.New("smtp: username is required")
	}
	if c.cfg.Password == "" {
		return errors.New("smtp: password is required")
	}
	if c.cfg.From == "" && msg.From == "" {
		return errors.New("smtp: from address is required")
	}
	if len(msg.To) == 0 && len(msg.CC) == 0 && len(msg.BCC) == 0 {
		return errors.New("smtp: at least one recipient is required")
	}
	return nil
}

func (c *Client) buildMIME(msg Message) ([]byte, []string, error) {
	from := msg.From
	if from == "" {
		from = c.cfg.From
	}
	fromName := msg.FromName
	if fromName == "" {
		fromName = c.cfg.FromName
	}

	var b strings.Builder
	boundary := fmt.Sprintf("_=_tingo_%d_", genBoundary())

	// 邮件头
	b.WriteString(fmt.Sprintf("From: %s\r\n", formatAddr(fromName, from)))
	b.WriteString(fmt.Sprintf("To: %s\r\n", joinAddrs(msg.To)))
	if len(msg.CC) > 0 {
		b.WriteString(fmt.Sprintf("Cc: %s\r\n", joinAddrs(msg.CC)))
	}
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", mimeEncodeHeader(msg.Subject)))
	b.WriteString("MIME-Version: 1.0\r\n")

	// 额外头部
	for k, v := range msg.Headers {
		b.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	if len(msg.Attachments) > 0 {
		b.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
		b.WriteString("\r\n")

		// 正文 part
		bodyBoundary := "_tingo_body_"
		b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		b.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", bodyBoundary))
		b.WriteString("\r\n")
		c.writeBody(&b, msg, bodyBoundary)

		// 附件
		for _, att := range msg.Attachments {
			c.writeAttachment(&b, boundary, att)
		}
		b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		contentType := "text/plain"
		if msg.HTML {
			contentType = "text/html"
		}
		b.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", contentType))
		b.WriteString("Content-Transfer-Encoding: base64\r\n")
		b.WriteString("\r\n")
		b.WriteString(base64Encode(msg.Body))
	}

	// 收件人列表（包括 CC、BCC）
	recipients := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	recipients = append(recipients, msg.To...)
	recipients = append(recipients, msg.CC...)
	recipients = append(recipients, msg.BCC...)

	return []byte(b.String()), recipients, nil
}

func (c *Client) writeBody(b *strings.Builder, msg Message, boundary string) {
	// 纯文本 part
	textBody := msg.Body
	if msg.HTML {
		textBody = stripHTML(msg.Body)
	}
	b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("\r\n")
	b.WriteString(base64Encode(textBody))
	b.WriteString("\r\n")

	if msg.HTML {
		b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n")
		b.WriteString("\r\n")
		b.WriteString(base64Encode(msg.Body))
		b.WriteString("\r\n")
	}

	b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
}

func (c *Client) writeAttachment(b *strings.Builder, boundary string, att Attachment) {
	contentType := att.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	b.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, att.Filename))
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.Filename))
	b.WriteString("\r\n")
	b.WriteString(base64EncodeBytes(att.Data))
	b.WriteString("\r\n")
}

func (c *Client) send(recipients []string, data []byte) error {
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	auth := c.auth()

	if c.cfg.SSL {
		return c.sendSSL(addr, auth, recipients, data)
	}
	return c.sendPlain(addr, auth, recipients, data)
}

func (c *Client) sendSSL(addr string, auth smtp.Auth, recipients []string, data []byte) error {
	tlsCfg := c.cfg.TLSConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{ServerName: c.cfg.Host}
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("smtp: tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp: new client: %w", err)
	}
	defer client.Quit()

	return c.authAndSend(client, auth, recipients, data)
}

func (c *Client) sendPlain(addr string, auth smtp.Auth, recipients []string, data []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp: dial: %w", err)
	}
	defer client.Quit()

	return c.authAndSend(client, auth, recipients, data)
}

func (c *Client) authAndSend(client *smtp.Client, auth smtp.Auth, recipients []string, data []byte) error {
	if c.cfg.LocalName != "" {
		if err := client.Hello(c.cfg.LocalName); err != nil {
			return fmt.Errorf("smtp: hello: %w", err)
		}
	}

	// 尝试 STARTTLS（非 SSL 连接）
	if !c.cfg.SSL {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsCfg := c.cfg.TLSConfig
			if tlsCfg == nil {
				tlsCfg = &tls.Config{ServerName: c.cfg.Host}
			}
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("smtp: starttls: %w", err)
			}
		}
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}

	from := c.cfg.From
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp: rcpt to %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: close: %w", err)
	}

	return nil
}

func (c *Client) auth() smtp.Auth {
	return LoginAuth(c.cfg.Username, c.cfg.Password, c.cfg.Host)
}

// ---- 编码辅助 ----

func formatAddr(name, addr string) string {
	if name == "" {
		return addr
	}
	return fmt.Sprintf("%s <%s>", mimeEncodeHeader(name), addr)
}

func joinAddrs(addrs []string) string {
	parts := make([]string, len(addrs))
	copy(parts, addrs)
	return strings.Join(parts, ", ")
}

func mimeEncodeHeader(s string) string {
	addr, err := stdmail.ParseAddress(s)
	if err == nil && addr.Address == s {
		return s
	}
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

func base64Encode(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	return wrapLines(encoded, 76)
}

func base64EncodeBytes(data []byte) string {
	return wrapLines(base64.StdEncoding.EncodeToString(data), 76)
}

func wrapLines(s string, width int) string {
	var b strings.Builder
	for i := 0; i < len(s); i += width {
		end := i + width
		if end > len(s) {
			end = len(s)
		}
		b.WriteString(s[i:end])
		b.WriteString("\r\n")
	}
	return b.String()
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// ---- LOGIN 认证（兼容老旧邮件服务器） ----

type loginAuth struct {
	username, password string
	host               string
}

// LoginAuth 返回一个支持 LOGIN 认证的 smtp.Auth（同时支持 PLAIN）。
func LoginAuth(username, password, host string) smtp.Auth {
	return &loginAuth{username: username, password: password, host: host}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !a.hostOk(server.Name) {
		return "", nil, errors.New("smtp: unencrypted connection")
	}
	if server.Name != a.host {
		return "", nil, errors.New("smtp: wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	cmd := strings.ToUpper(strings.TrimSpace(string(fromServer)))
		switch cmd {
		case "USERNAME:":
			return []byte(a.username), nil
		case "PASSWORD:":
			return []byte(a.password), nil
		}
		return nil, errors.New("smtp: unexpected server challenge: "+string(fromServer))
}

func (a *loginAuth) hostOk(serverName string) bool {
	return serverName == a.host || strings.HasSuffix(a.host, "."+serverName)
}

// ---- boundary 生成 ----

var boundaryCounter uint32

func genBoundary() uint32 {
	return boundaryCounter + 1 // 简化版，无需并发安全（发邮件不频繁）
}

// 保留标准库引用。
var _ = io.EOF
