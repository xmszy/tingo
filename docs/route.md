# 路由定义

Tingo 的路由系统支持三种定义方式：自动路由、约定路由、资源路由。

## 路由入口

路由定义在 `route/app.go` 中：

~~~go
package route

import "github.com/xmszy/tingo/frame"

func Register(r t.Router) {
    // 全局中间件
    r.Use(recovery.Middleware(), cors.Middleware(cors.Default()))

    // 路由定义
    t.AutoRoute(r)  // 自动路由
}
~~~

## 自动路由（推荐）

控制器在 `init()` 自注册，`route/app.go` 一行搞定：

~~~go
// controller/index.go
func init() {
    t.RegisterController("/", &Index{})
}

// controller/user.go
func init() {
    t.RegisterController("/user", &User{})
}

// route/app.go
func Register(r t.Router) {
    t.AutoRoute(r)
}
~~~

自动路由按控制器前缀长度降序注册，长前缀优先匹配。

## 约定路由

手动调用 `r.Controller(prefix, ctrl)`，方法名自动映射为 URL：

~~~go
r.Controller("/system", &SystemController{})

// SystemController 的方法：
// Index       → ANY /system
// GetInfo     → GET  /system/info
// PostClearCache → POST /system/clear_cache
// UserInfo    → ANY /system/user_info
~~~

> 方法名前缀 `Get`/`Post`/`Put`/`Delete`/`Patch` 自动识别为 HTTP 方法。

## 资源路由

~~~go
r.Resource("/users", &UserController{})
~~~

注册 7 个 RESTful 路由（只注册控制器上真实存在的方法）：

| 方法 | URL | 控制器方法 |
|---|---|---|
| GET | `/users` | `Index` |
| GET | `/users/create` | `Create` |
| POST | `/users` | `Save` |
| GET | `/users/:id` | `Read` |
| GET | `/users/:id/edit` | `Edit` |
| PUT | `/users/:id` | `Update` |
| DELETE | `/users/:id` | `Delete` |

## 手动路由

对于需要完全控制的情况，支持手动注册：

~~~go
// GET 路由
r.GET("/hello", func(c *t.Ctx) {
    c.JSON(200, t.Map{"message": "Hello"})
})

// 带路径参数
r.GET("/user/:id", func(c *t.Ctx) {
    id := c.Param("id")
    c.JSON(200, t.Map{"id": id})
})

// POST 路由
r.POST("/user", func(c *t.Ctx) {
    // ...
})

// 多方法
r.ANY("/health", func(c *t.Ctx) {
    c.String(200, "OK")
})

// 匹配所有方法
r.Match([]string{"GET", "POST"}, "/form", formHandler)
~~~

## 路由分组

~~~go
r.Group("/api", func(g t.Router) {
    // 分组中间件
    g.Use(authMiddleware)

    g.GET("/users", listUsers)
    g.POST("/users", createUser)
    // 嵌套分组
    g.Group("/v2", func(g2 t.Router) {
        g2.GET("/users", listUsersV2)
    })
})
~~~

## 路径参数

~~~go
// 命名参数
r.GET("/user/:id", handler)      // c.Param("id")

// 可选参数（Go 不支持 *，用正则替代）
r.GET("/user/:id/info", handler) // id 必填

// 通配路由
r.GET("/files/*filepath", handler) // c.Param("filepath")
~~~

## URL 生成

Tingo 通过 `url_gen` 表在注册期建立路由名到 URL 的映射，用于反向生成 URL：

~~~go
// 命名路由
r.GET("/user/:id", handler).Name("user.info")
r.GET("/user/list", listHandler).Name("user.list")

// 生成 URL
url := t.URL("user.info", map[string]string{"id": "1"})
// → /user/1

url = t.URL("user.list")
// → /user/list
~~~
