# 视图

Tingo 的模板引擎 `tview` 基于 `html/template`，支持布局渲染、模板继承、局部模板。

## 目录结构

视图文件默认放在 `app/view/` 目录：

~~~
app/view/
  index.html        # 控制器对应的视图
  layout/
    default.html    # 默认布局
    admin.html      # 管理后台布局
  public/
    header.html     # 公共头部
    footer.html     # 公共底部
    msg.html        # 提示消息
    error.html      # 错误页
    exception.html  # 异常页
  user/
    profile.html    # 用户资料页
    login.html      # 登录页
~~~

## 模板语法

Tingo 使用标准 Go `html/template` 语法：

~~~html
<!-- 输出变量 -->
<h1>{{ .Title }}</h1>
<p>{{ .Content }}</p>

<!-- 条件判断 -->
{{if .IsAdmin}}
    <a href="/admin">管理后台</a>
{{else}}
    <span>普通用户</span>
{{end}}

<!-- 循环 -->
<ul>
{{range .Users}}
    <li>{{ .Name }} - {{ .Email }}</li>
{{end}}
</ul>

<!-- 变量赋值 -->
{{$name := .CurrentUser.Name}}
{{$name}}

<!-- 管道 -->
<p>{{ .Content | html }}</p>
~~~

## 模板变量

在控制器中赋值：

~~~go
func (u *Page) Index(c *t.Ctx) {
    // 方式一：控制器的 Fetch/VAssign
    u.Assign("title", "首页")
    u.Assign("users", users)
    u.Fetch()

    // 方式二：直接渲染
    html, err := t.View().Render("index", t.Map{
        "title": "首页",
        "users": users,
    })
    if err != nil {
        c.String(500, "模板渲染失败")
        return
    }
    c.Data(200, "text/html; charset=utf-8", []byte(html))
}
~~~

## 布局渲染

使用 `RenderIn` 将内容嵌入到布局中：

~~~go
// 将 index.html 的内容渲染到 layout/default.html 的 {{ .Content }} 位置
html, err := t.View().RenderIn("layout/default", "index", data)
~~~

布局模板：

~~~html
<!-- layout/default.html -->
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }}</title>
    {{template "public/header" .}}
</head>
<body>
    <div class="container">
        {{ .Content }}  <!-- 子模板内容 -->
    </div>
    {{template "public/footer" .}}
</body>
</html>
~~~

## 控制器快捷渲染

继承 `t.Controller` 的控制器可直接使用渲染方法：

~~~go
func (u *Page) Index(c *t.Ctx) {
    u.Assign("title", "首页")
    u.Assign("users", users)

    // 渲染并输出（自动拼接 view/ 前缀和 .html 后缀）
    u.Fetch()

    // 使用指定布局
    u.Fetch("layout/admin")
}
~~~

~~~go
// 整个控制器共用布局
type Page struct {
    t.Controller
    layout string
}

func (p *Page) Initialize(c *t.Ctx) {
    p.layout = "layout/default"
}

func (p *Page) Index(c *t.Ctx) {
    p.Assign("title", "首页")
    p.Fetch(p.layout)
}
~~~

## 模板文件配置

~~~toml
# view.toml
root = "view"       # 模板根目录
ext = ".html"       # 模板文件后缀
~~~
