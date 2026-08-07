# 文件工具

`tfile` 提供常用文件操作函数，零外部依赖。

## 用户目录

### Home —— 获取用户主目录

~~~go
import "github.com/xmszy/tingo/frame"

home, err := t.FileHome()
fmt.Println(home) // C:\Users\username 或 /home/username
~~~

### HomeDir —— 主目录（含 fallback）

获取用户主目录，如果失败则返回当前工作目录：

~~~go
dir := t.FileHomeDir()
// 绝对不会返回空字符串
~~~

## 文件大小格式化

### FormatSize —— 字节数转可读格式

~~~go
t.FileFormatSize(0)           // "0 B"
t.FileFormatSize(1024)        // "1.0 KB"
t.FileFormatSize(1048576)     // "1.0 MB"
t.FileFormatSize(1536000)     // "1.5 MB"
t.FileFormatSize(1073741824)  // "1.0 GB"
~~~

### ReadableSize —— 读取文件并格式化

~~~go
size := t.FileReadableSize("data.bin")
fmt.Println(size) // "2.3 MB"
~~~

## 文件内容替换

### ReplaceInFile —— 正则替换

原地替换文件中的正则匹配内容：

~~~go
// 将文件中的 HTTP 链接替换为 HTTPS
err := t.FileReplaceIn("config/config.toml", `http://`, "https://")
~~~

### ReplaceStrInFile —— 字符串替换

原地替换文件中的纯字符串：

~~~go
err := t.FileReplaceStrIn("config/app.toml", "localhost:8080", "0.0.0.0:80")
~~~

> 如果文件内容未发生变化，不会执行写操作，避免修改时间戳无意义更新。

## 文件排序

### SortFiles —— 按名称/时间/大小排序

~~~go
files, _ := filepath.Glob("logs/*.log")

// 按名称升序
t.FileSortFiles(files, t.FileSortByName, false)

// 按修改时间降序（最新的在前）
t.FileSortFiles(files, t.FileSortByTime, true)

// 按大小降序
t.FileSortFiles(files, t.FileSortBySize, true)
~~~

排序方式常量：

| 常量 | 说明 |
|---|---|
| `t.FileSortByName` | 按文件名排序 |
| `t.FileSortByTime` | 按修改时间排序 |
| `t.FileSortBySize` | 按文件大小排序 |

## 完整函数表

| 函数 | 说明 |
|---|---|
| `t.FileHome()` | 用户主目录 `(string, error)` |
| `t.FileHomeDir()` | 主目录含 fallback |
| `t.FileFormatSize(size)` | 字节数格式化 |
| `t.FileReadableSize(path)` | 文件可读大小 |
| `t.FileReplaceIn(path, old, new)` | 正则内容替换 |
| `t.FileReplaceStrIn(path, old, new)` | 字符串替换 |
| `t.FileSortFiles(files, by, desc)` | 文件排序 |
