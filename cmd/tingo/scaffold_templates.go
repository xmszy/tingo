package main

// 本文件集中存放 tingo init 的脚手架模板。

const tplGomod = `module {{.Module}}
`

const tplMain = `// Command {{.Name}} 是应用入口。
package main

import (
	"log"

	"github.com/xmszy/tingo"
	_ "{{.Module}}/app"
)

func main() {
	if err := tingo.Run(); err != nil {
		log.Fatal(err)
	}
}
`

const tplAppConfig = `# 应用配置。
name = "{{.Name}}"
debug = "${APP_DEBUG:-true}"
default_app = "app"
default_timezone = "Asia/Shanghai"

# HTTP 服务。
[server]
addr = "${SERVER_ADDR:-:8080}"
mode = "${SERVER_MODE:-debug}"     # debug / release / test
print_routes = true
read_timeout = "60s"
read_header_timeout = "20s"
write_timeout = "60s"
idle_timeout = "120s"
shutdown_timeout = "10s"
max_header_bytes = 1048576
max_multipart_memory = 33554432
trusted_proxies = []

# HTTPS（生产环境启用）。
# [server.https]
# cert_file = "config/cert.pem"
# key_file = "config/key.pem"
`

const tplApplicationConfig = `# 默认应用配置。这里的值覆盖 config/app.toml 中的同名项。
prefix = "/"
default = true
`

const tplDatabaseConfig = `# 数据库配置。通过 default 切换默认连接。
default = "${DB_DRIVER:-mysql}"

# ── MySQL / MariaDB ──────────────────────────────────────
[connections.mysql]
type = "${DB_TYPE:-mysql}"
hostname = "${DB_HOST:-127.0.0.1}"
database = "${DB_NAME:-}"
username = "${DB_USER:-root}"
password = "${DB_PASS:-}"
hostport = "${DB_PORT:-3306}"
charset = "${DB_CHARSET:-utf8mb4}"
prefix = "${DB_PREFIX:-}"
max_open = 20
max_idle = 10

# ── PostgreSQL ───────────────────────────────────────────
# [connections.pgsql]
# type = "postgres"
# hostname = "${PGSQL_HOST:-127.0.0.1}"
# database = "${PGSQL_NAME:-}"
# username = "${PGSQL_USER:-postgres}"
# password = "${PGSQL_PASS:-}"
# hostport = "${PGSQL_PORT:-5432}"
# schema = "public"
# prefix = ""
# max_open = 20
# max_idle = 10

# ── SQLite ───────────────────────────────────────────────
# [connections.sqlite]
# type = "sqlite"
# database = "runtime/database.sqlite"
# prefix = ""

# ── SQL Server ───────────────────────────────────────────
# [connections.sqlserver]
# type = "sqlserver"
# hostname = "${MSSQL_HOST:-127.0.0.1}"
# database = "${MSSQL_NAME:-}"
# username = "${MSSQL_USER:-sa}"
# password = "${MSSQL_PASS:-}"
# hostport = "${MSSQL_PORT:-1433}"
# prefix = ""
# max_open = 20
# max_idle = 10
`

const tplRouteConfig = `# 路由配置。
redirect_trailing_slash = true    # 自动补全或去除结尾斜杠
redirect_fixed_path = false       # 路径大小写修正
handle_method_not_allowed = true  # 方法不匹配时返回 405
bind_route_meta = true            # 绑定应用/控制器/方法元信息
`

const tplLogConfig = `# 日志配置。
level = "${LOG_LEVEL:-info}"      # debug / info / warn / error / fatal
async = false                     # 开启异步写入可减少 IO 阻塞
async_buffer = 1024               # 异步模式下的 channel 容量
flags = ["time", "level"]         # 可选: time, file, func, level
time_format = "2006-01-02T15:04:05.000Z07:00"
prefix = ""

# 文件输出（不配置则写入 stderr）。
# file = "runtime/log/tingo.log"
# file_max_size = 100             # MB，单文件上限，超出则切分
# file_max_days = 30              # 日志文件保留天数

# 访问日志。
[access]
enabled = true
skip_paths = []                   # 跳过记录的路径列表
`

