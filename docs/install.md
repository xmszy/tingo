# 安装

## 环境要求

- Go 1.23+
- 数据库驱动（按需选择）：MySQL/PostgreSQL/SQLite/SQL Server

## 创建项目

使用 `tingo init` 脚手架命令创建新项目：

~~~bash
mkdir myproject && cd myproject
go mod init myproject
go run github.com/xmszy/tingo/cmd/tingo@latest init
~~~

或本地开发时：

~~~bash
# 在 tingo 仓库内
go run ./cmd/tingo init myproject
~~~

## 目录结构

初始化后的项目结构：

~~~
myproject/
  go.mod
  main.go
  .env.example

  app/                          # 应用目录
    app.go                      # 应用入口（注册 + 路由委托）
    kernel.go                   # 内核（中间件/服务/事件/Provider）
    exception.go                # 异常处理器
    common.go                   # 公共函数
    provider.go                 # 服务提供者

    controller/                 # 控制器目录
      index.go                  # 默认控制器
      base.go                   # 基类控制器

    model/                      # 模型目录
    middleware/                  # 中间件目录
      auth.go                   # 认证中间件示例
    service/                    # 服务目录
    validate/                   # 验证器目录
    view/                       # 视图目录

  config/                       # 全局配置
    app.toml
    database.toml
    log.toml
    route.toml
    session.toml
    view.toml

  route/                        # 路由定义
    app.go

  public/static/                # 静态资源
  runtime/                      # 运行时目录（日志/缓存）
~~~

## 启动服务

~~~bash
# 开发模式（热重载）
tingo run

# 或直接编译运行
go run main.go

# 指定地址
tingo run --addr :8080

# 构建
tingo build
~~~

启动后访问 `http://localhost:8080` 即可看到欢迎页面。

## 多应用模式

Tingo 原生支持多应用，每个应用可以有自己的配置、路由、控制器、模型等。
通过 `tingo init` 的 `--multi-app` 标志创建双应用骨架：

~~~bash
# 创建 index + admin 双应用骨架
tingo init myapp --multi-app

# 之后新增子应用
tingo make app api
~~~

各子应用通过 `app/applications.go` 的匿名导入触发 `init()` 注册，
应用调度由 `config/app.toml` 的 `[app]` 段统一决定。详见 [多应用模式](./multi_app.md)。
