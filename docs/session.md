# Session

Tingo 提供两种会话存储：内存（MemoryStore）和数据库（DBStore）。

## 基本用法

### 获取会话管理器

~~~go
import "github.com/xmszy/tingo/frame"

mgr := t.Session()
~~~

### 从请求获取会话

~~~go
func handler(c *t.Ctx) {
    sess, err := t.Session().Start(c)
    if err != nil {
        // 处理错误
        return
    }

    // 设置
    sess.Set("user_id", 1)
    sess.Set("user_name", "张三")

    // 获取（泛型，无需类型断言）
    userID, ok := t.SessionGet[int](sess, "user_id")

    // 删除
    sess.Delete("temp_data")

    // 销毁会话
    sess.Destroy()

    // 保存（DBStore 某些操作后需手动保存）
    sess.Save()
}
~~~

## 配置

~~~toml
# session.toml
cookie_name = "tingo_sid"
expire = "24h"
path = "/"
domain = ""
secure = false
http_only = true

[store]
type = "memory"
# 或 type = "database"，使用数据库存储
table = "sessions"
~~~

## 内存存储

默认使用内存存储，适合开发环境和单机部署：

~~~go
mgr := t.SessionNew(t.SessionConfig{
    CookieName: "tingo_sid",
    Expire:     24 * time.Hour,
    Store:      "memory",
})
~~~

## 数据库存储

生产环境推荐使用数据库存储（支持跨实例共享）：

~~~go
mgr := t.SessionNew(t.SessionConfig{
    CookieName: "tingo_sid",
    Expire:     24 * time.Hour,
    Store:      "database",
    StoreConfig: map[string]string{
        "table": "sessions",
    },
})

// 会话表结构
// CREATE TABLE sessions (
//     id VARCHAR(128) PRIMARY KEY,
//     data TEXT,
//     expire INT
// );
~~~

DBStore 通过 `db.SQL()` 访问数据库连接池。

## 过期与垃圾回收

- 内存存储：后台 goroutine 定期清理过期会话
- 数据库存储：读取时检查过期时间，GC 概率触发清理

## 会话中间件

通过 `tingo-contrib/sessions`（独立模块）中间件自动管理会话生命周期：

~~~go
import "github.com/xmszy/tingo-contrib/sessions"

r.Use(sessions.Middleware(t.Session()))
~~~

中间件自动执行 `Start` / `Save`，handler 中直接获取当前会话：

~~~go
func handler(c *t.Ctx) {
    sess := sessions.Get(c)
    name, _ := t.SessionGet[string](sess, "name")
}
~~~