const tplSessionConfig = `# 会话配置。
name = "PHPSESSID"
expire = "24m"                    # 会话有效期
cookie_path = "/"
secure = false                    # 仅 HTTPS 传输
http_only = false                 # 禁止客户端脚本访问

# 存储驱动（默认内存，按需切换）。
# [store]
# driver = "memory"               # memory / database
# connection = "mysql"            # 数据库驱动时所用的连接名
# table = "sessions"              # 数据库驱动时的表名
# gc_interval = "5m"              # 过期会话回收间隔
`

const tplViewConfig = `# 视图配置。
root = "app/view"                 # 模板根目录
extension = ".html"               # 模板文件扩展名
left_delim = "{{.LeftDelim}}"                 # 模板左边界符
right_delim = "{{.RightDelim}}"                # 模板右边界符
`

const tplCookieConfig = `# Cookie 设置。
expire = 0                          # 保存时间（秒），0 表示会话级
path = "/"                          # 保存路径
domain = ""                         # 有效域名
secure = false                      # 仅 HTTPS 传输
httponly = false                    # 禁止客户端脚本访问
samesite = ""                       # SameSite：strict / lax / none
`

const tplLangConfig = `# 多语言设置。
default_lang = "${DEFAULT_LANG:-zh-cn}"  # 默认语言
auto_detect_browser = true               # 自动侦测浏览器语言
allow_lang_list = []                     # 允许的语言列表
detect_var = "lang"                      # 自动侦测变量名
use_cookie = true                        # 使用 Cookie 记录语言
cookie_var = "tingo_lang"                # Cookie 变量名
header_var = "tingo-lang"               # Header 变量名
extend_list = []                         # 扩展语言包
allow_group = false                      # 是否支持语言分组
`

const tplMiddlewareConfig = `# 中间件配置。

# 别名映射（用于路由中按名称引用中间件）。
[alias]
# auth = "Auth"

# 优先级列表，数组中的中间件按顺序优先执行。
priority = []
`

const tplCacheConfig = `# 缓存配置。
default = "memory"

# 内存缓存（默认，进程内 LRU）。
[connections.memory]
driver = "memory"
shards = 256                      # 分片数，越大竞争越少
max_entries = 100000              # 全局容量，0 表示无上限
sweep_interval = "5m"             # 过期条目清扫间隔

# Redis 缓存（需要引入 contrib/drivers/redis）。
# [connections.redis]
# driver = "redis"
# addr = "${REDIS_ADDR:-127.0.0.1:6379}"
# password = "${REDIS_PASS:-}"
# db = ${REDIS_DB:-0}
# prefix = "cache:"               # 键前缀
`

const tplEnvExample = `CONFIG_EXT = toml
APP_DEBUG = true

# 数据库
DB_TYPE = mysql
DB_HOST = 127.0.0.1
DB_NAME = test
DB_USER = username
DB_PASS = password
DB_PORT = 3306
DB_CHARSET = utf8

# 缓存
REDIS_ADDR = 127.0.0.1:6379
REDIS_PASS =
REDIS_DB = 0

# 多语言
DEFAULT_LANG = zh-cn

# HTTP 服务
SERVER_ADDR = :8080
SERVER_MODE = debug
LOG_LEVEL = info
`

const tplApplication = `// Package app 定义默认应用。
package app

import (
	t "github.com/xmszy/tingo/frame"
	"{{.Module}}/app/controller"
	"{{.Module}}/app/route"
)

// 触发控制器包的 init()，完成自动路由自登记。
var _ = controller.Index{}

// init 注册应用。
func init() { t.App("app", &App{}) }

// App 是默认应用。内嵌 t.BaseApp 即可获得配置与中间件的默认实现，
// 只需覆写自己关心的部分。
type App struct {
	t.BaseApp
}

// Boot 在路由注册前执行应用装配：
// 异常处理器、全局中间件、容器绑定、事件订阅与系统服务。
func (*App) Boot() error {
	Kernel.SetException(ExceptionHandle())
	return Kernel.Boot(nil)
}

// Middlewares 返回应用级中间件，作用于本应用的全部路由。
func (*App) Middlewares() []t.Handler { return Kernel.Middlewares() }

// Routes 注册本应用的路由。
func (*App) Routes(r t.Router) { route.Register(r) }
`

