# URL 访问

## URL 解析规则

Tingo 默认采用 Tingo 风格的 URL 解析：

~~~
http://host:port/[应用前缀/][控制器/][操作][/参数]
~~~

示例：

| URL | 应用 | 控制器 | 操作 |
|---|---|---|---|
| `/` | 默认应用 | Index | Index |
| `/user` | 默认应用 | User | Index |
| `/user/info` | 默认应用 | User | Info |
| `/admin/` | admin | Index | Index |
| `/admin/index` | admin | Index | Index |
| `/admin/user/list` | admin | User | List |

> Index 操作（索引动作）默认映射到**控制器根路径**（如 `/admin`），
> 同时框架也会额外注册 `/admin/index` 这条「控制器/方法」写法，两者等价，
> 因此 `/admin`、`/admin/`、`/admin/index` 均可访问首页。

## 大小写

控制器名和操作名默认不区分大小写。URL 中的下划线会自动转换为驼峰：

| URL | 控制器.方法 |
|---|---|
| `/user_info` | `UserInfo.Index` |
| `/user_info/get_list` | `UserInfo.GetList` |

## URL 后缀

默认不限制 URL 后缀。可通过 `route.toml` 配置：

~~~toml
# 允许 .html 后缀
url_suffix = true
url_html_suffix = "html"
~~~

访问 `/user.html` 等同于 `/user`。

## 路由模式

### 资源路由

~~~go
r.Resource("/users", &UserController{})
~~~

自动注册 7 个 RESTful 路由。

### 约定路由

~~~go
r.Controller("/system", &SystemController{})
~~~

方法名自动映射为 URL。

### 自动路由

~~~go
// 控制器在 init() 自注册
func init() {
    t.RegisterController("/", &Index{})
}

// route/app.go 一行搞定
func Register(r t.Router) {
    t.AutoRoute(r)
}
~~~

详见 [路由](./route.md)。
