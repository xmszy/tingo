# Cookie

Tingo 提供便捷的 Cookie 读写操作，基于 `net/http` 标准库。

## 设置 Cookie

~~~go
func handler(c *t.Ctx) {
    // 基本设置
    c.SetCookie("token", token, 3600, "/", "", false, true)

    // 使用 WithCookie（链式风格）
    c.WithCookie("remember", "1").
        Path("/").
        MaxAge(86400 * 7).
        HttpOnly(true).
        Set()
}
~~~

参数说明：

| 参数 | 说明 |
|---|---|
| name | Cookie 名称 |
| value | Cookie 值 |
| maxAge | 过期秒数（0=会话结束，<0=删除） |
| path | 有效路径 |
| domain | 有效域名 |
| secure | 仅 HTTPS |
| httpOnly | 仅 HTTP 访问 |

## 读取 Cookie

~~~go
func handler(c *t.Ctx) {
    token, err := c.Cookie("token")
    if err != nil {
        // Cookie 不存在
    }
}
~~~

## 删除 Cookie

~~~go
c.SetCookie("token", "", -1, "/", "", false, true)
// 或
c.WithCookie("token", "").MaxAge(-1).Set()
~~~

## 配置

~~~toml
# cookie.toml
prefix = ""
path = "/"
domain = ""
secure = false
http_only = true
~~~

## 加密 Cookie

敏感数据不应直接存储在 Cookie 中。推荐使用 Session 存储敏感信息：

~~~go
// ✅ 推荐：Session 存储敏感数据，Cookie 仅存 Session ID
sess.Set("user_id", userID)

// ❌ 不推荐：Cookie 直接存敏感数据
c.SetCookie("user_id", userID, 3600, "/", "", false, true)
~~~
