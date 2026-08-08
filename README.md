# Tingo

**Tingo** 是一个贴近 ThinkPHP 开发范式、复用 gin 高性能内核的 Go Web 框架。

- **约定优于配置**：脚手架一键生成标准项目，`controller` / `model` / `app` 各安其位。
- **高性能**：路由与 HTTP 内核直接基于 gin，零额外抽象开销。
- **单 / 多应用一体**：同一份代码既可做单应用，也能按 `app/` 子目录拆成多应用同进程运行。
- **工程化**：内置 `tingo` CLI，覆盖脚手架、代码生成、数据库迁移与 CLI 任务。

> **完整开发手册**：[docs/index.md](./docs/index.md)

## 安装

```bash
go get github.com/xmszy/tingo
```

脚手架工具（按需安装）：

```bash
go install github.com/xmszy/tingo/cmd/tingo@latest
```

## 快速开始

用脚手架一步生成标准项目：

```bash
tingo init myproject
cd myproject
tingo run
```

生成的 `main.go` 只需匿名导入应用并启动：

```go
package main

import (
    "log"

    "github.com/xmszy/tingo"
    _ "myproject/app" // 触发应用 init() 注册
)

func main() {
    if err := tingo.Run(); err != nil {
        log.Fatal(err)
    }
}
```

一个最简控制器（`app/controller/index.go`）：

```go
package controller

import t "github.com/xmszy/tingo/frame"

func init() { t.RegisterController("/", &Index{}) }

type Index struct{ t.Controller }

func (*Index) Index(c *t.Ctx) {
    c.String("hello, Tingo!")
}
```

### 单应用 vs 多应用

Tingo 的项目默认是**单应用**（一个 `app/` 包）；当站点需要前台 / 后台 / 接口同进程运行时，可加 `--multi-app` 拆成多个子应用。

| 维度 | 单应用 | 多应用 |
| --- | --- | --- |
| 生成 | `tingo init myproject` | `tingo init myproject --multi-app`（或 `-a`） |
| 结构 | 单个 `app/` 包 | `app/index/`、`app/admin/` 等子包，各自 `t.App()` 注册 |
| 入口 | 一个 `main.go` 匿名导入 `app` | 一个 `main.go` + `app/applications.go` 聚合导入各子应用 |
| 路由 | 互不隔离 | 由 `config/app.toml` 的 `[app]` 段按 url 段 / 域名分流 |
| 适用 | 简单站点、单一服务 | 前后台一体、页面与接口同站 |

多应用示例：

```bash
tingo init myapp --multi-app
cd myapp && go mod tidy && go run main.go
# GET /         -> index 应用
# GET /admin/   -> admin 应用
```

> 详见 [docs/multi_app.md](./docs/multi_app.md)。

## 常用命令

```bash
tingo run                              # 启动开发服务器
tingo build                            # 编译生产版本
tingo make controller user             # 生成控制器
tingo gen model                        # 从数据库生成模型
```

## 进一步阅读

| 主题 | 文档 |
| --- | --- |
| 安装与配置 | [docs/install.md](./docs/install.md) |
| 目录结构 | [docs/directory.md](./docs/directory.md) |
| 路由 | [docs/route.md](./docs/route.md) |
| 控制器 | [docs/controller.md](./docs/controller.md) |
| 请求 / 响应 | [docs/request.md](./docs/request.md) · [docs/response.md](./docs/response.md) |
| 配置 | [docs/config.md](./docs/config.md) |
| 数据库 / 模型 | [docs/database.md](./docs/database.md) · [docs/model.md](./docs/model.md) |
| 中间件 / 扩展库 | [docs/middleware.md](./docs/middleware.md) · [docs/contrib.md](./docs/contrib.md) |
| 多应用 | [docs/multi_app.md](./docs/multi_app.md) |
| 命令行 (CLI) | [docs/cli.md](./docs/cli.md) |
| 架构与设计 | [docs/architecture.md](./docs/architecture.md) |
| 完整手册目录 | [docs/index.md](./docs/index.md) |
