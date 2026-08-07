// Package twebsocket 提供 WebSocket 支持。
//
// 基于标准库 net/http 的 WebSocket 升级，支持消息读写、ping/pong 保持连接、
// 并发安全的消息发送。
//
// 依赖 github.com/gorilla/websocket，需要：
//
//	go get github.com/gorilla/websocket
//
// 用法：
//
//	ws := twebsocket.New(c.ResponseWriter(), c.Request(), nil)
//	for {
//	    msgType, data, err := ws.ReadMessage()
//	    if err != nil { break }
//	    ws.WriteMessage(msgType, data) // echo
//	}
package twebsocket

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 默认 WebSocket 配置参数。
var (
	DefaultReadBufferSize  = 1024
	DefaultWriteBufferSize = 1024
	DefaultPingWait       = 60 * time.Second
	DefaultPongWait       = 60 * time.Second
	DefaultWriteWait      = 10 * time.Second
	DefaultMaxMessageSize = 512 * 1024 // 512KB
)

// Config WebSocket 连接配置。
type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingWait        time.Duration
	PongWait        time.Duration
	WriteWait       time.Duration
	MaxMessageSize  int64
	CheckOrigin     func(r *http.Request) bool
	Subprotocols    []string
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		ReadBufferSize:  DefaultReadBufferSize,
		WriteBufferSize: DefaultWriteBufferSize,
		PingWait:        DefaultPingWait,
		PongWait:        DefaultPongWait,
		WriteWait:       DefaultWriteWait,
		MaxMessageSize:  int64(DefaultMaxMessageSize),
	}
}

// Conn 封装 gorilla/websocket.Conn，提供更高级的 API。
type Conn struct {
	conn    *websocket.Conn
	cfg     *Config
	writeMu sync.Mutex
	closeCh chan struct{}
	closed  bool
	mu      sync.RWMutex
}

// New 创建 WebSocket 连接（升级 HTTP 请求）。
func New(w http.ResponseWriter, r *http.Request, cfg *Config) (*Conn, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		Subprotocols:    cfg.Subprotocols,
	}

	if cfg.CheckOrigin != nil {
		upgrader.CheckOrigin = cfg.CheckOrigin
	} else {
		// 默认允许所有来源
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	c := &Conn{
		conn:    wsConn,
		cfg:     cfg,
		closeCh: make(chan struct{}),
	}

	// 读取限制
	wsConn.SetReadLimit(cfg.MaxMessageSize)
	wsConn.SetReadDeadline(time.Now().Add(cfg.PongWait))
	wsConn.SetPongHandler(func(string) error {
		wsConn.SetReadDeadline(time.Now().Add(cfg.PongWait))
		return nil
	})

	return c, nil
}

// ReadMessage 读取一条消息。返回消息类型、数据，或错误。
func (c *Conn) ReadMessage() (int, []byte, error) {
	return c.conn.ReadMessage()
}

// WriteMessage 写入一条消息（并发安全）。
func (c *Conn) WriteMessage(messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(messageType, data)
}

// WriteJSON 写入 JSON 消息（并发安全）。
func (c *Conn) WriteJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait))
	return c.conn.WriteJSON(v)
}

// ReadJSON 读取 JSON 消息。
func (c *Conn) ReadJSON(v any) error {
	return c.conn.ReadJSON(v)
}

// SetReadDeadline 设置读取超时。
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// WriteControl 写入控制帧（ping/pong/close），并发安全。
func (c *Conn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return c.conn.WriteControl(messageType, data, deadline)
}

// Ping 发送 Ping 帧。
func (c *Conn) Ping() error {
	return c.WriteControl(websocket.PingMessage, nil, time.Now().Add(c.cfg.WriteWait))
}

// Close 关闭连接。
func (c *Conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closeCh)
	c.mu.Unlock()

	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	return c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(c.cfg.WriteWait))
}

// CloseWithCode 以指定状态码关闭连接。
func (c *Conn) CloseWithCode(code int, text string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closeCh)
	c.mu.Unlock()

	msg := websocket.FormatCloseMessage(code, text)
	return c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(c.cfg.WriteWait))
}

// CloseChan 返回关闭通知 channel。
func (c *Conn) CloseChan() <-chan struct{} {
	return c.closeCh
}

// IsClosed 判断连接是否已关闭。
func (c *Conn) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// UnderlyingConn 返回底层 gorilla websocket 连接。
func (c *Conn) UnderlyingConn() *websocket.Conn {
	return c.conn
}

// ---- 便捷升级函数 ----

// 标准 WebSocket 消息类型别名。
const (
	TextMessage   = websocket.TextMessage
	BinaryMessage = websocket.BinaryMessage
	CloseMessage  = websocket.CloseMessage
	PingMessage   = websocket.PingMessage
	PongMessage   = websocket.PongMessage
)

// 标准关闭状态码。
const (
	CloseNormalClosure    = websocket.CloseNormalClosure
	CloseGoingAway        = websocket.CloseGoingAway
	CloseProtocolError    = websocket.CloseProtocolError
	CloseUnsupportedData  = websocket.CloseUnsupportedData
	CloseNoStatusReceived = websocket.CloseNoStatusReceived
	CloseAbnormalClosure  = websocket.CloseAbnormalClosure
	CloseInvalidFrame     = websocket.CloseInvalidFramePayloadData
	ClosePolicyViolation  = websocket.ClosePolicyViolation
	CloseMessageTooBig    = websocket.CloseMessageTooBig
	CloseInternalServerErr = websocket.CloseInternalServerErr
)

// FormatCloseMessage 格式化 WebSocket 关闭消息。
func FormatCloseMessage(code int, text string) []byte {
	return websocket.FormatCloseMessage(code, text)
}
