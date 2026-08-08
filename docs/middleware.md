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
