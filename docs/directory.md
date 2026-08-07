# 目录结构

## 单应用模式

Tingo 默认使用单应用模式，安装后的目录结构：

~~~
├─app                    应用目录
│  ├─app.go              应用入口
│  ├─kernel.go           内核（中间件/Provider/事件注册）
│  ├─exception.go        异常处理器
│  ├─common.go           公共函数
│  ├─provider.go         服务提供者
│  │
│  ├─controller          控制器目录
│  │  ├─index.go         默认控制器
│  │  └─base.go          基类控制器
│  │
│  ├─model               模型目录
│  ├─service             业务服务目录
│  ├─middleware           中间件目录
│  ├─validate            验证器目录
│  └─view                视图目录
│
├─config                 全局配置目录
│  ├─app.toml            应用配置
│  ├─database.toml       数据库配置
│  ├─log.toml            日志配置
│  ├─route.toml          路由配置
│  ├─session.toml        Session 配置
│  ├─view.toml           视图配置
│  ├─cache.toml          缓存配置
│  ├─console.toml        控制台配置
│  ├─cookie.toml         Cookie 配置
│  ├─filesystem.toml     文件系统配置
│  ├─lang.toml           多语言配置
│  ├─middleware.toml     中间件配置
│  └─trace.toml          Trace 配置
│
├─route                  路由定义目录
│  └─app.go
│
├─public/static/         WEB 公开目录
├─runtime                运行时目录
├─main.go                入口文件
├─go.mod
├─go.sum
└─.env.example           环境变量示例
~~~

> 只有 `public/static/` 目录允许 HTTP 对外访问。

## 多应用模式

通过 `tingo init <name> --multi-app` 创建多应用骨架（`index` + `admin` 双应用），
随后用 `tingo make app <name>` 新增子应用。目录结构如下：

~~~
├─app                    应用目录
│  ├─applications.go     聚合导入，触发各子应用 init() 注册
│  │
│  ├─index                index 应用（默认应用）
│  │  ├─app.go            t.App("index", App{})
│  │  └─controller/index.go
│  │
│  └─admin                admin 应用
│     ├─app.go
│     └─controller/index.go
│
├─config
│  └─app.toml            [app] 段统一调度（default_app / app_map / domain_bind / deny_app）
├─public/static/
├─runtime/
└─main.go
~~~

各子应用通过 `app/applications.go` 的匿名导入触发 `init()` 注册，
应用调度由 `config/app.toml` 的 `[app]` 段统一决定。详见 [多应用](./multi_app.md)。

## 目录说明

| 目录/文件 | 说明 |
|---|---|
| `app/` | 应用核心目录，存放控制器、模型、服务等 |
| `app/app.go` | 应用入口，实现 `t.Application` 接口 |
| `app/kernel.go` | 应用内核，注册中间件、Provider、事件 |
| `app/exception.go` | 全局异常处理器 |
| `app/common.go` | 公共函数（类似 TP 的 common.php） |
| `app/provider.go` | 服务提供者，绑定接口实现 |
| `app/controller/` | 控制器目录，负责请求处理 |
| `app/model/` | 模型目录，负责数据访问 |
| `app/service/` | 业务逻辑服务目录 |
| `app/middleware/` | 自定义中间件目录 |
| `config/` | 全局配置文件目录（TOML 格式） |
| `route/` | 路由定义文件目录 |
| `public/static/` | 可公开访问的静态资源 |
| `runtime/` | 运行时生成的文件（日志、缓存等，可写） |
| `main.go` | 入口文件，调用 `tingo.Run()` |
