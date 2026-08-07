package tdb

import (
	"encoding/json"
	"time"

	"github.com/xmszy/tingo/os/tcache"
)

// Cache 启用查询缓存。
// key 为空时自动基于 SQL 生成缓存键；ttl 指定缓存过期时间。
func (m *Model[T]) Cache(key string, ttl time.Duration) *Model[T] {
	model := m.Clone()
	model.cacheKey = key
	model.cacheTTL = ttl
	model.cacheEnabled = true
	return model
}

// CacheForever 永久缓存（无过期）。
func (m *Model[T]) CacheForever(key string) *Model[T] {
	return m.Cache(key, 0)
}

// WithoutCache 禁用当前查询的缓存。
func (m *Model[T]) WithoutCache() *Model[T] {
	model := m.Clone()
	model.cacheEnabled = false
	return model
}

// cachedAll 带缓存的 All 查询。
func (m *Model[T]) cachedAll() ([]T, error) {
	cacheKey := m.cacheKey
	if cacheKey == "" {
		cacheKey = m.buildCacheKey()
	}

	c := cacheForDB(m.db)
	if c == nil {
		return m.All()
	}

	// 尝试从缓存读取
	if data, ok := c.Get(cacheKey); ok {
		if b, isBytes := data.([]byte); isBytes {
			var result []T
			if err := json.Unmarshal(b, &result); err == nil {
				return result, nil
			}
		}
	}

	// 从数据库查询
	result, err := m.All()
	if err != nil {
		return nil, err
	}

	// 写入缓存
	data, err := json.Marshal(result)
	if err == nil {
		c.Set(cacheKey, data, m.cacheTTL)
	}
	return result, nil
}

// ensureCacheUsed prevents unused method warning until cache is wired into query path.
var _ = (*Model[struct{}]).cachedAll

// buildCacheKey 基于 table + SQL + args 生成缓存键。
func (m *Model[T]) buildCacheKey() string {
	sqlStr, args := m.buildSelect()
	key := sqlStr
	for _, a := range args {
		key += "\x00" + anyToString(a)
	}
	var h uint64 = 5381
	for _, c := range []byte(key) {
		h = ((h << 5) + h) + uint64(c)
	}
	return "tdb:cache:" + m.table + ":" + uint64ToHex(h)
}

// cacheForDB 获取 DB 的缓存实例。
func cacheForDB(db *DB) *tcache.Cache {
	if db == nil || db.cache == nil {
		return nil
	}
	return db.cache
}

// uint64ToHex 将 uint64 转为 16 进制字符串。
func uint64ToHex(n uint64) string {
	if n == 0 {
		return "0"
	}
	const hexChars = "0123456789abcdef"
	b := make([]byte, 16)
	i := 15
	for n > 0 {
		b[i] = hexChars[n&0xf]
		n >>= 4
		i--
	}
	return string(b[i+1:])
}

// anyToString 将 any 转为字符串表示。
func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return itoa64(int64(val))
	case int64:
		return itoa64(val)
	case float64:
		b, _ := json.Marshal(val)
		return string(b)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

