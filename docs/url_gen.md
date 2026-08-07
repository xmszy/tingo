# URL 生成

Tingo 支持通过命名路由反向生成 URL。

## 命名路由

注册路由时使用 `Name` 方法命名：

~~~go
// 手动路由命名
r.GET("/user/:id", handler).Name("user.info")
r.GET("/user/list", listHandler).Name("user.list")

// 资源路由命名
r.Resource("/articles", &ArticleController{}).Name("article")
~~~

## 生成 URL

### 基本用法

~~~go
url := t.URL("user.info", map[string]string{"id": "1"})
// → /user/1

url = t.URL("user.list")
// → /user/list
~~~

### 资源路由 URL

~~~go
t.URL("article.index")                // → /articles
t.URL("article.read", "id=5")         // → /articles/5
t.URL("article.edit", "id=5")         // → /articles/5/edit
t.URL("article.create")               // → /articles/create
~~~

## 在视图中使用

Tingo 的模板引擎支持 URL 生成函数：

~~~html
<a href="{{ url "user.info" "id" 1 }}">查看用户</a>
<a href="{{ url "article.index" }}">文章列表</a>
~~~

## 带域名的 URL

生成包含域名的完整 URL：

~~~go
// 根据当前请求自动添加协议和域名
fullURL := t.FullURL(c, "user.info", map[string]string{"id": "1"})
// → http://localhost:8080/user/1
~~~

## 在控制器中重定向

~~~go
func (u *User) Save(c *t.Ctx) {
    // ... 创建用户 ...
    return u.Redirect(c, t.URL("user.info", "id=1"))
}
~~~
