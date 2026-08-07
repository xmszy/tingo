// Package des 提供 DES / Triple DES 对称加密/解密。
// 设计要点：
//   - 基于标准库 crypto/des 和 crypto/cipher，零外部依赖。
//   - 支持 CBC 和 CFB 两种模式。
//   - 提供 PKCS#5 填充工具函数。
//   - 默认 IV = "I Love Go Frame!"。
//   - 注意：DES 已不再安全，建议使用 AES 或 Triple DES（3DES）。
package des

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"fmt"
)

// 默认 IV，与 gf 保持一致。
var defaultIV = []byte("I Love Go Frame!")

// SetDefaultIV 修改全局默认 IV。
func SetDefaultIV(iv []byte) { defaultIV = iv }

// Encrypt 使用 DES-CBC 加密（EncryptCBC 的别名）。
func Encrypt(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	return EncryptCBC(plainText, key, iv...)
}

// Decrypt 使用 DES-CBC 解密（DecryptCBC 的别名）。
func Decrypt(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	return DecryptCBC(cipherText, key, iv...)
}

// ──────────────── DES CBC 模式 ────────────────

// EncryptCBC 使用 DES-CBC 加密。
func EncryptCBC(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("des.EncryptCBC: %w", err)
	}
	plainText = PKCS5Padding(plainText, block.BlockSize())
	ivData := pickIV(iv...)
	mode := cipher.NewCBCEncrypter(block, ivData)
	cipherText := make([]byte, len(plainText))
	mode.CryptBlocks(cipherText, plainText)
	return cipherText, nil
}

// DecryptCBC 使用 DES-CBC 解密。
func DecryptCBC(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("des.DecryptCBC: %w", err)
	}
	ivData := pickIV(iv...)
	mode := cipher.NewCBCDecrypter(block, ivData)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)
	plainText, err = PKCS5UnPadding(plainText, block.BlockSize())
	if err != nil {
		return nil, fmt.Errorf("des.DecryptCBC: %w", err)
	}
	return plainText, nil
}

// ──────────────── DES CFB 模式 ────────────────

// EncryptCFB 使用 DES-CFB 加密。
func EncryptCFB(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("des.EncryptCFB: %w", err)
	}
	ivData := pickIV(iv...)
	stream := cipher.NewCFBEncrypter(block, ivData)
	cipherText := make([]byte, len(plainText))
	stream.XORKeyStream(cipherText, plainText)
	return cipherText, nil
}

// DecryptCFB 使用 DES-CFB 解密。
func DecryptCFB(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("des.DecryptCFB: %w", err)
	}
	ivData := pickIV(iv...)
	stream := cipher.NewCFBDecrypter(block, ivData)
	plainText := make([]byte, len(cipherText))
	stream.XORKeyStream(plainText, cipherText)
	return plainText, nil
}

// ──────────────── Triple DES CBC ────────────────

// EncryptTripleCBC 使用 3DES-CBC 加密（key 长度需为 24 字节）。
func EncryptTripleCBC(plainText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, fmt.Errorf("des.EncryptTripleCBC: %w", err)
	}
	plainText = PKCS5Padding(plainText, block.BlockSize())
	ivData := pickIV(iv...)
	mode := cipher.NewCBCEncrypter(block, ivData)
	cipherText := make([]byte, len(plainText))
	mode.CryptBlocks(cipherText, plainText)
	return cipherText, nil
}

// DecryptTripleCBC 使用 3DES-CBC 解密（key 长度需为 24 字节）。
func DecryptTripleCBC(cipherText, key []byte, iv ...[]byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, fmt.Errorf("des.DecryptTripleCBC: %w", err)
	}
	ivData := pickIV(iv...)
	mode := cipher.NewCBCDecrypter(block, ivData)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)
	plainText, err = PKCS5UnPadding(plainText, block.BlockSize())
	if err != nil {
		return nil, fmt.Errorf("des.DecryptTripleCBC: %w", err)
	}
	return plainText, nil
}

// ──────────────── PKCS 填充 ────────────────

// PKCS5Padding 对数据做 PKCS#5 填充。blockSize 可选，默认 8（DES 块大小）。
func PKCS5Padding(src []byte, blockSize ...int) []byte {
	bs := 8
	if len(blockSize) > 0 && blockSize[0] > 0 {
		bs = blockSize[0]
	}
	padding := bs - len(src)%bs
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padText...)
}

// PKCS5UnPadding 对 PKCS#5 填充的数据去填充。
func PKCS5UnPadding(src []byte, blockSize ...int) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("des.PKCS5UnPadding: empty data")
	}
	padding := int(src[len(src)-1])
	if padding > len(src) {
		return nil, fmt.Errorf("des.PKCS5UnPadding: invalid padding size")
	}
	return src[:len(src)-padding], nil
}

// ──────────────── 辅助 ────────────────

func pickIV(iv ...[]byte) []byte {
	if len(iv) > 0 && len(iv[0]) == des.BlockSize {
		return iv[0]
	}
	dst := make([]byte, des.BlockSize)
	if len(iv) > 0 {
		copy(dst, iv[0])
		return dst
	}
	copy(dst, defaultIV)
	return dst
}