const tplRoute = `// Package route 集中声明应用路由。
package route

import t "github.com/xmszy/tingo/frame"

// Register 注册本应用的全部路由。
func Register(r t.Router) {
	// 自动路由：挂载所有在 init() 中登记的控制器。
	// 控制器的 init() 通过 t.RegisterController 完成自登记，其包需被 import
	// 才会执行（见 app/app.go 中的 controller 包导入）。
	t.AutoRoute(r)

	// 需要自定义路径/资源路由/分组的，可在自动路由之后继续追加：

	// 资源路由：一次注册 7 个 RESTful 动作。
	// r.Resource("/users", &controller.User{})

	// 路由分组。
	// r.Group("/api", func(r t.Router) {
	//     r.GET("/profile", (&controller.Index{}).Json)
	// }, middleware.Auth())
}
`

const tplController = `// Package controller 是应用控制器层。
package controller

import t "github.com/xmszy/tingo/frame"

func init() {
	// 自动路由：把自己登记到全局表，框架启动时由 t.AutoRoute 挂载。
	t.RegisterController("/", &Index{})
}

// Index 是首页控制器，内嵌 t.Controller 获得基类能力
// （Success/Error/Result/Validate/Bind 等）。
type Index struct {
	t.Controller
}

// Initialize 在每个动作执行前调用。
// 不需要时可以直接删除该方法。
func (*Index) Initialize(c *t.Ctx) error { return nil }

// Index 对应 GET /
func (*Index) Index(c *t.Ctx) {
	c.String("hello,Tingo!")
}

// Think 对应 GET /think
func (*Index) Think(c *t.Ctx) {
	c.String("hello, %s!", c.Param("name"))
}

// Hello 对应 GET /hello/:name
func (*Index) Hello(c *t.Ctx) {
	c.String("hello,%s", c.Param("name"))
}

// Json 演示统一 JSON 响应。
func (i *Index) Json(c *t.Ctx) error {
	return i.Success(c, t.Map{"framework": "tingo"})
}
`

/* ------------------------------------------------------------------ */
/* app/ 下的约定文件                                                     */
/* ------------------------------------------------------------------ */

const tplBaseController = `// Package controller 的控制器基类。
//
// 项目中所有控制器都内嵌 Base，便于统一追加鉴权、公共数据等能力。
package controller

import t "github.com/xmszy/tingo/frame"

// Base 是本项目的控制器基类。
//
// 它内嵌框架的 t.Controller，因而自带：
//   - Success / Error / Result / Redirect  统一响应
//   - Validate / Bind / BindValidate       参数绑定与校验
//
// 在此追加的方法，所有业务控制器都能直接使用。
type Base struct {
	t.Controller
}

// User 演示如何在基类中提供公共能力：
// 从上下文取出鉴权中间件写入的当前用户标识。
func (Base) User(c *t.Ctx) string {
	return t.Req(c).Header("X-User-Id")
}
`

const tplExceptionHandle = `// Package app 的全局异常处理器。
package app

import (
	t "github.com/xmszy/tingo/frame"
)

// ExceptionHandle 构造全局异常处理器。
//
// 它决定了两件事：
//   - 哪些异常需要写日志（IgnoreReport 清单之外的都会记录）；
//   - 异常以什么形式呈现（Ajax/JSON 请求输出 JSON，否则输出 HTML 错误页）。
func ExceptionHandle() *t.ExceptionHandle {
	h := t.NewExceptionHandle()

	// debug 开启时输出错误详情与堆栈，生产环境请通过配置关闭。
	h.Debug = t.Config().Bool("debug", false)

	// 异常写入框架日志。
	h.Reporter = t.NewLogReporter(nil)

	// 纯 API 项目可打开此开关，让 HTML 请求也返回 JSON。
	// h.RenderJSON = true

	return h
}
`

