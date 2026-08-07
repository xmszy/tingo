// Package rsa 提供 RSA 非对称加密/解密。
// 设计要点：
//   - 基于标准库 crypto/rsa、crypto/x509、crypto/rand，零外部依赖。
//   - 支持 PKCS#1 和 PKCS#8/PKIX 两种密钥格式。
//   - 提供 OAEP（推荐）+ PKCS1v15 两种填充方案。
//   - 支持密钥生成和格式检测。
package rsa

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"hash"
)

// ──────────────── 加密/解密（PKCS#1 v1.5，自动检测密钥格式） ────────────────

// Encrypt 使用 RSA 公钥加密，自动识别 PKCS#1 / PKIX 格式。
func Encrypt(plainText, publicKey []byte) ([]byte, error) {
	key, err := parsePublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.Encrypt: %w", err)
	}
	return rsa.EncryptPKCS1v15(rand.Reader, key, plainText)
}

// Decrypt 使用 RSA 私钥解密，自动识别 PKCS#1 / PKCS#8 格式。
func Decrypt(cipherText, privateKey []byte) ([]byte, error) {
	key, err := parsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.Decrypt: %w", err)
	}
	return rsa.DecryptPKCS1v15(rand.Reader, key, cipherText)
}

// ──────────────── PKCS#1 格式明确指定 ────────────────

// EncryptPKCS1 使用 PKCS#1 格式公钥加密。
func EncryptPKCS1(plainText, publicKey []byte) ([]byte, error) {
	key, err := parsePKCS1PublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.EncryptPKCS1: %w", err)
	}
	return rsa.EncryptPKCS1v15(rand.Reader, key, plainText)
}

// DecryptPKCS1 使用 PKCS#1 格式私钥解密。
func DecryptPKCS1(cipherText, privateKey []byte) ([]byte, error) {
	key, err := parsePKCS1PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.DecryptPKCS1: %w", err)
	}
	return rsa.DecryptPKCS1v15(rand.Reader, key, cipherText)
}

// ──────────────── PKIX/PKCS#8 格式明确指定 ────────────────

// EncryptPKIX 使用 PKIX 格式公钥加密。
func EncryptPKIX(plainText, publicKey []byte) ([]byte, error) {
	key, err := parsePKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.EncryptPKIX: %w", err)
	}
	return rsa.EncryptPKCS1v15(rand.Reader, key, plainText)
}

// DecryptPKCS8 使用 PKCS#8 格式私钥解密。
func DecryptPKCS8(cipherText, privateKey []byte) ([]byte, error) {
	key, err := parsePKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.DecryptPKCS8: %w", err)
	}
	return rsa.DecryptPKCS1v15(rand.Reader, key, cipherText)
}

// ──────────────── Base64 便捷方法 ────────────────

// EncryptBase64 使用 Base64 公钥加密，返回 Base64 密文。
func EncryptBase64(plainText []byte, publicKeyBase64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return "", fmt.Errorf("rsa.EncryptBase64: %w", err)
	}
	cipher, err := Encrypt(plainText, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipher), nil
}

// DecryptBase64 使用 Base64 私钥解密 Base64 密文。
func DecryptBase64(cipherTextBase64, privateKeyBase64 string) ([]byte, error) {
	cipher, err := base64.StdEncoding.DecodeString(cipherTextBase64)
	if err != nil {
		return nil, fmt.Errorf("rsa.DecryptBase64: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("rsa.DecryptBase64: %w", err)
	}
	return Decrypt(cipher, key)
}

// ──────────────── OAEP（推荐） ────────────────

// EncryptOAEP 使用 OAEP（SHA-256）加密。
func EncryptOAEP(plainText, publicKey []byte) ([]byte, error) {
	return EncryptOAEPWithHash(plainText, publicKey, nil, crypto.SHA256.New())
}

// DecryptOAEP 使用 OAEP（SHA-256）解密。
func DecryptOAEP(cipherText, privateKey []byte) ([]byte, error) {
	return DecryptOAEPWithHash(cipherText, privateKey, nil, crypto.SHA256.New())
}

// EncryptOAEPWithHash 使用 OAEP 加密（指定 hash 和 label）。
func EncryptOAEPWithHash(plainText, publicKey, label []byte, hash hash.Hash) ([]byte, error) {
	key, err := parsePublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.EncryptOAEP: %w", err)
	}
	return rsa.EncryptOAEP(hash, rand.Reader, key, plainText, label)
}

// DecryptOAEPWithHash 使用 OAEP 解密（指定 hash 和 label）。
func DecryptOAEPWithHash(cipherText, privateKey, label []byte, hash hash.Hash) ([]byte, error) {
	key, err := parsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("rsa.DecryptOAEP: %w", err)
	}
	return rsa.DecryptOAEP(hash, rand.Reader, key, cipherText, label)
}

// ──────────────── 密钥生成 ────────────────

// GenerateKeyPair 生成 RSA 密钥对（PKCS#1 格式），返回私钥和公钥的 PEM 字节。
func GenerateKeyPair(bits int) (privateKey, publicKey []byte, err error) {
	sk, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("rsa.GenerateKeyPair: %w", err)
	}
	privDer := x509.MarshalPKCS1PrivateKey(sk)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDer})
	pubDer, err := x509.MarshalPKIXPublicKey(&sk.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("rsa.GenerateKeyPair: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})
	return privPEM, pubPEM, nil
}

// GenerateKeyPairPKCS8 生成 RSA 密钥对（PKCS#8 格式）。
func GenerateKeyPairPKCS8(bits int) (privateKey, publicKey []byte, err error) {
	sk, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("rsa.GenerateKeyPairPKCS8: %w", err)
	}
	privDer, err := x509.MarshalPKCS8PrivateKey(sk)
	if err != nil {
		return nil, nil, fmt.Errorf("rsa.GenerateKeyPairPKCS8: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDer})
	pubDer, err := x509.MarshalPKIXPublicKey(&sk.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("rsa.GenerateKeyPairPKCS8: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})
	return privPEM, pubPEM, nil
}

// GenerateDefaultKeyPair 使用默认 2048 位生成密钥对（PKCS#1）。
func GenerateDefaultKeyPair() (privateKey, publicKey []byte, err error) {
	return GenerateKeyPair(2048)
}

// ──────────────── 密钥格式检测 ────────────────

// GetPrivateKeyType 检测私钥的编码格式，返回 "PKCS1" / "PKCS8" / "unknown"。
func GetPrivateKeyType(privateKey []byte) (string, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return "", fmt.Errorf("rsa.GetPrivateKeyType: invalid PEM")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return "PKCS1", nil
	case "PRIVATE KEY":
		return "PKCS8", nil
	default:
		return "unknown", nil
	}
}

// ──────────────── 内部解析 ────────────────

func parsePublicKey(publicKey []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	switch block.Type {
	case "RSA PUBLIC KEY":
		return parsePKCS1PublicKey(publicKey)
	default:
		return parsePKIXPublicKey(publicKey)
	}
}

func parsePrivateKey(privateKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return parsePKCS1PrivateKey(privateKey)
	default:
		return parsePKCS8PrivateKey(privateKey)
	}
}

func parsePKCS1PublicKey(publicKey []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func parsePKIXPublicKey(publicKey []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}

func parsePKCS1PrivateKey(privateKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func parsePKCS8PrivateKey(privateKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return rsaKey, nil
}
