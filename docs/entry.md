# 入口文件

## main.go

Tingo 的入口文件非常简洁，类似 Tingo 的 `public/index.php`：

~~~go
package main

import (
    "log"

    "github.com/xmszy/tingo"
    _ "myproject/app" // 多应用：聚合导入，触发各子应用 init() 注册
)

func main() {
    if err := tingo.Run(); err != nil {
        log.Fatal(err)
    }
}
~~~

## run 命令

开发时可直接使用 CLI 命令：

~~~bash
# 启动开发服务器
tingo run

# 指定端口
tingo run --addr :9090

# 启用热重载（文件变更自动重启）
tingo run --watch
~~~

`tingo run --watch` 通过 mtime 轮询检测文件变化，自动重新编译并重启服务。

## build 命令

构建生产环境二进制：

~~~bash
# 基本构建
tingo build

# 指定输出路径
tingo build --output bin/server

# 交叉编译
tingo build --platform linux/amd64

# 生成 Dockerfile
tingo build --docker
~~~

## 程序化启动

除了 `tingo.Run()`，也可以通过 `tingo` 包进行程序化控制：

~~~go
package main

import (
    "context"
    "os/signal"
    "syscall"

    "github.com/xmszy/tingo"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    if err := tingo.RunContext(ctx); err != nil {
        panic(err)
    }
}
~~~
