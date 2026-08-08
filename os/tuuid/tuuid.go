// Package tuuid 提供 UUID/GUID 生成。
// 设计要点：
//   - 基于标准库 crypto/rand，零外部依赖。
//   - 提供 UUID v4（随机）和简易 v1（时间戳 + 随机）生成。
package tuuid

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"
)

var counter atomic.Uint64

// V4 生成 UUID v4（随机）。
func V4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// fallback: 使用 less secure 但不会失败的方案
		ts := uint64(time.Now().UnixNano())
		binary.BigEndian.PutUint64(b[:8], ts)
		binary.BigEndian.PutUint64(b[8:], ts^0x0123456789ABCDEF+counter.Add(1))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// V1Simple 生成简易 UUID v1（时间戳 + 随机节点）。
func V1Simple() string {
	ts := uint64(time.Now().UnixNano()/100 + 0x01B21DD213814000) // 100ns ticks since UUID epoch
	var node [6]byte
	if _, err := rand.Read(node[:]); err != nil {
		// fallback: 使用时间戳派生的伪随机节点，保证不返回全零弱 UUID
		v := uint64(time.Now().UnixNano()) ^ counter.Add(1)
		for i := range node {
			node[i] = byte(v >> (8 * (i % 8)))
		}
	}

	var b [16]byte
	binary.BigEndian.PutUint32(b[0:4], uint32(ts>>32))      // time_low
	binary.BigEndian.PutUint16(b[4:6], uint16(ts>>16))       // time_mid
	binary.BigEndian.PutUint16(b[6:8], uint16(ts)|0x1000)    // time_hi_and_version
	b[8] = 0x80                                               // variant
	copy(b[10:], node[:])

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Short 生成 8 字符短 ID（Base62）。
func Short() string {
	const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return V4()[:8]
	}
	for i := range b {
		b[i] = base62[int(b[i])%len(base62)]
	}
	return string(b)
}

// SID 生成 session ID（32 位十六进制）。
func SID() string {
	var b [16]byte
	rand.Read(b[:])
	return fmt.Sprintf("%x", b[:])
}

// Bytes 生成指定长度的随机字节。
func Bytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}
