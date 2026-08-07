# 请求参数

Tingo 提供 Tingo 风格的请求参数读取，支持路由参数、Query 参数、表单和 JSON Body
的自动识别与取值，对应 Tingo 的 `$request->param()` / `$request->get()` / `$request->post()`。

## 创建请求对象

通过 `tapp.Req(c)` 从上下文创建请求读取器，这是零成本视图（仅持有一个指针），可放心在栈上创建：

~~~go
import "github.com/xmszy/tingo/tapp"

func (u *User) Index(c *core.Ctx) {
    req := tapp.Req(c)
    // ...
}
~~~

在 Tingo 风格控制器（继承 `t.Controller`）中，可直接用 `t.Req(c)` 助手：

~~~go
func (u *User) Index(c *t.Ctx) {
    req := t.Req(c)
    name := req.Param("name")
    // ...
}
~~~

## Param —— 综合取值

`Param` 方法按 Tingo `$request->param()` 的优先级取值：

~~~text
路由参数 < query < 请求体（JSON body 或表单）
~~~

后者覆盖前者。例如请求 POST `/user/1?name=张三` JSON body `{"name":"李四"}` 时，
`Param("name")` 返回 `"李四"`（JSON body 优先级最高）。

~~~go
func (u *User) Index(c *t.Ctx) {
    req := t.Req(c)
    // 综合取值
    id := req.Param("id")           // 路由参数或 query 参数
    name := req.Param("name")       // query 或请求体
    page := req.Param("page", "1")  // 带默认值
}
~~~

## Get —— 取 Query 参数

`Get` 方法仅从 URL Query 参数中取值，对应 `$request->get()`：

~~~go
// GET /users?page=2&size=20
page := req.Get("page", "1")    // "2"
size := req.Get("size", "10")   // "20"
keyword := req.Get("keyword")   // ""，不存在
~~~

## Post —— 取请求体参数

`Post` 方法从请求体中取值，**自动检测 JSON body**（Content-Type 包含 `json` 时）：

~~~go
// POST /users  JSON body: {"name":"张三","age":30}
name := req.Post("name")   // "张三"（来自 JSON body）
age := req.Post("age")     // "30"（JSON body）

// POST /users  表单: name=张三&age=30
name := req.Post("name")   // "张三"（来自表单）
~~~

对应 Tingo 中 `getInputData()` 自动解析 JSON 请求体的行为。

## JSON Body 支持

当请求 `Content-Type` 包含 `json` 时，`Param`、`Post`、`All`、`Strings`、`Has`
等方法自动从 JSON body 取值，无需额外配置：

~~~go
// POST /api/user  Content-Type: application/json
// Body: {"name":"王五","age":25,"tags":["go","php"],"city":"深圳"}
func (u *User) Save(c *t.Ctx) {
    req := t.Req(c)
    name := req.Param("name")       // "王五"
    age := req.Post("age")          // "25"
    tags := req.Strings("tags")     // ["go", "php"]
    hasCity := req.Has("city")      // true
    all := req.All()                // {"name":"王五","age":"25","tags":"[go php]","city":"深圳"}
}
~~~

### JSON 结构体绑定

`JSON` 方法直接将 JSON body 解析到结构体，替代手动取值：

~~~go
type SaveUser struct {
    Name string   `json:"name"`
    Age  int      `json:"age"`
    Tags []string `json:"tags"`
}

func (u *User) Save(c *t.Ctx) {
    var input SaveUser
    if err := t.Req(c).JSON(&input); err != nil {
        c.JSON(400, t.Map{"error": "解析失败"})
        return
    }
    // input.Name == "王五", input.Age == 25, input.Tags == ["go","php"]
}
~~~

### 获取原始 Body

`Body` 方法返回请求体的原始字节，支持重复读取：

~~~go
raw, err := req.Body()
// raw = []byte(`{"name":"张三","age":30}`)
~~~

~~~go
// 支持多次读取（框架会自动还原 Request.Body）
raw1, _ := req.Body()
raw2, _ := req.Body()
// raw1 和 raw2 内容相同
~~~

## Has —— 判断参数是否存在

~~~go
if req.Has("keyword") {
    // 处理搜索
}
~~~

## 类型转换

`Request` 提供便捷的类型转换方法，缺失时返回默认值：

~~~go
page := req.Int("page", 1)          // int
size := req.Int64("size", 20)       // int64
active := req.Bool("active", false)  // bool
score := req.Float64("score", 0.0)   // float64
~~~

## 批量取值

### Only

仅返回指定字段：

~~~go
data := req.Only("name", "age")  // map[string]string{"name":"张三","age":"30"}
~~~

### Exclude

排除指定字段，返回其余：

~~~go
data := req.Exclude("password", "token")  // 排除敏感字段后的全部参数
~~~

### Strings

取同名多值参数，支持 JSON body 数组字段：

~~~go
// ?ids=1&ids=2&ids=3 或 表单 ids=1&ids=2&ids=3
ids := req.Strings("ids")  // ["1", "2", "3"]

// JSON body: {"tags":["go","php","python"]}
tags := req.Strings("tags")  // ["go", "php", "python"]
~~~

### All

返回全部参数（路由 + query + 表单 + JSON body），后者覆盖前者：

~~~go
all := req.All()  // map[string]string
~~~

> 注意：`All()` 会分配 map，仅适合非热点路径。高频场景建议使用结构体绑定或 `Param/Post/Get` 单值方法。

## Input —— 从数据集取值

`Input` 方法从指定的 `map[string]string` 数据集中取值，对应 TP 的 `input()` 函数：

~~~go
// 从 All() 返回值中取
data := req.All()
name := req.Input(data, "name")

// 自动使用 All() 作为数据源
name := req.Input(nil, "name", "默认值")

// 从自定义数据集取
custom := map[string]string{"status": "active"}
status := req.Input(custom, "status")
~~~

## 完整例子

~~~go
package controller

import (
    "github.com/xmszy/tingo/core"
    "github.com/xmszy/tingo/frame"
)

type User struct {
    t.Controller
}

// Index GET /user
func (u *User) Index(c *core.Ctx) {
    req := t.Req(c)
    page := req.Int("page", 1)
    size := req.Int("size", 20)
    keyword := req.Param("keyword")
    // 查询逻辑...
}

// Save POST /user
func (u *User) Save(c *core.Ctx) {
    var input struct {
        Name  string `json:"name"`
        Age   int    `json:"age"`
        Email string `json:"email"`
    }
    if err := t.Req(c).JSON(&input); err != nil {
        c.JSON(400, t.Map{"error": "参数格式错误"})
        return
    }
    // 保存用户...
    c.JSON(200, t.Map{"message": "创建成功"})
}

// Delete DELETE /user/:id
func (u *User) Delete(c *core.Ctx) {
    req := t.Req(c)
    id := req.Param("id")
    if id == "" {
        c.JSON(400, t.Map{"error": "缺少id参数"})
        return
    }
    // 删除逻辑...
}
~~~
