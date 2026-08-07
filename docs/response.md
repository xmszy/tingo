# 响应

Tingo 提供多种响应方式，涵盖 JSON、HTML、重定向、文件下载等。

## 控制器快捷方法

控制器内嵌 `t.Controller` 后，可使用以下快捷方法：

### Success —— JSON 成功响应

~~~go
func (u *User) Index(c *t.Ctx) {
    u.Success(c, users)
}
// → {"code":0,"data":[...],"message":"success"}

// 自定义消息
u.Success(c, users, "获取成功")
// → {"code":0,"data":[...],"message":"获取成功"}
~~~

### Error —— JSON 错误响应

~~~go
func (u *User) Save(c *t.Ctx) {
    if err != nil {
        u.Error(c, "保存失败", 500)
        return
    }
}
// → {"code":500,"data":null,"message":"保存失败"}
~~~

### Result —— 自定义 JSON 响应

~~~go
u.Result(c, 0, "自定义消息", t.Map{"id": 1})
// → {"code":0,"message":"自定义消息","data":{"id":1}}
~~~

### Redirect —— 重定向

~~~go
func (u *User) Save(c *t.Ctx) {
    // 保存成功，重定向到列表
    u.Redirect(c, "/user/list")
}

// 延迟跳转（Tingo 风格）
u.Redirect(c, "/user/list", 3)  // 3 秒后跳转，显示倒计时页面
~~~

## Ctx 直接响应

继承自 gin 的响应方法：

### JSON

~~~go
c.JSON(200, t.Map{"message": "OK"})
c.JSON(200, gin.H{"data": result})  // gin.H 也是 t.Map 的别名
c.JSONPretty(200, data, "  ")       // 格式化 JSON
~~~

### 纯文本

~~~go
c.String(200, "Hello, %s", name)
~~~

### HTML

~~~go
c.HTML(200, "template.html", t.Map{"name": "张三"})

// 配合 tview 布局
html, _ := t.View().RenderIn("layout", "page", data)
c.Data(200, "text/html; charset=utf-8", []byte(html))
~~~

### XML

~~~go
c.XML(200, data)
~~~

### 无内容

~~~go
c.Status(204)  // No Content
c.String(200, "")  // 空内容
~~~

## 文件下载

### 文件下载

~~~go
func (u *Export) Csv(c *t.Ctx) {
    c.File("/path/to/export.csv")
}
~~~

### 强制下载

~~~go
func (u *Export) Csv(c *t.Ctx) {
    c.FileAttachment("/path/to/report.pdf", "report.pdf")
}
~~~

### 内存数据下载

~~~go
func (u *Export) Csv(c *t.Ctx) {
    c.Data(200, "application/csv; charset=utf-8", []byte(csvContent))
    c.Header("Content-Disposition", `attachment; filename="export.csv"`)
}
~~~

## Header 设置

~~~go
c.Header("X-Custom", "value")
c.Header("Cache-Control", "no-cache")
c.Header("Access-Control-Allow-Origin", "*")
~~~

## 全局响应格式化

Tingo 的 `core.Responder` 接口控制全局响应格式：

~~~go
// app/exception.go
type ExceptionHandle struct {
    tapp.ExceptionHandle
}

func (e *ExceptionHandle) Render(c *core.Ctx, err error) {
    // 自定义全局错误响应格式
    if te, ok := err.(*errors.Error); ok {
        c.JSON(te.Status, t.Map{
            "code":    te.Code,
            "message": te.Message,
            "data":    te.Meta,
        })
        return
    }
    c.JSON(500, t.Map{
        "code":    500,
        "message": "系统错误",
    })
}
~~~

成功时同样通过 `responder` 控制格式。

## 响应绑定

控制器方法的返回值自动封装为响应：

~~~go
type UserRes struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

// 返回结构体 → 自动包装为 {"code":0,"data":{...},"message":"success"}
func (u *User) Read(c *t.Ctx) (*UserRes, error) {
    // ...
    return &UserRes{ID: 1, Name: "张三"}, nil
}

// 返回 error → 自动转为错误响应
func (u *User) Save(c *t.Ctx) error {
    // ...
    return errors.New("业务错误")
}
~~~