const tplKernel = `// Package app 是应用装配入口。
//
// 这里登记的都是具体的 Go 值与函数，
// 编译期即可确定，启动期一次性装配完成，运行时零查找开销。
package app

import (
	t "github.com/xmszy/tingo/frame"
)

// Kernel 是本项目的应用装配内核。
var Kernel = t.NewKernel().
	// ---- 全局中间件 ----
	// 异常捕获中间件由框架自动置于最外层，此处只需登记业务中间件。
	Use(
	// t.Log(),
	// t.CORS(),
	).

	// ---- 容器绑定 ----
	Provide(func(c *t.Container) error {
		// t.Bind(func(*t.Container) (*service.User, error) {
		//     return service.NewUser(), nil
		// })
		return nil
	}).

	// ---- 事件订阅 ----
	Subscribe(func() error {
		// t.On("user.registered", func(u User) { ... })
		return nil
	}).

	// ---- 系统服务 ----
	Register(
		&AppService{},
	)
`

const tplAppService = `package app

import (
	t "github.com/xmszy/tingo/frame"
)

// AppService 是应用级系统服务。
//
// Register 在装配期执行，用于向容器登记服务；
// Boot 在所有服务注册完毕后执行，用于依赖其他服务的初始化。
type AppService struct{}

// Register 注册服务到容器。
func (*AppService) Register(c *t.Container) error {
	return nil
}

// Boot 在全部服务注册完成后执行初始化。
func (*AppService) Boot(c *t.Container) error {
	return nil
}

// Priority 决定装配顺序，值小者先执行。
func (*AppService) Priority() int { return 0 }
`

const tplCommon = `package app

// 本文件存放项目级助手函数。
//
// 框架已内置常用助手，可直接使用：
//
//	t.Abort(404, "不存在")   中断请求并抛出异常
//	t.Snake / t.Camel        命名风格转换
//	t.Validate(data, rules)  数据校验
//
// 在此追加项目自己的公共函数即可。
`

const tplMiddlewareAuth = `// Package middleware 存放项目中间件。
package middleware

import t "github.com/xmszy/tingo/frame"

// Auth 是一个鉴权中间件示例。
//
// 中间件在注册期即展开进路由树，运行时没有任何动态分发开销。
func Auth() t.Handler {
	return func(c *t.Ctx) {
		if t.Req(c).Header("X-User-Id") == "" {
			t.Abort(401, "请先登录")
		}
		c.Next()
	}
}
`

const tplPublicIndexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Name}}</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
         display: flex; justify-content: center; align-items: center;
         min-height: 100vh; background: #f5f5f5; color: #333; }
  .card { text-align: center; background: #fff; border-radius: 12px;
          padding: 48px 64px; box-shadow: 0 2px 12px rgba(0,0,0,.08); }
  h1 { font-size: 2rem; margin-bottom: 8px; color: #1677ff; }
  p  { font-size: .95rem; color: #666; margin-bottom: 24px; }
  .ver { display: inline-block; background: #e6f4ff; color: #1677ff;
         padding: 2px 10px; border-radius: 10px; font-size: .8rem; }
</style>
</head>
<body>
<div class="card">
  <h1>{{.Name}}</h1>
  <p>Tingo · Golang Web Framework</p>
  <span class="ver">v0.1.0</span>
</div>
</body>
</html>
`

const tplRobotsTxt = `User-agent: *
Disallow:
`

const tplHTAccess = `# Apache URL 重写规则。
# 将请求转发给 Go 应用（默认监听 :8080）。
#
# 若使用 Nginx，等效配置如下：
#
#   location / {
#       try_files $uri $uri/ @backend;
#   }
#   location @backend {
#       proxy_pass http://127.0.0.1:8080;
#       proxy_set_header Host $host;
#       proxy_set_header X-Real-IP $remote_addr;
#       proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
#       proxy_set_header X-Forwarded-Proto $scheme;
#   }
#
# 若使用 Caddy，等效配置如下：
#
#   example.com {
#       reverse_proxy localhost:8080
#   }

<IfModule mod_rewrite.c>
  Options +FollowSymlinks -Multiviews
  RewriteEngine On

  RewriteCond %{REQUEST_FILENAME} !-d
  RewriteCond %{REQUEST_FILENAME} !-f
  RewriteRule ^(.*)$ index.php/$1 [QSA,PT,L]
</IfModule>
`

// ── 控制台指令配置 ──
const tplConsoleConfig = `# 控制台指令定义。
# 在此注册项目自定义指令，每个指令需实现 tconsole.Command 接口。

[commands]
# "app\\command\\Hello" = "hello"  # 类名 → 指令名称
# "app\\command\\Build" = "build"  # 类名 → 指令名称
`

// ── 文件系统配置 ──
const tplFilesystemConfig = `# 文件系统配置。

# 默认磁盘。
default = "local"

# 磁盘列表。
[disks.local]
type = "local"
root = "runtime/storage"

[disks.public]
type = "local"
root = "public/storage"
url = "/storage"
visibility = "public"

# 更多磁盘类型（按需启用）：
# [disks.s3]
# type = "s3"
# key = "${S3_KEY:-}"
# secret = "${S3_SECRET:-}"
# region = "${S3_REGION:-}"
# bucket = "${S3_BUCKET:-}"
# url = ""
# endpoint = ""
`

// ── Trace 调试配置 ──
const tplTraceConfig = `# Trace 调试设置
# 仅在 config/app.toml 的 debug=true（或 .env 的 APP_DEBUG=true）时生效，
# 由框架自动挂载调试工具栏（无需在 middleware 中手动 Use）。

# 内置 Html（页面注入）与 Console（命令行输出）两种方式。
type = "Html"

# 读取的日志通道名（Console 模式相关）。
channel = "trace"

# 工具栏面板可见性（固定分区：基本/文件/流程/错误/SQL/调试）。
[panels]
base = true        # 基本（请求/内存/时间戳/会话）
file = true        # 文件（参与请求的源文件）
info = true        # 流程（info 级日志时间线）
error = true       # 错误/警告（panic 与 LogError）
sql = true         # SQL 查询（需启用 tdb 日志）
log = true         # 调试（业务 ttrace.Trace() 记录）
`

// ── 新增：app/.htaccess，对齐 TP app/.htaccess ──
const tplAppHtaccess = `deny from all
`

// ── 新增：README.md 项目说明 ──
const tplReadme = `# {{.Name}}

基于 [Tingo](https://github.com/xmszy/tingo) 构建的 Web 应用。

## 快速开始

	cd {{.Name}}
	go mod tidy
	tingo run

访问 http://localhost:8080

## 目录结构

| 目录 | 说明 |
|------|------|
| app/ | 应用代码（控制器/模型/服务/中间件/验证器） |
| config/ | 配置文件（TOML 格式） |
| public/ | 公共资源入口（index.html/static/） |
| route/ | 顶层路由定义 |
| view/ | 视图模板 |
| extend/ | 扩展类库 |
| runtime/ | 运行时文件（日志/缓存/存储） |

## 常用命令

	tingo run               启动开发服务器
	tingo run --watch       启动并监听文件变更
	tingo build             编译生产版本
	tingo gen model         从数据库生成模型
	tingo make controller   生成控制器

## 许可证

Apache-2.0
`

// ── 新增：LICENSE 文件 ──
const tplLicense = `Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

Copyright (c) 2025-present, Tingo Contributors.
`



const tplGitignore = `# 构建产物
/bin/
*.exe
*.test

# 本地环境
.env

# 运行时
/runtime/*
!/runtime/.gitkeep

# IDE
/.idea
/.vscode
/.settings
/.buildpath
/.project

# 系统文件
.DS_Store
Thumbs.db
`
