# tingo

Tingo 风格的 Go Web 框架。

- **开发范式**对齐 Tingo：单应用默认、配置驱动、模型、资源路由
- **工程能力**借鉴 GoFrame：组件化、门面包、代码生成
- **性能内核**复用 gin：radix 路由树 + 零成本上下文转换

```
module github.com/xmszy/tingo
```

> **📖 完整开发手册**：[docs/index.md](./docs/index.md) —— 对标 Tingo 8.0 手册结构，涵盖序言/基础/架构/路由/控制器/请求/响应/数据库/模型/视图/错误日志/验证/杂项/命令行/扩展库/附录。

## 快速开始

```go
package main

import (
    "log"

    "github.com/xmszy/tingo"
    _ "myproject/app"
)

func main() {
    if err := tingo.Run(); err != nil {
        log.Fatal(err)
    }
}
```

## 工程结构（Tingo 单应用）

```text
app/
  app.go
  config/             # 默认应用配置，覆盖根 config 的同名配置
    app.toml
  route/
    app.go
  controller/
  model/
  service/
  middleware/
  validate/
  view/
config/                # 进程级全局配置
  app.toml
  database.toml
  route.toml
  log.toml
  session.toml
  view.toml
public/static/
runtime/
main.go
```

默认应用实现 `t.Application`，`app.go` 只负责注册应用并委托 `app/route`：

```go
func init() { t.App("app", &App{}) }

type App struct{}

func (*App) Routes(r t.Router) { route.Register(r) }
```

应用前缀、域名、默认状态等元数据写在 `app/config/app.toml`，无需硬编码 `Config()`：

```toml
prefix = "/"
default = true
```

`main` 只匿名导入 `app` 并调用 `tingo.Run()`。`tingo make app <name>` 会按相同约定生成命名应用（详见 [多应用文档](./docs/multi_app.md)）：

```text
app/
  applications.go      # 聚合导入，触发各子应用 init() 注册
  admin/
    app.go
    controller/
  model/
  service/
  middleware/
  validate/
  view/
```

## 配置作用域

框架启动时自动加载配置，不需要业务代码手工注册目录。默认读取 `.toml`；可通过系统环境变量或 `.env` 中的 `CONFIG_EXT` 统一切换为 `toml`、`yaml`、`yml`、`json` 或 `ini`：

```dotenv
CONFIG_EXT=ini
```

切换后，全局与所有应用只加载该后缀的文件；配置内容和文件名需要同步转换，例如 `config/app.ini`。同一个命名空间不会混合加载多种格式。文件名就是命名空间：

- `config/app.toml` → `app.*`
- `config/database.toml` → `database.*`
- `app/config/app.toml` → 默认应用视图中的 `app.*`
- `app/admin/config/database.toml` → `admin` 应用视图中的 `database.*`

INI 支持根键与点分节，点分节会转换为嵌套配置树：

```ini
default=mysql

[connections.mysql]
type=mysql
hostname=127.0.0.1
hostport=3306
```

`.env` 默认采用 Tingo 的平面变量命名，数据库使用 `DB_*`：

```dotenv
APP_DEBUG=true
DB_TYPE=mysql
DB_HOST=127.0.0.1
DB_NAME=test
DB_USER=username
DB_PASS=password
DB_PORT=3306
DB_CHARSET=utf8
```

仍支持 Tingo 风格分节；例如 `[DATABASE] HOSTNAME=...` 会规范化为环境变量 `DATABASE_HOSTNAME`。点路径读取按相同规则规范化：

```dotenv
[APP]
DEBUG=(true)
DEFAULT_TIMEZONE=Asia/Shanghai
```

配置内容支持严格环境插值。`${DB_TYPE:-mysql}` 在变量缺失或为空时使用默认值，`${DB_PASS:-}` 表示允许空默认值；没有默认值的 `$VAR` 或 `${VAR}` 若缺失会返回包含配置来源和变量名的启动错误，不再静默展开为空字符串。

```go
debug := t.Env("app.debug", false)
host, exists := t.EnvLookup("db.host")
value := t.EnvValue("db.password") // (empty) 返回空字符串
_ = t.EnvHas("db.host")
_ = t.EnvExpand("${DB_HOST}:3306")
_ = t.EnvMap("app.options")
```

