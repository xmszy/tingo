# 路由分组

路由分组用于对一组路由应用相同的中间件或路径前缀。

## 基本分组

~~~go
r.Group("/api", func(g t.Router) {
    // 组内所有路由自动加 /api 前缀
    g.GET("/users", listUsers)      // → /api/users
    g.POST("/users", createUser)    // → /api/users
    g.GET("/health", healthCheck)   // → /api/health
})
~~~

## 分组中间件

~~~go
r.Group("/api", func(g t.Router) {
    // 仅该分组内生效
    g.Use(jwt.Middleware(jwt.Config{Secret: "s3cret"}))
    g.Use(rateLimiter)

    g.GET("/me", meHandler)
    g.POST("/order", createOrderHandler)
})
~~~

## 嵌套分组

~~~go
r.Group("/api", func(g t.Router) {
    g.Use(authMiddleware)

    // V1 版本
    g.Group("/v1", func(v1 t.Router) {
        v1.GET("/users", listUsersV1)
    })

    // V2 版本（需要管理员权限）
    g.Group("/v2", func(v2 t.Router) {
        v2.Use(adminMiddleware)
        v2.GET("/users", listUsersV2)
    })
})

// 最终路由：
// /api/v1/users  → authMiddleware + listUsersV1
// /api/v2/users  → authMiddleware + adminMiddleware + listUsersV2
~~~

## 分组与资源路由

在分组内注册资源路由：

~~~go
r.Group("/admin", func(g t.Router) {
    g.Use(authMiddleware, adminMiddleware)

    g.Resource("/users", &AdminUserController{})
    g.Resource("/articles", &AdminArticleController{})
})

// 生成路由：
// GET    /admin/users           → AdminUserController.Index
// POST   /admin/users           → AdminUserController.Save
// GET    /admin/users/:id       → AdminUserController.Read
// ...
~~~

## 分组与控制器

~~~go
r.Group("/system", func(g t.Router) {
    g.Use(authMiddleware)

    g.Controller("/cache", &CacheController{})
    g.Controller("/config", &ConfigController{})
})

// 生成路由（CacheController）：
// ANY   /system/cache             → CacheController.Index
// GET   /system/cache/info        → CacheController.GetInfo
// POST  /system/cache/clear       → CacheController.PostClear
// ...
~~~

## 模块隔离（Module）

`Module` 是对标 Tingo「模块/控制器/动作」路径约定的便捷封装：自动套 `/{module}` 前缀
并隔离中间件组，模块级中间件仅作用于该模块内路由。

~~~go
r.Module("admin", func(m t.Router) {
    m.Use(adminMiddleware)          // 仅 admin 模块生效
    m.GET("/user", listUser)        // → /admin/user
    m.POST("/user", createUser)     // → /admin/user
})
~~~

`Module` 等价于 `Group("/admin", ...)`，区别在于语义清晰——调用处明确这是一个模块边界，
便于多人协作时按模块拆分路由文件。
