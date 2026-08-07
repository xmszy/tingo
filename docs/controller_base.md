# 基础控制器

Tingo 的 `t.Controller` 提供了 Tingo BaseController 的常用方法，
所有控制器应当内嵌它。

## 基础控制器定义

~~~go
// app/controller/base.go
package controller

import "github.com/xmszy/tingo/frame"

type Base struct {
    t.Controller
}
~~~

## 内置方法

### Success —— 成功响应

~~~go
func (u *User) Index(c *t.Ctx) {
    data := map[string]string{"name": "张三"}
    u.Success(c, data)  // {"code":0,"data":{"name":"张三"},"message":"success"}
}
~~~

### Error —— 错误响应

~~~go
func (u *User) Save(c *t.Ctx) {
    if name == "" {
        u.Error(c, "用户名不能为空", 400)
        return
    }
}
// → {"code":400,"data":null,"message":"用户名不能为空"}
~~~

### Result —— 自定义响应

~~~go
func (u *User) Login(c *t.Ctx) {
    u.Result(c, 0, "登录成功", t.Map{"token": token})
}
// → {"code":0,"message":"登录成功","data":{"token":"xxx"}}
~~~

### Redirect —— 重定向

~~~go
func (u *User) Save(c *t.Ctx) {
    // ... 保存成功后重定向
    u.Redirect(c, "/user/list")
}

// 带跳转提示
u.Redirect(c, "/user/list", 2)  // 延迟 2 秒跳转
~~~

### Bind —— 参数绑定

~~~go
func (u *User) Save(c *t.Ctx) error {
    var input SaveReq
    if err := u.Bind(c, &input); err != nil {
        return err  // 框架自动返回 400
    }
    // 使用 input...
    return nil
}
~~~

### BindValidate —— 参数绑定 + 校验

~~~go
func (u *User) Save(c *t.Ctx) error {
    var input SaveReq
    // 先绑定参数，再用 rules 校验
    if err := u.BindValidate(c, &input, rules); err != nil {
        return err
    }
    return nil
}
~~~

## Initialize —— 控制器初始化钩子

控制器实现 `Initializer` 接口后，每个请求处理前会调用 `Initialize`：

~~~go
type User struct {
    t.Controller
    svc *UserService
}

func (u *User) Initialize(c *t.Ctx) {
    // 每个请求前执行，可做权限校验、获取当前用户等
    user, err := resolveUser(c)
    if err != nil {
        tapp.Abort(401, "未登录")
    }
    u.svc = NewUserService(user)
}

func (u *User) Index(c *t.Ctx) {
    // u.svc 已初始化，直接使用
}
~~~

## 控制器中间件

控制器可实现 `MiddlewareDeclarer` 声明自己的中间件：

~~~go
func (u *User) Middleware() []t.Handler {
    return []t.Handler{
        authMiddleware,
        logMiddleware,
    }
}
~~~

详情见 [控制器中间件](./controller_middleware.md)。

## 完整示例

~~~go
// app/controller/user.go
package controller

import "github.com/xmszy/tingo/frame"

func init() {
    t.RegisterController("/user", &User{})
}

type User struct {
    t.Controller
    svc *UserService
}

func (u *User) Middleware() []t.Handler {
    return []t.Handler{authMiddleware}
}

func (u *User) Initialize(c *t.Ctx) {
    user := resolveUser(c)
    u.svc = NewUserService(user)
}

// GET /user
func (u *User) Index(c *t.Ctx) {
    list, _ := u.svc.List()
    u.Success(c, list)
}

// GET /user/:id
func (u *User) Read(c *t.Ctx) {
    id := c.Param("id")
    user, err := u.svc.Find(id)
    if err != nil {
        u.Error(c, "用户不存在", 404)
        return
    }
    u.Success(c, user)
}

// POST /user
func (u *User) Save(c *t.Ctx) error {
    var input SaveReq
    if err := u.Bind(c, &input); err != nil {
        return err
    }
    if err := u.svc.Create(&input); err != nil {
        return err
    }
    u.Success(c, nil)
    return nil
}
~~~
