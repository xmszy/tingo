# 路由参数

## 变量规则

Tingo 的路由参数底层基于 gin 的 radix 树路径参数：

~~~go
// 命名参数
r.GET("/user/:id", handler)       // :id 匹配任何字符直到下一个 /
r.GET("/user/:id/info", handler)  // 多级路径参数

// 通配参数
r.GET("/files/*filepath", handler) // *filepath 匹配剩余所有路径
r.GET("/static/*path", staticHandler)
~~~

获取路径参数：

~~~go
func handler(c *t.Ctx) {
    id := c.Param("id")           // 字符串
    filepath := c.Param("filepath") // 通配参数
}
~~~

## 参数约束（正则验证）

Gin 支持在路由中嵌入正则约束：

~~~go
// 仅匹配数字
r.GET("/user/:id", handler)
// 在 gin 中注册为 /user/:id，由 middlewares 做类型校验

// 推荐做法：在 handler 中校验
func handler(c *t.Ctx) {
    id := c.Param("id")
    if _, err := strconv.Atoi(id); err != nil {
        c.JSON(400, t.Map{"error": "id 必须为数字"})
        return
    }
}
~~~

## 请求参数优先级

当请求参数可以通过多种来源获取时，Tingo 遵循 Tingo 的优先级规则：

### 结构体绑定

~~~go
type ListReq struct {
    Page    int    `form:"page,default=1"  binding:"min=1"`
    Size    int    `form:"size,default=20" binding:"min=1,max=100"`
    Keyword string `form:"keyword"`
}
~~~

参数绑定优先级：`URI → Query → Body`，后者覆盖前者。

### 手动取值（tapp.Request）

~~~go
req := t.Req(c)
name := req.Param("name")  // 优先级：路由 < query < 请求体
page := req.Get("page")    // 仅 query
age := req.Post("age")     // 仅请求体（支持 JSON body）
~~~

## 可选参数

通过设置默认值实现：

~~~go
req := t.Req(c)
page := req.Int("page", 1)       // 缺省为 1
size := req.Int("size", 20)      // 缺省为 20
keyword := req.Param("keyword")  // 缺省为空字符串
~~~

## 数组参数

同名多值参数：

~~~go
// URL: /users?ids=1&ids=2&ids=3
ids := req.Strings("ids")  // ["1", "2", "3"]

// 表单: ids=1&ids=2&ids=3
ids := req.Strings("ids")  // ["1", "2", "3"]

// JSON body: {"tags":["go","php","python"]}
tags := req.Strings("tags")  // ["go", "php", "python"]
~~~
