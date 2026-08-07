# 资源路由

资源路由是 RESTful API 开发的最快方式，注册 7 个标准路由。

## 基本用法

~~~go
r.Resource("/users", &UserController{})
~~~

## 注册的路由

| 方法 | URL | 控制器方法 | 说明 |
|---|---|---|---|
| GET | `/users` | `Index` | 列表 |
| GET | `/users/create` | `Create` | 新增页面 |
| POST | `/users` | `Save` | 保存 |
| GET | `/users/:id` | `Read` | 详情 |
| GET | `/users/:id/edit` | `Edit` | 编辑页面 |
| PUT | `/users/:id` | `Update` | 更新 |
| DELETE | `/users/:id` | `Delete` | 删除 |

> Tingo **只注册控制器上真实存在的方法**，未定义的方法不会注册为路由。

## 只读资源路由

~~~go
// 仅 Index 和 Read 方法（只读）
type UserController struct {
    t.Controller
}

func (u *UserController) Index(c *t.Ctx) { /* 列表 */ }
func (u *UserController) Read(c *t.Ctx) { /* 详情 */ }
// Save、Update、Delete 未定义 → 不会注册 POST/PUT/DELETE 路由
~~~

## 限制资源路由

如果希望只暴露部分路由，可以只实现对应方法：

~~~go
type ArticleController struct {
    t.Controller
}

func (a *ArticleController) Index(c *t.Ctx)    { /* GET /articles */ }
func (a *ArticleController) Read(c *t.Ctx)     { /* GET /articles/:id */ }
func (a *ArticleController) Save(c *t.Ctx)     { /* POST /articles */ }
// 不实现 Create、Edit、Update、Delete
// → 只注册 GET /articles、GET /articles/:id、POST /articles
~~~

## 嵌套资源路由

~~~go
r.Resource("/users/:uid/articles", &ArticleController{})

// 在 handler 中获取父级 ID：
func (a *ArticleController) Index(c *t.Ctx) {
    uid := c.Param("uid")
    // 列出用户 uid 的所有文章
}
~~~

## 命名路由

~~~go
r.Resource("/users", &UserController{}).Name("user")

// URL 生成
t.URL("user.index")          // /users
t.URL("user.read", "id=1")   // /users/1
t.URL("user.edit", "id=1")   // /users/1/edit
~~~
