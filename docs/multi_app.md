# 多应用

> 适用版本：tingo v0.2.0+（含 `init --multi-app`、`make app`、配置驱动应用调度）

tingo 的“多应用”指**同一进程内**同时运行多个可独立部署的子应用（如 `index` 前台、`admin` 后台、`api` 接口），通过统一入口和配置路由到不同子应用。它不同于“多模块”（每个模块独立进程/独立域名部署）。

## 与多模块的区别

| 维度 | 多模块 | 多应用 |
|------|--------|--------|
| 部署 | 每个模块独立进程、独立端口/域名 | 同一进程、统一端口 |
| 入口 | 各模块各自 `main.go` | 单一 `main.go` + 聚合导入 |
| 路由 | 互不相关 | 通过 url 段 / 域名 / 默认应用分流 |
| 适用 | 大型拆分、团队隔离 | 前后台一体、接口与页面同站 |

> 单应用的“多模块”通过 `tingo init <name>`（默认）生成的单 `app/` 即可；本文只讲多应用。

## 快速开始

```bash
# 生成一个 index + admin 双应用骨架
tingo init myapp --multi-app
cd myapp
go mod tidy
go run main.go
```

- 访问 `http://localhost:8080/` → `index` 应用
- 访问 `http://localhost:8080/admin/` → `admin` 应用

也可简写：

```bash
tingo init myapp -a
```

## 目录约定

```
myapp/
├── main.go                      # 入口，调用 t.Boot()
├── config/
│   └── app.toml                 # [app] 段定义应用调度
├── app/
│   ├── applications.go          # 聚合导入，触发各子应用 init() 注册
│   ├── index/                   # index 应用
│   │   ├── app.go               # t.App("index", App{})
│   │   └── controller/index.go
│   └── admin/                   # admin 应用
│       ├── app.go
│       └── controller/index.go
└── go.mod
```

每个子应用是一个独立 Go package（`package index`、`package admin`），在 `init()` 中通过 `t.App()` 注册：

```go
// app/index/app.go
package index

import (
	"myapp/app/index/controller"
	t "github.com/xmszy/tingo/frame"
)

type App struct {
	t.BaseApp
}

func (App) Routes(r t.Router) {
	r.Controller("/", &controller.IndexController{})
}

func init() {
	t.App("index", App{})
}
```

## 应用调度（config/app.toml 的 [app] 段）

多应用不靠命令行逐个挂载，而由 `config/app.toml` 的 `[app]` 段统一调度：

```toml
[app]
default_app    = "index"     # 默认应用（未匹配到其它应用时使用）
auto_multi_app = true        # true: 自动按 app/ 下的子目录名识别应用
# app_map    = { "m" = "admin" }   # url 段别名: /m/... -> admin 应用
# domain_bind = { "admin.example.com" = "admin" }  # 域名绑定: 该域名 -> admin 应用
# deny_app   = ["common"]    # 禁止通过 url 直接访问的应用（仅作内部复用）
```

`scheduling` 字段与 TP 的 `deny_app_list` 作用相同：`deny_app` 中的应用不会作为“可通过 url 直接访问”的应用被自动挂载。

调度解析在引擎 Boot 阶段完成（`core.AppConfigProvider`），最终为每个应用解析出 `Prefix` / `Domain` / `Default` / `Disabled` 等 `AppConfig`。

### 访问规则

| 配置 | 访问方式 | 命中应用 |
|------|----------|----------|
| `default_app = "index"` | `GET /` | `index` |
| 子应用 `admin` | `GET /admin/` | `admin` |
| `app_map = { "m" = "admin" }` | `GET /m/` | `admin` |
| `domain_bind = { "admin.example.com" = "admin" }` | `Host: admin.example.com` | `admin` |

## 聚合导入（Go 的限制）

Go 是编译型语言，**无法在运行时扫描目录自动注册应用**。因此需要通过“聚合导入”显式触发各子应用的 `init()`：

```go
// app/applications.go
package app

import (
	_ "myapp/app/index"
	_ "myapp/app/admin"
)
```

`tingo make app <name>` 在新建子应用时会自动维护该文件的匿名导入。

## 新增一个应用

```bash
# 在现有多应用项目中新增 api 应用
tingo make app api
```

该命令会生成：

```
app/api/
├── app.go
└── controller/index.go
```

并自动在 `app/applications.go` 追加 `_ "myapp/app/api"` 匿名导入。

随后（可选）在 `config/app.toml` 配置调度，例如把 `/api/` 绑定到 `api` 应用，或加 `app_map` / `domain_bind`。

## 在子应用内生成代码（make 的 @ 语法）

参考 ThinkPHP 的 `php think make:controller admin/User`，tingo 用 `@` 表示“应用@名称”：

```bash
# 在 admin 应用内生成 User 控制器
tingo make controller admin@User

# 等价于显式 --app
tingo make controller User --app admin
```

`@` 内联语法优先于 `--app` 参数。生成的文件位于 `app/<应用名>/controller/<Name>.go`。

支持 `controller` 与 `app` 类型；更多类型见 `tingo make -h`。

## 纯 API 应用

若某子应用只提供接口、不含页面，可直接用 `r.GET/r.POST` 注册，或使用 `t.AutoRoute` 自动映射 controller 方法到路由，无需 `t.Controller` 页面包装。

## 常见问题

**Q：为什么访问 `/admin` 404？**
A：路由注册在 `/admin/`（带尾斜杠）。访问 `/admin` 由 `RedirectTrailingSlash` 自动跳转；若自定义了 `app_map`，需用别名段访问。

**Q：新增应用后路由不生效？**
A：检查 `app/applications.go` 是否已匿名导入该应用包（`make app` 会自动维护）。未导入则 `init()` 不执行，`t.App()` 未注册。

**Q：多应用与单应用能否切换？**
A：`tingo init <name>`（不加 `--multi-app`）生成单应用骨架；已生成的单应用项目可手工按本文目录约定改造成多应用，或重新 `init --multi-app` 到新目录迁移。
