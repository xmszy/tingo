# 缓存

Tingo 的缓存组件 `tcache` 是泛型内存缓存，支持过期时间。

## 基本用法

### 设置缓存

~~~go
import (
    "time"
    "github.com/xmszy/tingo/frame"
)

// 设置（带过期时间）
t.CacheSet(t.Cache(), "key", "value", time.Hour)

// 设置数字
t.CacheSet(t.Cache(), "user_count", 100, time.Minute*30)

// 设置结构体
t.CacheSet(t.Cache(), "user:1", user, time.Hour)
~~~

### 获取缓存

~~~go
// 泛型获取（无需类型断言）
value, found := t.CacheGet[string](t.Cache(), "key")
if found {
    fmt.Println(value)
}

// 数字类型
count, found := t.CacheGet[int](t.Cache(), "user_count")

// 结构体类型
user, found := t.CacheGet[*User](t.Cache(), "user:1")
~~~

### 删除缓存

~~~go
t.Cache().Delete("key")
t.Cache().Clear()  // 清空所有
~~~

### 存在判断

~~~go
if t.Cache().Has("key") {
    // 缓存存在
}
~~~

## 缓存过期

### 带 TTL 的设置

~~~go
t.CacheSet(t.Cache(), "token", token, time.Minute*10)
t.CacheSet(t.Cache(), "config", cfg, time.Hour*24)
t.CacheSet(t.Cache(), "static", "forever", 0)  // 永不过期
~~~

### Take —— 不存在时自动加载

~~~go
user, err := t.CacheGetOrLoad(t.Cache(), "user:100", time.Hour, func() (*User, error) {
    return userService.Find(100)
})
~~~

## 散列（Hash）

~~~go
// 设置 hash 字段
t.Cache().HSet("user:1", "name", "张三")
t.Cache().HSet("user:1", "email", "zhangsan@example.com")

// 获取 hash 字段
name, _ := t.CacheGet[string](t.Cache(), "user:1")  // 获取整体
// 或逐字段
// t.Cache().HMGet("user:1", "name", "email")

// 删除 hash 字段
t.Cache().HDel("user:1", "email")
~~~

## 列表

~~~go
t.Cache().LPush("queue", "job1")
t.Cache().LPush("queue", "job2")
item, _ := t.Cache().RPop("queue")  // "job1"
~~~

## 原子操作

### SetIfNotExist —— 不存在时写入

仅在 key 不存在（或已过期）时才写入，返回 true 表示写入成功。分片锁内完成 检查+写入，原子 CAS 语义，避免缓存击穿：

~~~go
ok := t.Cache().SetIfNotExist("lock:user:1", true, time.Second*10)
if ok {
    // 获取锁成功，执行业务逻辑
    defer t.Cache().Delete("lock:user:1")
    // ...
} else {
    // 锁已被其他请求持有
}
~~~

### GetOrSet —— 双重检查锁

与 `GetOr` 不同，`GetOrSet` 在锁内二次检查，并发场景下保证 fn 只被调用一次：

~~~go
user, err := t.CacheGetOrSet(t.Cache(), "user:100", time.Hour, func() (*User, error) {
    // 这段代码在并发下只会执行一次
    return userService.Find(100)
})
~~~

### Update —— 更新值不改过期

~~~go
// 更新值，保持原有过期时间和 LRU 序位
t.Cache().Update("user:1", updatedUser)
~~~

### UpdateExpire —— 更新过期不改值

~~~go
// 延长过期时间，保持值不变
t.Cache().UpdateExpire("user:1", time.Hour)
// 设为永不过期
t.Cache().UpdateExpire("user:1", 0)
~~~

### Keys —— 获取所有 Key

~~~go
keys := t.Cache().Keys()
for _, k := range keys {
    fmt.Println(k)
}
~~~

## 缓存标签/命名空间

~~~go
// 按前缀清空
t.Cache().ClearPrefix("user:")

// 所有 user:* 缓存被清除
~~~

## 配置

~~~toml
# cache.toml
default = "memory"

[stores.memory]
type = "memory"
expire = "1h"
cleanup_interval = "5m"
max_entries = 10000
~~~