`(true)`、`(false)`、`(null)`、`(empty)` 按 Tingo 语义解析；`Lookup/Has` 可以区分变量缺失与变量存在但为空。系统环境变量优先于 `.env`，加载文件不会覆盖已经存在的变量。

全局配置和应用配置按字段深合并，应用只需声明差异项；未覆盖的嵌套字段不会被归零。固定优先级为：

```text
框架默认值 < config/*.toml < 应用 config/*.toml < 环境变量/占位符 < 显式 Go Option
```

HTTP 地址、运行模式等进程级选项只读取全局配置。应用元数据、业务配置和数据库连接可以读取合并后的应用视图。

```go
// 全局配置
addr := t.Config().String("app.server.addr", ":8080")

// 指定应用的合并配置
name := t.ConfigFor("admin").String("app.name", "admin")

// handler 中读取当前请求所属应用的合并配置
func Index(c *t.Ctx) {
    cfg := t.ConfigFrom(c)
    pageSize := cfg.Int("app.pagination.page_size", 20)
    c.JSON(t.Map{"page_size": pageSize})
}
```

配置树支持 `Get`、`Lookup`、`Has`、`String`、`Bool`、`Int`、`Int64`、`Float64`、`Strings` 和 `DecodeAt`，路径可包含切片索引（例如 `servers.0.host`）。`Data`、全局配置与应用配置始终返回独立快照；请求作用域通过 `*t.Ctx` 显式解析，不依赖 goroutine-local 或可变的“当前应用”。运行时动态数据应使用缓存或独立 Store，而不是修改启动配置。

初始化项目默认生成六类有真实消费者的配置：

| 文件 | 职责与消费者 |
|---|---|
| `config/app.toml` | 应用名、调试、时区与 HTTP 服务参数；启动生命周期和 HTTP 引擎消费 |
| `config/database.toml` | 默认连接与命名连接；数据库管理器按应用作用域消费 |
| `config/route.toml` | 尾斜杠、固定路径、405 与路由元信息开关；HTTP 路由引擎消费 |
| `config/log.toml` | 级别、异步、缓冲、格式、前缀及 HTTP 访问日志；默认 HTTP 中间件和 `t.LogConfigured*` 消费 |
| `config/session.toml` | Cookie 名、有效期、Path、Secure、HttpOnly；`t.SessionConfigured*` 消费 |
| `config/view.toml` | 模板根目录、后缀和定界符；`t.ViewConfigured*` 消费 |

Tingo 只生成有真实消费者的配置。Tingo 的 `cache.php`、`console.php`、`cookie.php`、`filesystem.php`、`lang.php`、`middleware.php`、`trace.php`，以及 PHP 闭包、动态路径、Think 模板标签、数据库主从等字段目前不会被伪装成已支持能力；对应组件具备完整运行语义后再加入正式配置 schema。

```go
logger, err := t.LogConfiguredFor("admin")
sessions, err := t.SessionConfiguredFor("admin")
views := t.ViewConfiguredFor("admin")
```

HTTP 不会自行扫描 `config` 目录，而是在生命周期 Boot 后读取统一配置注册表；因此 `CONFIG_EXT`、深合并、应用作用域和优先级只有一条运行时数据流。默认访问日志也在 Boot 后从全局 `log.access` 配置一次性装配：

```toml
[access]
enabled = true
skip_paths = ["/detect/version"]
```

`enabled = false` 会关闭默认访问日志；`skip_paths` 只过滤日志输出，不注册路由、不吞掉请求，也不改变响应状态。例如未注册的 `/detect/version` 命中跳过规则后仍会真实返回 404。

## 路由

### 资源路由

`r.Resource("/users", ctrl)` 按 Tingo 约定注册，**只注册控制器上真实存在的方法**：

| 方法 | 路径 | 控制器动作 |
|---|---|---|
| GET | `/users` | `Index` |
| GET | `/users/create` | `Create` |
| POST | `/users` | `Save` |
| GET | `/users/:id` | `Read` |
| GET | `/users/:id/edit` | `Edit` |
| PUT | `/users/:id` | `Update` |
| DELETE | `/users/:id` | `Delete` |

