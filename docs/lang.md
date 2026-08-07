# 多语言

Tingo 的国际化组件 `tlang` 支持多语言翻译。

## 配置

~~~toml
# lang.toml
default = "zh-cn"
auto_detect = true
detect_var = "lang"
path = "lang"
~~~

## 语言文件

语言文件放在 `app/lang/` 目录，按语言目录组织：

~~~
app/lang/
  zh-cn/
    common.json
    user.json
    article.json
  en-us/
    common.json
    user.json
    article.json
~~~

JSON 内容示例：

~~~json
// zh-cn/user.json
{
    "name": "用户名",
    "email": "邮箱",
    "password": "密码",
    "login": "登录",
    "register": "注册",
    "user_not_found": "用户不存在",
    "login_success": "登录成功"
}
~~~

~~~json
// en-us/user.json
{
    "name": "Username",
    "email": "Email",
    "password": "Password",
    "login": "Login",
    "register": "Register",
    "user_not_found": "User not found",
    "login_success": "Login successful"
}
~~~

## 基本用法

### 获取翻译

~~~go
import "github.com/xmszy/tingo/frame"

// 全局翻译
msg := t.Lang("user.login_success")

// 带参数
msg := t.Lang("user.welcome", t.LangP("name", "张三"))

// 请求级翻译（自动检测语言）
msg := t.LangFrom(c, "user.login_success")
~~~

### 分类加载

~~~go
// 加载特定分类
t.LangLoad("user")

// 获取特定分类翻译
msg := t.Lang("user.login_success")

// 公共分类
msg := t.Lang("common.save")
msg := t.Lang("common.delete")
~~~

## 语言检测

### 自动检测

配置 `auto_detect = true` 后，框架自动从以下来源检测语言：

1. URL 参数：`?lang=en-us`
2. Cookie：`lang=en-us`
3. Accept-Language Header

### 手动切换

~~~go
func handler(c *t.Ctx) {
    // 切换到英文
    t.LangSwitch(c, "en-us")

    // 使用翻译
    msg := t.LangFrom(c, "user.login_success")  // "Login successful"
}
~~~

## 视图中使用

~~~html
<p>{{ lang "user.name" }}</p>
<p>{{ lang "user.welcome" (lang_p "name" .UserName) }}</p>
~~~

控制器中赋值：

~~~go
func (u *User) Login(c *t.Ctx) {
    msg := t.LangFrom(c, "user.login_success")
    u.Success(c, nil, msg)
}
~~~
