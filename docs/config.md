# 配置

## 配置文件

Tingo 使用 TOML 作为默认配置格式，支持 YAML、JSON、INI。默认读取 `.toml` 文件，
可通过环境变量 `CONFIG_EXT` 切换：

~~~dotenv
CONFIG_EXT=ini
~~~

### 配置作用域

配置文件按文件名自动映射为一组配置项：

- `config/app.toml` → 可通过 `t.Config().String("app.debug")` 读取
- `config/database.toml` → 可通过 `t.Config().String("database.default")` 读取
- `app/admin/config/database.toml` → admin 应用专属数据库配置

### 配置优先级

~~~
框架默认值 < config/*.toml < 应用 config/*.toml < 环境变量/占位符 < 显式 Go Option
~~~

## 应用配置 app.toml

~~~toml
# app.toml
debug = true
default_timezone = "Asia/Shanghai"
default_app = "app"
default_lang = "zh-cn"

[server]
addr = ":8080"
read_timeout = "30s"
write_timeout = "30s"
max_header_bytes = 1048576
~~~

在代码中读取：

~~~go
debug := t.Config().Bool("app.debug", false)
addr := t.Config().String("app.server.addr", ":8080")

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
skip_paths = ["/detect/version"]
~~~

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

Tingo 默认加载根目录的 `.env` 文件，支持 Tingo 风格的变量命名：

~~~dotenv
APP_DEBUG=true
DB_TYPE=mysql
DB_HOST=127.0.0.1
DB_NAME=test
DB_USER=root
DB_PASS=password
DB_PORT=3306
CONFIG_EXT=toml
~~~

在代码中使用 `tenv` 门面方法读取：

~~~go
debug := t.Env("app.debug", false)
value, ok := t.EnvLookup("db.host")
_ = t.EnvHas("custom.flag")
_ = t.EnvExpand("${DB_HOST}:3306")
~~~
