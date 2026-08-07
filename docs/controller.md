# 控制器定义

## 什么是控制器

控制器是 HTTP 请求的入口，负责接收参数、调用服务、返回响应。

Tingo 的控制器对应 Tingo 中的 `app/controller/` 目录，每个控制器文件定义一个结构体。

## 定义控制器

~~~go
// app/controller/user.go
package controller

import "github.com/xmszy/tingo/frame"

func init() {
    t.RegisterController("/user", &User{})  // 自注册到自动路由
}

type User struct {
    t.Controller  // 继承 BaseController，获得 Success/Error/Redirect 等方法
}
~~~

## 方法签名

Tingo 的控制器方法支持三种签名，`core.Adapt` 自动适配：

### 简单签名

~~~go
func (u *User) Hello(c *t.Ctx) {
    c.JSON(200, t.Map{"message": "Hello World"})
}
~~~

### 带请求绑定（推荐）

~~~go
type ListReq struct {
    Page    int    `form:"page,default=1"  binding:"min=1"`
    Size    int    `form:"size,default=20" binding:"min=1,max=100"`
    Keyword string `form:"keyword"`
}

type ListRes struct {
    Total int         `json:"total"`
    Items interface{} `json:"items"`
}

func (u *User) Index(c *t.Ctx, req *ListReq) (*ListRes, error) {
    items, total, err := u.svc.Search(req.Keyword, req.Page, req.Size)
    if err != nil {
        return nil, err
    }
    return &ListRes{Total: total, Items: items}, nil
}
~~~

- 入参 `*T`：自动绑定（uri → query → body），校验（binding tag），转换（default tag）
- 出参 `*T, error`：自动封装为 `{"data": T}` 或 `{"code": ..., "message": ...}`
- 入参 `*t.Ctx` 可选：用于需要 ctx 的场景

### 仅返回 error

~~~go
func (u *User) Delete(c *t.Ctx) error {
    id := c.Param("id")
    return u.svc.Delete(id)
}
~~~

## 参数绑定

支持的标签：

| 标签 | 说明 |
|---|---|
| `form:"name"` | 字段名 |
| `form:"name,default=1"` | 默认值 |
| `json:"name"` | JSON 字段名 |
| `uri:"id"` | URI 路径参数 |
| `binding:"min=1"` | 数值范围校验 |
| `binding:"required"` | 必填 |
| `binding:"min=1,max=100"` | 范围校验 |

支持的路径参数绑定：

~~~go
// GET /user/:id
r.GET("/user/:id", handler)

type ReadReq struct {
    ID string `uri:"id"`
}
func (u *User) Read(c *t.Ctx, req *ReadReq) (*UserRes, error) {
    // req.ID 自动绑定自路径参数 :id
}
~~~

## 上下文访问

控制器方法可以同时接收 `*t.Ctx` 和请求体：

~~~go
func (u *User) Save(c *t.Ctx, req *SaveReq) error {
    // c 提供完整上下文：获取当前用户、日志、配置等
    currentUser, _ := CurrentUser.Get(c)

    t.LogFrom(c).Infow("save user", "name", req.Name)
    return u.svc.Create(currentUser.ID, req)
}
~~~

## 自动路由

在 `init()` 中注册控制器，在 `route/app.go` 中一行调用 `t.AutoRoute(r)`：

~~~go
// controller/user.go
func init() {
    t.RegisterController("/user", &User{})
}

// controller/product.go
func init() {
    t.RegisterController("/product", &Product{})
}

// route/app.go
func Register(r t.Router) {
    r.Use(recovery.Middleware())
    t.AutoRoute(r)
}
~~~

`AutoRoute` 内部按控制器前缀长度降序注册，长前缀优先匹配。
