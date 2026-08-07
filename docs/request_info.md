# 请求信息

Tingo 通过 `*core.Ctx`（即 `*t.Ctx`）提供完整的请求信息访问。

## 请求方法

~~~go
func handler(c *t.Ctx) {
    method := c.Method()           // "GET", "POST", "PUT", "DELETE"
    c.IsGet()                      // bool
    c.IsPost()                     // bool
    c.IsPut()                      // bool
    c.IsDelete()                   // bool
    c.IsPatch()                    // bool
    c.IsOption()                   // bool
    c.IsAjax()                     // 是否为 Ajax 请求
}
~~~

## URL 信息

~~~go
func handler(c *t.Ctx) {
    path := c.Path()               // "/user/list"
    rawQuery := c.Query()          // "page=1&size=20"
    fullPath := c.FullPath()       // 匹配的路由模板 "/user/:id"
    host := c.Host()               // "localhost:8080"
    scheme := c.Scheme()           // "http" 或 "https"
    ip := c.ClientIP()             // 客户端 IP
}
~~~

## Header 信息

~~~go
func handler(c *t.Ctx) {
    c.GetHeader("Authorization")
    c.GetHeader("Content-Type")
    c.RequestHeader()              // http.Header
    contentType := c.ContentType()
    c.IsWebsocket()                // 是否为 WebSocket
}
~~~

## 路径参数

~~~go
// 路由: /user/:id/post/:post_id
r.GET("/user/:id/post/:post_id", handler)

func handler(c *t.Ctx) {
    id := c.Param("id")            // 路径参数
    pid := c.Param("post_id")
}
~~~

## 获取原始请求体

~~~go
func handler(c *t.Ctx) {
    raw, err := c.Body()           // []byte，支持重复读取
    // raw = `{"name":"张三","age":30}`
}
~~~

## 获取原始请求

~~~go
func handler(c *t.Ctx) {
    req := c.Request()             // *http.Request
    w := c.Writer()                // http.ResponseWriter
}
~~~

## 请求类型判断

在 `tapp.Request` 中提供了以下判断方法：

~~~go
req := t.Req(c)
req.IsGet()
req.IsPost()
req.IsPut()
req.IsDelete()
req.IsPatch()
req.IsHead()
req.IsAjax()
req.IsPjax()
req.IsJson()     // Content-Type 含 json
req.IsMobile()   // User-Agent 判断
~~~
