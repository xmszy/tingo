# 更新日志（Changelog）

本项目所有重要变更均记录于此文件。

格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## v0.0.1（2026-08-07）

> 首个正式版本。基于 Go 1.18+ 泛型构建，零业务外部依赖（仅标准库 + 少量官方/轻量第三方），提供从 HTTP 核心到 ORM、任务队列、代码生成的一体化 Web 开发能力。

### 核心框架（`core`）
- `Ctx`：请求上下文，统一封装请求/响应、参数绑定、JSON/HTML 输出、控制器标识
- `Router` 接口：`GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS/Any`、`Group`、`Controller`、`Resource` 及中间件 `Use`
- `Application` 多应用挂载：子域名/前缀/域名绑定、禁用开关
- `Container` 依赖注入容器：类型化 provider、父子容器继承
- 处理器反射适配：注册期分析 handler 签名，预编译调用计划（支持 `*Ctx`、`*gin.Context`、`context.Context`、`*Req` 结构体自动绑定）
- `M` 快捷 map 类型、`SetResponder` 自定义响应渲染

### Web 服务（`net`）
- `tnet`：HTTP 服务封装（优雅关闭、多地址监听）
- `tclient`：类型友好 HTTP 客户端，`New` + Option 链式配置（超时、重试、BaseURL、TLS、Header、中间件链）
- `twebsocket`：基于 gorilla/websocket 的连接升级与消息收发封装
- `topenapi`：OpenAPI 3 规范生成，泛型 Schema 推导、路由收集、自动 snake_case 字段
- `tlb`：负载均衡算法库（Random / RoundRobin / WeightedRoundRobin / LeastConnection / ConsistentHash）

### 数据库 ORM（`database/tdb`）
- **泛型 Model[T]**：`Where/Select/Order/Limit/Offset/Group/Join/With` 链式查询
- **读写分离**：主从配置，自动按操作路由
- **事务**：`Transaction(fn)` 自动提交/回滚
- **软删除**：`SoftDelete` / `SoftDeleteInt` 软删除模型，查询自动过滤、支持恢复（`WithTrashed`/`Restore`）
- **模型关联**：`HasOne` / `HasMany` / `BelongsTo` / `BelongsToMany` / `HasOneThrough` / `MorphOne` / `MorphMany` / `MorphTo`，支持 `With` / `Load` 预加载
- **数据迁移**：`Migrator` 支持 `Up` / `Down` / `Reset` / `Status`、版本追踪、up/down 双向SQL
- **数据填充**：`Seeder` 种子数据
- 连接池、预处理、`Raw` 原生 SQL、字段级 `tdb` 标签映射
- 驱动：`database/sql`（MySQL/PostgreSQL/SQLite 等）

### 任务队列（`os/tqueue`）
- 泛型 `Message[T]`：含 ID、Payload、Attempts、Headers、延迟投递
- `MemoryQueue`：基于 `tevent` 事件总线的内存驱动，失败重试（MaxRetry）+ 死信回调
- **`RedisQueue`（新增）**：基于 `os/tredis` 的 Redis 驱动，满足对接外部中间件需求
  - `LPUSH` + `BRPOP` 阻塞消费，`ZADD` 有序集合实现持久化延迟队列
  - `Start(workers)` 多协程消费、`Stop()` 优雅退出，重试/死信语义与内存驱动一致
  - 零额外依赖（复用框架自带 tredis RESP 客户端）

### 代码生成（`os/tcodegen`）
- 基于标准库 `go/ast` 解析结构体标签，生成 tdb Model 脚手架（NewXxxModel + 列名常量）
- 生成资源控制器（ResourceController）骨架
- **注解路由生成（新增）**：解析 `//tingo:route GET /path` 方法注释，生成 `Annotations()` 方法，对接 `tapp.RouteAnnotated` 接口
- 生成代码经 gofmt 格式化、可独立编译、可手写修改

### 基础组件库（`os`，零外部依赖）
- `tcontainer` 容器辅助 · `tconv/tconvert/ttype/tstructs` 类型转换与反射
- `tcookie` Cookie · `tconsole` 控制台输出（色彩/表格）· `tcrypto` 加解密与哈希
- `tencoding` 编解码（JSON/TOML/YAML/INI）· `tfile/tfilesystem` 文件与文件系统
- `tmutex/tpool/tproc/ttrace` 并发原语、协程池、进程管理、调用链追踪
- `tpage` 分页 · `tpipeline` 流水线编排 · `trand/tuuid` 随机与 UUID
- `tredis` 纯 Go RESP 协议 Redis 客户端（连接池、AUTH、管道、订阅，零依赖）
- `tregex/tres/tspath/tstr/ttime/ttimer/ttree/tutil/tvalid/tmode` 正则/资源/路径/字符串/时间/定时器/树/通用/校验/运行模式
- `tview` 模板引擎（布局继承、区块、共享变量、内置模板函数）
- `tevent` 事件总线 · `tqueue` 任务队列 · `tcodegen` 代码生成

### 应用层（`tapp`）
- 应用生命周期：`Base` 多应用基类、`Configure` 启动编排、优雅关闭
- **注解路由（新增）**：`AnnotationRoute` / `RegisterAnnotated` / `AutoRouteAnnotated`，支持 `//tingo:route` 风格声明与方法名反射注册
- 约定式自动路由：`RegisterController` / `AutoRoute`（驼峰转下划线）
- 全局异常处理：`Exception` / `Reporter` / `Recover`，`NoRoute` 兜底
- `Validator` 校验器接入、`AutoRoute` 自动挂载

### Contrib 扩展库（`contrib/`，独立 module）
- 中间件：**secure**（安全响应头）、**recovery**（panic 恢复）、**gzip**（响应压缩）、**logger**（访问日志）、**static**（静态文件）、**auth**（Basic/Bearer 鉴权）、**cache**（响应缓存）、**cors**（跨域）、**csrf**（Double-Submit）
- 业务支撑：**validate**（TP 风格校验）、**jwt**（JWT 认证）、**rate**（令牌桶限流）、**ratelimit**（内存 + Redis 分布式滑动窗口）、**upload**（文件上传）、**captcha**（图形验证码）、**sessions**（Cookie + Redis 存储）、**formtoken**（CSRF 令牌）、**image**（图像处理）、**smtp**（邮件）、**mongo**（MongoDB）、**nosql/redis**（Redis 封装）、**rpc**（RPC 调用）、**lang**（i18n）、**view**（视图）、**config**（配置中心）、**drivers**（多驱动适配）
- 可观测：**metric**（Prometheus `/metrics`）、**trace**（TraceID + 慢日志）、**debug**（Whoops 风格错误页）
- 服务治理：**registry**（服务注册/发现，file 后端 + 接口）

### 命令行（`cmd`）
- `tingo` 命令：`serve`（启动）、`migrate`（迁移）、`gen`（代码生成 model/controller/route）、`version`
- 跨平台构建脚本（Linux/macOS/Windows）

### 文档
- 文档覆盖快速开始、HTTP 核心、路由、中间件、请求/响应、验证、错误、生命周期、多应用、控制器、REST、模型、关联、软删除、迁移、查询、事务、队列、Redis、视图、会话、缓存、国际化、日志、配置、代码生成等

---
