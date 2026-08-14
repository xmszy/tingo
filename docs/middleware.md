# 中间件

## 定义中间件

中间件是 `core.Handler` 类型（即 `gin.HandlerFunc`）：

~~~go
// 自定义中间件
func AuthMiddleware(c *t.Ctx) {
    token := c.GetHeader("Authorization")
    if token == "" {
        c.JSON(401, t.Map{"error": "未授权"})
        c.Abort()
        return
    }
    // 验证 token...
    c.Next()
}
~~~

## 全局中间件

在应用路由根部注册全局中间件：

~~~go
// route/app.go
func Register(r t.Router) {
    // 全局中间件
    r.Use(
        recovery.Middleware(),
        cors.Middleware(cors.Default()),
        logger.Middleware(),
    )

    // 路由定义
    t.AutoRoute(r)
}
~~~

## 路由分组中间件

特定路由组使用中间件：

~~~go
func Register(r t.Router) {
    r.Use(cors.Middleware(cors.Default()))

    // 公开路由（不需要认证）
    r.GET("/public/data", publicHandler)

    // 需要认证的路由组
    r.Group("/api", func(g t.Router) {
        g.Use(authMiddleware)
        g.GET("/me", meHandler)
        g.POST("/profile", updateProfileHandler)
    })

    // 管理员路由组
    r.Group("/admin", func(g t.Router) {
        g.Use(authMiddleware, adminMiddleware)
        g.GET("/users", listUsersHandler)
    })
}
~~~

## 控制器中间件

控制器可以通过 `MiddlewareDeclarer` 声明自己的中间件：

~~~go
type UserController struct {
    t.Controller
}

func (u *UserController) Middleware() []t.Handler {
    return []t.Handler{
        authMiddleware,
    }
}
~~~

## contrib 内置中间件

Tingo 的 `tingo-contrib` 模块提供了丰富的中间件：

| 包 | 功能 |
|---|---|
| `tingo-contrib/cors` | 跨域资源共享 |
| `tingo-contrib/secure` | 安全响应头（HSTS/X-Frame/XSS） |
| `tingo-contrib/recovery` | panic 恢复（TP 风格） |
| `tingo-contrib/gzip` | 响应 gzip 压缩 |
| `tingo-contrib/logger` | HTTP 访问日志 |
| `tingo-contrib/static` | 静态文件服务 |
| `tingo-contrib/auth` | HTTP Basic / Bearer 鉴权 |
| `tingo-contrib/jwt` | JWT 认证 |
| `tingo-contrib/sessions` | 会话中间件 |
| `tingo-contrib/rate` | 令牌桶限流 |
| `tingo-contrib/ratelimit` | 分布式滑动窗口限流 |
| `tingo-contrib/csrf` | CSRF 防护 |
| `tingo-contrib/validate` | TP 风格参数校验 |
| `tingo-contrib/metric` | Prometheus 指标 |
| `tingo-contrib/trace` | 链路追踪 |
| `tingo-contrib/debug` | 错误调试页 |

使用示例：

~~~go
import (
    "github.com/xmszy/tingo-contrib/cors"
    "github.com/xmszy/tingo-contrib/jwt"
    "github.com/xmszy/tingo-contrib/recovery"
)

func Register(r t.Router) {
    r.Use(
        recovery.Middleware(),
        cors.Middleware(cors.Default()),
    )

    r.Group("/api", func(g t.Router) {
        g.Use(jwt.Middleware(jwt.Config{Secret: "s3cret"}))
        g.GET("/me", meHandler)
    })
}
~~~

## 内置限流与签名中间件

`net/thttp/middleware` 包内置对标 Tingo 常用能力的开箱即用中间件，无需引入 contrib 模块。

### 限流（RateLimit）

基于固定窗口令牌桶，默认按客户端 IP 限流，对标 Tingo 的 `Throttle`：

~~~go
import "github.com/xmszy/tingo/net/thttp/middleware"

// 默认：单 IP 每 60 秒 60 次
r.Use(middleware.RateLimit())

// 自定义阈值与 key（如按用户 token 限流）
r.Use(middleware.RateLimit(func(c *middleware.RateLimitConfig) {
    c.Limit = 100
    c.Window = time.Minute
    c.KeyFunc = func(ctx *t.Ctx) string {
        return ctx.GetHeader("X-User-Token")
    }
}))
~~~

> 进程内限流适用于单实例；分布式部署请实现 `middleware.RateLimiter` 接口（如基于 Redis），
> 通过 `c.Limiter` 注入。被限流时返回 `429` 并设置 `Retry-After` 头。

### 签名校验（Sign）

按 `app_key + timestamp + nonce + 参数` 做 HMAC-SHA256 签名校验，对标 Tingo 的 `CheckSign`：

~~~go
r.Use(middleware.Sign(func(c *middleware.SignConfig) {
    c.Secret = "my-secret"                      // 单一密钥
    c.IgnorePaths = []string{"/health", "/login"} // 白名单跳过
    c.TimeTolerance = 300 * time.Second         // 时间戳容差，防重放
}))

// 多租户 / 密钥轮换：按 app_key 返回对应密钥
r.Use(middleware.Sign(func(c *middleware.SignConfig) {
    c.GetSecret = func(appKey string) (string, bool) {
        return secretStore.Lookup(appKey)
    }
}))
~~~

客户端需按约定拼接并签名：`k1=v1&k2=v2&nonce=...`（按参数名升序，排除 `sign` 本身），
以 `secret` 做 HMAC-SHA256 后 hex 编码放入 `sign` 参数。

## 执行顺序

中间件按注册顺序执行，洋葱模型：

~~~
请求 →
  Middleware1.Before
    Middleware2.Before
      Handler
    Middleware2.After
  Middleware1.After
→ 响应
~~~

- `c.Next()` 调用下一个中间件或 handler
- `c.Abort()` 终止后续中间件，直接返回
