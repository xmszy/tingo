# 路径搜索

`tspath` 提供路径搜索和常用路径快捷方法，零外部依赖。

## 文件搜索

### Search —— 在目录列表中搜索文件

~~~go
import "github.com/xmszy/tingo/frame"

paths := []string{"./config", "./app/admin/config", "/etc/myapp"}
file, err := t.PathSearch(paths, "database.toml")
if err != nil {
    // 在所有路径中都没找到
}
fmt.Println(file) // "./config/database.toml"
~~~

按传入的目录顺序依次搜索，返回第一个匹配的文件路径。

### SearchGlob —— Glob 模式搜索

~~~go
file, err := t.PathSearchGlob(paths, "*.toml")
// 返回第一个匹配的 .toml 文件路径
~~~

## 常用路径快捷方法

### Home —— 用户主目录

~~~go
import "github.com/xmszy/tingo/os/tspath"

home, err := tspath.Home()
// Linux: /home/username
// macOS: /Users/username
// Windows: C:\Users\username
~~~

### HomeOrPwd —— 主目录或工作目录

获取用户主目录，如果失败则返回当前工作目录：

~~~go
dir := tspath.HomeOrPwd()
~~~

### WorkDir —— 当前工作目录

~~~go
wd := tspath.WorkDir()
~~~

### ExeDir —— 可执行文件所在目录

~~~go
exeDir := tspath.ExeDir()
// 返回可执行文件所在的目录路径
~~~

## 在框架中的应用

路径搜索常用于配置文件的查找：

~~~go
paths := []string{
    "./config",
    tspath.HomeOrPwd() + "/.config/myapp",
    "/etc/myapp",
}

cfgFile, err := t.PathSearch(paths, "app.toml")
if err != nil {
    panic("找不到配置文件 app.toml")
}
~~~

## 完整函数表

| 函数 | 说明 |
|---|---|
| `t.PathSearch(paths, name)` | 在目录列表中搜索文件 |
| `t.PathSearchGlob(paths, pattern)` | Glob 模式搜索 |
| `tspath.Home()` | 用户主目录 |
| `tspath.HomeOrPwd()` | 主目录（fallback 工作目录） |
| `tspath.WorkDir()` | 当前工作目录 |
| `tspath.ExeDir()` | 可执行文件目录 |
