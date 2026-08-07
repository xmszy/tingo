// Package aes 提供 AES 对称加密/解密。
// 设计要点：
//   - 基于标准库 crypto/aes 和 crypto/cipher，零外部依赖。
//   - 支持 CBC 和 CFB 两种模式。
//   - 提供 PKCS#5 / PKCS#7 填充工具函数。
//   - 默认 IV = "I Love Go Frame!"（与 gf 保持一致），可通过可变参数覆盖。
package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// 默认 IV，与 gf gaes 保持一致。
var defaultIV = []byte("I Love Go Frame!")

// SetDefaultIV 修改全局默认 IV。
func SetDefaultIV(iv []byte) { defaultIV = iv }

// Encrypt 使用 AES-CBC 加密（EncryptCBC 的别名）。
func Encrypt(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	return EncryptCBC(plainText, key, iv...)
}

// Decrypt 使用 AES-CBC 解密（DecryptCBC 的别名）。
func Decrypt(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	return DecryptCBC(cipherText, key, iv...)
}

// ──────────────── CBC 模式 ────────────────

// EncryptCBC 使用 AES-CBC 加密。iv 可选，默认使用全局 defaultIV。
func EncryptCBC(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.EncryptCBC: %w", err)
	}
	plainText = PKCS5Padding(plainText, block.BlockSize())
	ivData := pickIV(iv...)
	mode := cipher.NewCBCEncrypter(block, ivData)
	cipherText := make([]byte, len(plainText))
	mode.CryptBlocks(cipherText, plainText)
	return cipherText, nil
}

// DecryptCBC 使用 AES-CBC 解密。iv 可选，默认使用全局 defaultIV。
func DecryptCBC(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.DecryptCBC: %w", err)
	}
	ivData := pickIV(iv...)
	mode := cipher.NewCBCDecrypter(block, ivData)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)
	plainText, err = PKCS5UnPadding(plainText, block.BlockSize())
	if err != nil {
		return nil, fmt.Errorf("aes.DecryptCBC: %w", err)
	}
	return plainText, nil
}

// ──────────────── CFB 模式 ────────────────

// EncryptCFB 使用 AES-CFB 加密。iv 可选。
func EncryptCFB(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.EncryptCFB: %w", err)
	}
	ivData := pickIV(iv...)
	stream := cipher.NewCFBEncrypter(block, ivData)
	cipherText := make([]byte, len(plainText))
	stream.XORKeyStream(cipherText, plainText)
	return cipherText, nil
}

// DecryptCFB 使用 AES-CFB 解密。iv 可选。
func DecryptCFB(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.DecryptCFB: %w", err)
	}
	ivData := pickIV(iv...)
	stream := cipher.NewCFBDecrypter(block, ivData)
	plainText := make([]byte, len(cipherText))
	stream.XORKeyStream(plainText, cipherText)
	return plainText, nil
}

// ──────────────── PKCS 填充 ────────────────

// PKCS5Padding 对数据做 PKCS#5 填充。blockSize 可选，默认为 aes.BlockSize。
func PKCS5Padding(src []byte, blockSize ...int) []byte {
	bs := aes.BlockSize
	if len(blockSize) > 0 && blockSize[0] > 0 {
		bs = blockSize[0]
	}
	padding := bs - len(src)%bs
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padText...)
}

// PKCS5UnPadding 对 PKCS#5 填充的数据去填充。blockSize 可选。
func PKCS5UnPadding(src []byte, blockSize ...int) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, fmt.Errorf("aes.PKCS5UnPadding: empty data")
	}
	padding := int(src[length-1])
	if padding > length {
		return nil, fmt.Errorf("aes.PKCS5UnPadding: invalid padding size %d > data length %d", padding, length)
	}
	return src[:length-padding], nil
}

// PKCS7Padding 对数据做 PKCS#7 填充。
func PKCS7Padding(src []byte, blockSize int) []byte {
	return PKCS5Padding(src, blockSize)
}

// PKCS7UnPadding 对 PKCS#7 填充的数据去填充。
func PKCS7UnPadding(src []byte, blockSize int) ([]byte, error) {
	return PKCS5UnPadding(src, blockSize)
}

// ──────────────── 辅助 ────────────────

func pickIV(iv ...[]byte) []byte {
	if len(iv) > 0 && len(iv[0]) == aes.BlockSize {
		return iv[0]
	}
	// 如果传入的 iv 长度不对，截取或补齐到 BlockSize。
	if len(iv) > 0 {
		dst := make([]byte, aes.BlockSize)
		copy(dst, iv[0])
		return dst
	}
	return defaultIV
}