### 约定式路由

`r.Controller("/system", ctrl)` 将方法名映射为 URL（驼峰转下划线，等价于 Tingo 的 `url_convert`）：

| 方法名 | 路由 |
|---|---|
| `Index` | `ANY /system` |
| `GetInfo` | `GET /system/info` |
| `PostClearCache` | `POST /system/clear_cache` |
| `UserInfo` | `ANY /system/user_info` |

方法名前缀（`Get`/`Post`/`Put`/`Delete`/`Patch`）会被识别为 HTTP 方法；`Getter` 这类词不会被误判。

## 控制器

推荐 `(ctx, req) → (res, error)` 签名，绑定、校验、错误映射、响应封装全部由框架完成：

```go
type ListReq struct {
    Page    int    `form:"page,default=1"  binding:"min=1"`
    Size    int    `form:"size,default=20" binding:"min=1,max=100"`
    Keyword string `form:"keyword"`
}

func (u *User) Index(c *t.Ctx, req *ListReq) (*ListRes, error) {
    items, total, err := u.svc.List(req.Keyword, req.Page, req.Size)
    if err != nil {
        return nil, err
    }
    return &ListRes{Total: total, Items: items}, nil
}
```

参数按 `uri → query → body` 依次绑定，后者覆盖前者（对应 Tingo 的 `$request->param()`）。

### 请求参数获取

Tingo 提供 Tingo 风格的请求参数读取，自动识别路由参数、Query、表单和 JSON Body：

~~~go
func (u *User) Save(c *t.Ctx) {
    req := t.Req(c)                        // 创建请求读取器（零成本视图）

    name := req.Param("name")              // 综合取值：路由 < query < 请求体
    page := req.Get("page", "1")           // 仅从 Query 取值
    age := req.Post("age")                 // 从请求体取值（支持 JSON body）
    id := req.Int("id", 0)                 // 类型转换

    hasKeyword := req.Has("keyword")       // 判断是否存在
    all := req.All()                       // 全部参数

    // JSON 结构体绑定
    var input struct {
        Name string `json:"name"`
        Age  int    `json:"age"`
    }
    if err := req.JSON(&input); err != nil { /* ... */ }
}
~~~

**关键特性**：当 Content-Type 包含 `json` 时，`Param`/`Post`/`All` 等方法自动解析 JSON body，对应 TP 的 `getInputData()` 行为。

> 完整文档见 [docs/request.md](./docs/request.md)。

### 支持的 handler 签名

| 签名 | 开销 |
|---|---|
| `func(*t.Ctx)` | 零开销 |
| `func(*gin.Context)` | 零开销 |
| `func(*t.Ctx) error` | 零开销 |
| `t.W(func(*t.Ctx, *Req) (Res, error))` | 零反射，`sync.Pool` 复用 Req |
| `func(*t.Ctx, *Req) (Res, error)` | 注册期反射分析，运行时按预编译方案执行 |

性能敏感路径建议用前四种。

## 错误

错误即数据，携带 HTTP 状态码与业务码，可直接序列化：

```go
var ErrUserNotFound = t.NewError(404, "USER_NOT_FOUND", "用户不存在")

// 派生副本，不污染包级变量
return nil, ErrUserNotFound.WithMessagef("用户 %s 不存在", id)
return nil, ErrEmailTaken.WithMeta("email", email)
```

完全兼容标准库：`errors.Is` / `errors.As` / `%w` 均可用，`Is` 按业务码比较。

## 类型安全的上下文键

```go
var CurrentUser = t.Key[*model.User]("auth.user")

CurrentUser.Set(c, u)
u, ok := CurrentUser.Get(c)   // 无需类型断言
```

## 性能

设计目标：**不慢于 gin**。核心机制：

1. **零成本上下文** —— `type Ctx gin.Context`，两者内存布局完全一致，指针转换是编译期 no-op；
2. **零成本 handler 转换** —— `func(*Ctx)` 与 `func(*gin.Context)` 表示相同，函数值直接重解释，不引入包装层；
3. **注册期展开** —— 多应用、控制器、中间件在启动时全部 flatten 进 gin 的 radix tree，运行时无字符串解析与动态分发；
4. **元信息零分配** —— `App()`/`Controller()`/`Action()` 通过注册期建立的只读查找表解析，不写 gin 的 `Keys` map（该 map 惰性分配约 400B/3 allocs）。

