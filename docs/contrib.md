# 扩展库

Tingo 的 `tingo-contrib` 是独立模块（仓库 `github.com/xmszy/tingo-contrib`），提供中间件和工具组件，不进入核心依赖。原 `contrib/` 目录下的组件现全部位于该独立仓库。

## 已提供组件

### 安全

| 包 | 说明 |
|---|---|
| `tingo-contrib/cors` | 跨域资源共享中间件 |
| `tingo-contrib/secure` | 安全响应头（HSTS、X-Frame、XSS） |
| `tingo-contrib/csrf` | CSRF 防护（Cookie Double-Submit） |
| `tingo-contrib/jwt` | JWT 认证 |

### 限流

| 包 | 说明 |
|---|---|
| `tingo-contrib/rate` | 令牌桶限流（内存） |
| `tingo-contrib/ratelimit` | 分布式滑动窗口限流（Redis） |

### 工具

| 包 | 说明 |
|---|---|
| `tingo-contrib/validate` | TP 风格参数校验 |
| `tingo-contrib/recovery` | panic 恢复中间件 |
| `tingo-contrib/gzip` | 响应压缩 |
| `tingo-contrib/logger` | HTTP 访问日志 |
| `tingo-contrib/static` | 静态文件服务 |
| `tingo-contrib/auth` | HTTP Basic / Bearer 鉴权 |
| `tingo-contrib/sessions` | 会话管理中间件 |

### 可观测性

| 包 | 说明 |
|---|---|
| `tingo-contrib/metric` | Prometheus 指标导出 |
| `tingo-contrib/trace` | TraceID 注入 + 慢日志 |
| `tingo-contrib/debug` | Whoops 风格错误调试页 |

### 配置

| 包 | 说明 |
|---|---|
| `tingo-contrib/registry` | 服务注册发现（file 后端 + 接口） |

## 使用方式

~~~go
import (
    "github.com/xmszy/tingo-contrib/cors"
    "github.com/xmszy/tingo-contrib/jwt"
    "github.com/xmszy/tingo-contrib/recovery"
    "github.com/xmszy/tingo-contrib/validate"
)

func Register(r t.Router) {
    r.Use(
        recovery.Middleware(),
        cors.Middleware(cors.Default()),
    )

    r.Group("/api", func(g t.Router) {
        g.Use(jwt.Middleware(jwt.Config{Secret: "s3cret"}))
        // ...
    })
}
~~~

### CORS 配置

~~~go
cors.Middleware(cors.Config{
    AllowOrigins:     []string{"*"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
    ExposeHeaders:    []string{"Content-Length"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
})
~~~

### JWT 配置

~~~go
jwt.Middleware(jwt.Config{
    Secret:    "your-256-bit-secret",
    ExpiresIn: 24 * time.Hour,
    Issuer:    "tingo",
    Lookup:    "header:Authorization:Bearer ",  // 从 Header 获取
    // Lookup: "query:token",                   // 从 Query 获取
    // Lookup: "cookie:jwt",                    // 从 Cookie 获取
})
~~~

### 限流

~~~go
import "github.com/xmszy/tingo-contrib/rate"

r.Use(rate.Middleware(rate.Config{
    Rate:  10,            // 每秒 10 个请求
    Burst: 20,            // 突发容量 20
}))
~~~

### Prometheus 指标

~~~go
import "github.com/xmszy/tingo-contrib/metric"

// 注册指标端点
metric.Register(r)

// 访问 http://localhost:8080/metrics
~~~

### TraceID

~~~go
import "github.com/xmszy/tingo-contrib/trace"

r.Use(trace.Middleware(trace.Config{
    SlowThreshold: time.Second,  // 慢请求阈值
}))
~~~

请求自动注入 `X-Trace-ID` 响应头，慢请求记录 WARN 日志。

## 安装 contrib

~~~bash
go get github.com/xmszy/tingo-contrib
~~~

或在 `go.mod` 中添加：

~~~go
require github.com/xmszy/tingo-contrib v0.1.0
~~~
