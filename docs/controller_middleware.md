# 控制器中间件

控制器级别的中间件允许为特定控制器注册中间件，仅对该控制器的路由生效。

## 声明控制器中间件

控制器实现 `MiddlewareDeclarer` 接口：

~~~go
type User struct {
    t.Controller
}

func (u *User) Middleware() []t.Handler {
    return []t.Handler{
        authMiddleware,
        logMiddleware,
    }
}
~~~

## 路由级 vs 控制器级 vs 方法级

Tingo 的中间件只在路由级别生效（gin radix 树特性），控制器中间件
在注册期展开为路由前缀级的中间件：

~~~go
// 控制器声明中间件
func (u *User) Middleware() []t.Handler {
    return []t.Handler{authMiddleware}
}

// t.RegisterController("/user", &User{})
// → 实际效果：给 /user* 路由组加上 authMiddleware
~~~

## 多个控制器的组合

~~~go
type AdminUser struct {
    t.Controller
}

func (a *AdminUser) Middleware() []t.Handler {
    return []t.Handler{authMiddleware, adminMiddleware}
}

type PublicPage struct {
    t.Controller
}

// PublicPage 不声明中间件 → 无需认证
~~~

## 排除特定方法

如果某控制器的部分方法不需要中间件，目前有两种方式：

### 方式一：拆分成两个控制器

~~~go
type PublicUser struct {
    t.Controller
}

// 公开方法，不需要登录
func (p *PublicUser) Login(c *t.Ctx) { /* ... */ }
func (p *PublicUser) Register(c *t.Ctx) { /* ... */ }

type User struct {
    t.Controller
}

func (u *User) Middleware() []t.Handler {
    return []t.Handler{authMiddleware}
}

// 需要登录的方法
func (u *User) Profile(c *t.Ctx) { /* ... */ }
func (u *User) Orders(c *t.Ctx) { /* ... */ }
~~~

### 方式二：在 handler 中手动判断

~~~go
func (u *User) Middleware() []t.Handler {
    return []t.Handler{
        func(c *t.Ctx) {
            // 排除登录和注册方法
            path := c.Path()
            if strings.HasSuffix(path, "/login") || strings.HasSuffix(path, "/register") {
                c.Next()
                return
            }
            checkAuth(c)
            c.Next()
        },
    }
}
~~~
