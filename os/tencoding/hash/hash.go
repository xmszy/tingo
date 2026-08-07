// Package hash 提供哈希编解码。
// 设计要点：
//   - 基于标准库 hash/fnv，零外部依赖。
//   - 提供 FNV-1a / DJB / BKDR 等通用哈希函数。
package hash

import (
	"hash/fnv"
)

// Fnv32 计算 32 位 FNV-1a 哈希。
func Fnv32(data []byte) uint32 {
	h := fnv.New32a()
	h.Write(data)
	return h.Sum32()
}

// Fnv64 计算 64 位 FNV-1a 哈希。
func Fnv64(data []byte) uint64 {
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}

// Fnv32String 计算字符串的 32 位 FNV-1a 哈希。
func Fnv32String(s string) uint32 { return Fnv32([]byte(s)) }

// Fnv64String 计算字符串的 64 位 FNV-1a 哈希。
func Fnv64String(s string) uint64 { return Fnv64([]byte(s)) }

// BKDR 经典 BKDR 哈希（用于字符串简单散列）。
func BKDR(s string) uint32 {
	var seed uint32 = 131
	var h uint32
	for i := 0; i < len(s); i++ {
		h = h*seed + uint32(s[i])
	}
	return h
}

// DJB DJB2 哈希。
func DJB(s string) uint32 {
	var h uint32 = 5381
	for i := 0; i < len(s); i++ {
		h = ((h << 5) + h) + uint32(s[i])
	}
	return h
}

// SDBM SDBM 哈希。
func SDBM(s string) uint32 {
	var h uint32
	for i := 0; i < len(s); i++ {
		h = uint32(s[i]) + (h << 6) + (h << 16) - h
	}
	return h
}

// AP AP 哈希。
func AP(s string) uint32 {
	var h uint32
	for i := 0; i < len(s); i++ {
		if i&1 == 0 {
			h ^= ((h << 7) ^ uint32(s[i]) ^ (h >> 3))
		} else {
			h ^= (^((h << 11) ^ uint32(s[i]) ^ (h >> 5)))
		}
	}
	return h
}
