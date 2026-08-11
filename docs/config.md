# 配置

## 配置文件

Tingo 使用 TOML 作为默认配置格式，支持 YAML、JSON、INI。默认读取 `.toml` 文件，
可通过环境变量 `CONFIG_EXT` 切换：

~~~dotenv
CONFIG_EXT=ini
~~~

### 配置作用域

配置文件按文件名自动映射为一组配置项：

- `config/app.toml` → 可通过 `t.Config().Bool("debug")` 读取
- `config/database.toml` → 可通过 `t.Config().String("database.default")` 读取
- `app/admin/config/database.toml` → admin 应用专属数据库配置

### 配置优先级

~~~
框架默认值 < config/*.toml < 应用 config/*.toml < 环境变量/占位符 < 显式 Go Option
~~~

## 应用配置 app.toml

~~~toml
# app.toml
debug = false        # 总调试开关（.env 中 APP_DEBUG=true 时自动为 true）：开启后挂载 ttrace 调试工具栏、打印 SQL 等
default_timezone = "Asia/Shanghai"
default_app = "app"
default_lang = "zh-cn"

[server]
# addr 可写为 ":8080" 或纯端口号 "8080"（框架自动补 ":"）
addr = ":8080"
read_timeout = "30s"
write_timeout = "30s"
max_header_bytes = 1048576
~~~

在代码中读取：

~~~go
debug := t.Config().Bool("debug", false)
addr := t.Config().String("server.addr", ":8080")

// 在 handler 中读取当前请求所属应用的配置
cfg := t.ConfigFrom(c)
pageSize := cfg.Int("app.page_size", 20)
~~~

## 数据库配置 database.toml

~~~toml
# database.toml
default = "mysql"

[connections.mysql]
type = "mysql"
hostname = "127.0.0.1"
database = "test"
username = "root"
password = ""
hostport = "3306"
charset = "utf8mb4"
prefix = ""
~~~

支持环境变量占位：

~~~toml
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
~~~

`${VAR:-default}` 语法：变量存在且非空则使用变量值，否则使用默认值。

## 路由配置 route.toml

~~~toml
# route.toml
url_suffix = false
url_method_suffix = false
url_route_rules = false

[slash]
remove = true
add = false

[method]
405 = true

[meta]
enabled = true
~~~

## 日志配置 log.toml

~~~toml
# log.toml
global_level = "info"
async = true

[access]
enabled = true
skip_paths = []                   # 跳过记录的路径列表，默认空即可
skip_not_found = true
~~~

`access.skip_paths`（默认 `[]`）：显式跳过记录的路径列表（如健康检查 `/health`）。由于 `skip_not_found` 已默认过滤所有 404/405，**浏览器/探针探测（如 `/detect/version`、Chrome DevTools 的 `/.well-known/appspecific/com.chrome.devtools.json`）作为 404 已被默认静默**，通常无需在此配置。

`access.skip_not_found`（默认 `true`）：未命中路由的 404/405 响应不写访问日志，避免浏览器/探针探测、端口扫描、DDoS 类噪声在日志中放大。需要审计所有请求（含 404）时设为 `false`。

## Session 配置 session.toml

~~~toml
# session.toml
cookie_name = "tingo_sid"
expire = "24h"
path = "/"
secure = false
http_only = true
~~~

## 视图配置 view.toml

~~~toml
# view.toml
root = "view"
ext = ".html"
~~~

## 环境变量

Tingo 在加载配置前会**自动**读取项目根目录下的 `.env`（及 `.env.local`，文件不存在时忽略）。因此 `config/*.toml` 中的 `${APP_DEBUG}` 等占位符可以直接展开，无需手动 `t.EnvLoad`。如需额外加载其他文件或覆盖已有变量，仍可显式调用 `t.EnvLoad(...)`。也可直接通过 `os.Setenv` / shell `export` 预先设置。

配置文件里的 `${VAR}` 占位符在解析时展开，支持 `${VAR:-default}` 形式（变量缺失时取默认值）。

~~~dotenv
APP_DEBUG=false       # 统一的总调试开关：true 时 debug=true 并挂载 ttrace 调试工具栏，默认 false
DB_TYPE=mysql
DB_HOST=127.0.0.1
DB_NAME=test
DB_USER=root
DB_PASS=password
DB_PORT=3306
SERVER_ADDR=8080      # 端口可只写数字，框架自动补 ":"；或写 ":8080"
CONFIG_EXT=toml
~~~

在代码中使用 `tenv` 门面方法读取：

~~~go
debug := t.Env("APP_DEBUG", "false")   // 读环境变量名（大写），不是配置键
value, ok := t.EnvLookup("DB_HOST")
_ = t.EnvHas("CUSTOM_FLAG")
_ = t.EnvExpand("${DB_HOST}:3306")
~~~