隔离测量（同一 gin.Engine，仅 handler 包装方式不同）：

```
BenchmarkNoop_Gin                32.28 ns/op
BenchmarkNoop_Tingo              32.59 ns/op
BenchmarkRaw_GinHandler          83.30 ns/op
BenchmarkRaw_TingoDirectCast     83.29 ns/op     Ctx 方法无额外成本
BenchmarkRaw_TingoViaGinOf       84.59 ns/op
```

端到端对照中，**所有场景的 allocs/op 与 gin 完全一致**。

### 性能门禁

性能基准测试已抽离到独立仓库 **[tingo-benchmark](https://github.com/xmszy/tingo-benchmark)**，
其中 `allocs/op` 是硬性指标，任何回归都会导致失败。详见该仓库 README。


### Tingo 风格装饰器 W 的成本量化

`W` / `WN` 装饰器把 `func(*Ctx, *Req)(*Res, error)` 业务方法统一收口到 `responder` 协议，
相比裸 `func(*Ctx)` 直接 `c.JSON`，多一层「反射/泛型调用 + responder 包装 + 结构化响应体」。
基准见 tingo-benchmark 仓库的 `BenchmarkTingo_StaticHandler_*`，口径为 httptest 真实 HTTP
（与性能门禁一致，含 net/http 栈开销，故绝对值高于隔离口径）。

| 写法 | allocs/op（含 net/http） | 说明 |
|---|---|---|
| 裸 handler（`c.JSON(fixture)`） | 72 | 零反射、零包装 |
| `WN(func(c) (payload, error){ return fixture, nil })` | 79 | 多 7 次分配 |

`WN` 装饰器相比裸 handler 多约 7 次分配，主要来自：① `responder.Reply` 构造结构化响应体
（`{"code","message","data"}`）；② 业务返回值经 `any` 装箱传入 responder。

**已落地优化**：`responder.Reply` 的 `gin.H` map 改为 `sync.Pool` 复用（见 `core/handler.go`），
消除每次成功响应分配一个 map 的开销；`W` 的请求结构体清零由
`*req = *new(Req)` 改为 `var zero Req; *req = zero`（栈上零值，消除每请求临时堆分配）。

取舍建议：性能敏感的纯 API 路由用裸 `func(*Ctx)` 签名（零反射、最低 allocs，与 gin 持平）；
需要 Tingo 式「入参自动绑定 + 统一响应结构 + 错误链式派生」的业务路由用 `W`/`WN`，
成本已知且可控。两者在注册期均已展开进 gin radix 树，运行时无动态分发。


## 关于 unsafe

框架仅在 `core.ginOf` / `core.HandlerOf` 使用 `unsafe`，用于函数值重解释。其前提由构建时的测试守护：

- `TestCtxLayoutCompatible` —— 校验 `Ctx` 与 `gin.Context` 的大小与对齐一致
- `TestGinOfIdentity` / `TestHandlerOfIdentity` —— 校验转换后收到同一指针、状态互通

gin 一旦升级导致布局变化，测试会立即失败。

## 基础组件（os 包）

业务代码可经 `t` 门面使用基础组件；需要自定义配置来源时直接依赖 `os/tcfg` 的稳定契约。

### 配置 tcfg —— 只读、可适配、类型安全

```go
//go:embed config.toml
var raw []byte

cfg, err := t.ConfigWithBytes("toml", raw)
if err != nil {
    return err
}
name := cfg.String("app.name", "tingo")
port := cfg.Int("server.port", 8080)
```

`Config` 是非泛型只读门面，底层只依赖 `Adapter`：

```go
type Adapter interface {
    Available(ctx context.Context, resource ...string) bool
    Get(ctx context.Context, path string) (any, error)
    Data(ctx context.Context) (tcfg.Tree, error)
}
```

内置 `ContentAdapter` 支持 `Tree` 及 TOML、YAML、JSON、INI 内容；`FileAdapter` 支持显式文件叠加或显式目录与统一后缀。框架的约定目录仍由配置注册表负责，不会自动搜索工作目录、二进制目录等不确定来源。

`Data` 和路径读取返回独立快照，调用方不能修改适配器缓存。点路径支持 map 与切片索引；`DecodeAt` 和 `Loader[T]` 使用同一套弱类型转换，因此 INI 中的数字可绑定到字符串端口等目标字段。

#### Loader[T]（强类型绑定与显式监听）

`Loader[T]` 是唯一的泛型配置入口，负责把整树或指定子路径绑定为强类型快照：

```go
type ServerConf struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}

loader, err := t.ConfigLoader[ServerConf](cfg, "server")
if err != nil {
    return err
}
server := loader.Get()
```

需要热更新时，显式使用支持监听的 `FileAdapter`，并为监听器命名：

```go
adapter, err := tcfg.NewFileAdapter("config/app.toml")
if err != nil {
    return err
}
loader, err := t.ConfigLoaderWithAdapter[ServerConf](adapter, "server")
if err != nil {
    return err
}
loader.
    OnChange(func(next ServerConf) error { return apply(next) }).
    OnWatchError(func(_ context.Context, err error) { report(err) })
if err := loader.Watch(ctx, "http-server", 5*time.Second); err != nil {
    return err
}
defer loader.StopWatch()
```

文件变化时适配器先构建完整新快照，再原子替换缓存；解析或绑定失败会进入 `OnWatchError`。默认配置注册表仍是启动期快照，只有显式调用 `Watch` 的组件参与热更新。配置内核不提供含糊的运行期 `Set`，动态数据应进入缓存、容器或独立 Store。

`GetEffective` 的固定优先级为“环境变量 > Adapter > 默认值”，点路径会转换为大写下划线环境键；消费层的显式 Go Option 仍在其后覆盖。

### 日志 tlog —— 结构化、可异步

```go
l := t.NewLogger(t.LogConfig{Async: true, Level: t.LogDebug})
l.Infow("http request", t.LogF("path", "/x"), t.LogF("ms", 3))

// 包级便捷函数（默认 logger）
t.LogInfow("event", t.LogF("user", "ada"))
```

零外部依赖（不引入 fatih/color、otel）；同步/异步双模式；分级 + 结构化字段 + caller 行号。
访问日志中间件 (`middleware.Logger(middleware.LoggerWithLog(l))`) 走同一通道。

### 缓存 tcache —— 并发安全、支持过期

```go
c := t.CacheNew(t.Options{MaxEntries: 10000, SweepInterval: time.Minute})
t.CacheSet(c, "k", 42, time.Minute)
v, ok := t.CacheGet[int](c, "k")
v, _ = c.GetOr("price", time.Minute, func() (any, error) { return fetchPrice() }) // 回源模式
```

分片（默认 256）降低锁竞争；惰性删除 + 可选后台清扫；容量上限时按分片近似 LRU 淘汰。

### 环境变量 tenv —— 泛型读取

```go
port := t.Env[int]("PORT", 8080)                  // 缺失/解析失败回退默认
name := t.EnvMust[string]("APP_NAME")             // 缺失则 panic
m := tenv.GetMap("FEATURES")                      // "a=1,b=2" → map
```


## 数据库与模型

全局连接维护在 `config/database.toml`；需要隔离时，应用可在 `app/config/database.toml` 或 `app/<name>/config/database.toml` 只覆盖差异字段。MySQL、PostgreSQL、SQLite、SQL Server 驱动已由框架自动注册：

```toml
default = "${DB_DRIVER:-mysql}"

[connections.mysql]
type = "${DB_TYPE:-mysql}"
hostname = "${DB_HOST:-127.0.0.1}"
database = "${DB_NAME:-}"
username = "${DB_USER:-root}"
password = "${DB_PASS:-}"
hostport = "${DB_PORT:-3306}"
charset = "${DB_CHARSET:-utf8mb4}"
prefix = "${DB_PREFIX:-}"
```

从数据库生成模型：

```bash
tingo gen model
tingo gen model --tables user,order
tingo gen model --connection report
```

生成的 `app/model/user.go` 同时包含字段、表名和查询入口：

```go
user, err := model.NewUser().WhereEQ("id", 1).First()
users, err := model.NewUser().Where("age > ?", 18).Order("id DESC").All()
_, err = model.NewUser().WhereEQ("id", 1).Update(map[string]any{"status": 1})

// 命名连接
reports, err := model.NewReport("report").All()
```

底层仍是 `tdb.Model[T]` 和 `database/sql`。生成模型使用 `t.Database()` 获取全局默认连接；应用隔离场景使用 `t.DatabaseFor("admin")`，handler 中使用 `t.DatabaseFrom(c)`。高级场景可用 `t.DBOpen()` 与 `tdb.Open()` 显式控制连接；`Update`/`Delete` 无 WHERE 时仍由安全护栏拒绝。

## 视图 tview —— 模板渲染（M1+）

零外部依赖（基于 `html/template`），自动转义防 XSS；支持布局继承与区段：

```go
v := t.ViewNew("./views", t.ViewWithExt(".html"))
html, _ := v.Render("user/profile", map[string]any{"name": "ada"})        // 单模板
html, _ = v.RenderIn("layout", "user/profile", data)                      // 布局 + 子模板
v.Share("app_name", "tingo")                                             // 注入全局变量
v.Funcs(template.FuncMap{"upper": strings.ToUpper})                      // 自定义模板函数
```

内置模板函数：`raw`、`default`、`upper/lower/title`、`trim`、`replace`、`join`、`hasPrefix/hasSuffix/contains`。
`raw` 用于输出可信 HTML（不转义）；`default` 处理空值。

## 会话 tsession —— 服务端状态（M1+）

存储驱动抽象：内置 `MemoryStore`（基于 tcache），可换 `DBStore`（基于 tdb 的 `sessions` 表）。
客户端仅持有会话 ID 的 Cookie 信封，数据存于服务端：

```go
mgr := t.SessionNew(t.SessionConfig{CookieName: "sid", TTL: 24 * time.Hour})
sess, _ := mgr.LoadOrCreate(cookieID)          // 请求开始
sess.Set("uid", 1)
mgr.Save(sess)                                  // 请求结束
name, _ := t.SessionGet[string](sess, "name")  // 泛型读取
```

## 国际化 tlang —— 多语言（M1+）

零依赖，支持 `{name}` 占位符与 `{0}` 位置占位符，缺失 key 回退到 fallback 语言：

```go
tr := t.LangNew("zh", "en")
tr.Add("zh", map[string]string{"welcome": "你好 {name}"})
tr.Translate("welcome", map[string]any{"name": "ada"})   // 你好 ada
```

## 事件 tevent / 队列 tqueue —— 解耦（M1+）

类型安全事件总线，支持同步/异步分发、一次性监听、panic 恢复：

```go
bus := t.BusNew(false)
ev := t.EventNew[UserCreated]("user.created")
t.BusSubscribe(bus, ev, func(_ context.Context, p UserCreated) error { return nil })
t.BusDispatch(bus, ctx, ev, UserCreated{ID: 1})
```

内存任务队列基于事件总线解耦，支持失败重试与死信：

```go
q := t.QueueNew[EmailJob](false, 3)            // 最多重试 3 次
q.Subscribe(func(_ context.Context, m t.QueueMessage[EmailJob]) error { return send(m.Payload) })
q.Publish(ctx, EmailJob{To: "a@b.c"})
```

## 定时任务 tcron —— 调度（M1+）

内置 5 字段 cron 解析（分 时 日 月 周），无忙等精确调度：

```go
c := t.CronNew(nil)
c.Add("backup", "0 3 * * *", func() { backup() })   // 每天 3:00
c.Start()
defer c.Stop()
```

## 代码生成 tcodegen + CLI（M1+）

`cmd/tingo` 是框架命令行工具，子命令一览：

```bash
# 项目脚手架（Tingo 风格单应用）
go run ./cmd/tingo init [name]
go run ./cmd/tingo make controller user
go run ./cmd/tingo make model user

# 数据库反向生成单层模型
go run ./cmd/tingo gen model
go run ./cmd/tingo gen model --tables user,order

# 多应用是显式扩展能力
go run ./cmd/tingo init myapp --multi-app   # 生成 index/admin 双应用骨架
go run ./cmd/tingo make app api             # 新增子应用

# 运行与维护
go run ./cmd/tingo build [--output bin]
go run ./cmd/tingo run [--addr :8080]
go run ./cmd/tingo test [go-test args...]
go run ./cmd/tingo clean
go run ./cmd/tingo version
```

`init` 生成的标准目录：

```
<root>/
  go.mod  main.go  .env.example
  app/
    app.go
    controller/
    model/
    middleware/
  config/
    app.toml
    database.toml
  route/
  public/static/
  runtime/
```

入口只需匿名导入 `app`；HTTP 与数据库均由配置自动装配。

## 扩展模块 contrib（独立子 module）

`contrib/` 是独立 Go module（`github.com/xmszy/tingo/contrib`），提供可选组件，不污染核心。
参考 gin-contrib 与 GoFrame contrib，按功能分包，每个包可单独引入。组件统一返回 `core.Handler`，
通过 `t.Use(...)` 或 `app.Use(...)` 注册到引擎/应用/路由。

| 包 | 功能 | 依赖 |
| --- | --- | --- |
| `contrib/cors` | 跨域资源共享中间件 | 标准库 |
| `contrib/secure` | 安全响应头（HSTS/X-Frame/XSS）中间件 | 标准库 |
| `contrib/recovery` | panic 恢复中间件 | 标准库 |
| `contrib/gzip` | 响应 gzip 压缩中间件 | 标准库 |
| `contrib/logger` | HTTP 访问日志中间件 | 标准库 |
| `contrib/static` | 静态文件服务中间件 | 标准库 |
| `contrib/auth` | HTTP Basic / Bearer 鉴权中间件 | 标准库 |
| `contrib/cache` | 响应缓存中间件（内存，TTL） | 标准库 |
| `contrib/upload` | 文件上传辅助（大小/扩展名校验、批量） | 标准库 |
| `contrib/rate` | 令牌桶限流中间件（应用级/QPS+突发） | 标准库 |
| `contrib/ratelimit` | 限流中间件：内存令牌桶 + **Redis 分布式滑动窗口** | go-redis/v9 |
| `contrib/jwt` | JWT 认证中间件（签发/校验 Bearer） | golang-jwt/jwt/v5 |
| `contrib/sessions` | 会话中间件（Cookie 存储 + Redis 存储） | go-redis/v9 |
| `contrib/captcha` | 图形验证码（生成/校验/输出图片） | dchest/captcha |
| `contrib/csrf` | **表单 CSRF 防护**（Cookie Double-Submit，一次性令牌） | 标准库 |
| `contrib/validate` | **Tingo 风格校验器**（规则串 + 结构体 tag 双支持） | 标准库 |
| `contrib/metric` | 轻量 Prometheus 指标中间件（QPS/延迟/错误率，/metrics） | 标准库 |
| `contrib/trace` | 轻量链路追踪中间件（TraceID 注入 + 慢请求日志） | 标准库 |
| `contrib/debug` | 开发调试错误页（Whoops 风格 HTML 异常页，含堆栈） | 标准库 |
| `contrib/registry` | 服务注册发现抽象（file 后端，接口可接 etcd/nacos） | 标准库 |

使用示例：

```go
import (
	"github.com/xmszy/tingo/frame"
	"github.com/xmszy/tingo/contrib/cors"
	"github.com/xmszy/tingo/contrib/jwt"
	"github.com/xmszy/tingo/contrib/recovery"
)

func (*App) Routes(r t.Router) {
	// 应用级全局中间件：在应用 Routes 根部用 r.Use（对默认应用而言即全局生效）。
	r.Use(recovery.Middleware(), cors.Middleware(cors.Default()))

	// 路由级中间件（仅该组生效）。
	r.Group("/api", func(g t.Router) {
		g.Use(jwt.Middleware(jwt.Config{Secret: "s3cret"}))
		g.GET("/me", meHandler)
	})
}
```

> 独立模块引入：`go get github.com/xmszy/tingo/contrib/cors`（单个包）。

## 开发

```bash
go test ./...                                  # 全部测试
go test ./contrib/...                          # contrib 模块测试
```
