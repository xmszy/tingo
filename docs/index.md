# Tingo 开发手册

Tingo 是 Tingo 风格的 Go Web 框架，本手册对标 [Tingo 8.0 开发手册](https://doc.Tingo.cn/v8_0) 的组织方式。

## 序言

- [Tingo 介绍](./preface.md) —— 设计哲学、适用场景、对比 Tingo
- [安装](./install.md) —— 环境要求、项目初始化、启动服务
- [开发规范](./convention.md) —— 命名规范、约定优于配置
- [目录结构](./directory.md) —— 单应用/多应用目录详解
- [配置](./config.md) —— TOML/YAML/JSON 多格式、环境变量

## 架构

- [架构总览](./architecture.md) —— 分层架构、核心组件、零成本抽象
- [请求流程](./lifecycle.md) —— 完整请求生命周期、入口文件、异常处理
- [入口文件](./entry.md) —— main.go、run / build 命令
- [多应用](./multi_app.md) —— 多应用创建、配置作用域、kernel
- [URL 访问](./url_access.md) —— URL 解析规则、路由模式
- [容器和依赖注入](./container.md) —— 泛型容器、类型安全上下文键
- [服务](./service.md) —— Service / BootableService / Prioritized
- [门面（Facade）](./facade.md) —— `t` 包的完整功能表
- [中间件](./middleware.md) —— 定义、全局/分组/控制器级、contrib 中间件
- [事件](./event.md) —— 泛型事件总线、任务队列

## 路由

- [路由定义](./route.md) —— 自动/约定/资源/手动路由、分组
- [路由参数](./route_param.md) —— 路径参数、参数约束、数组参数
- [路由分组](./route_group.md) —— 分组中间件、嵌套分组、资源路由分组
- [资源路由](./route_resource.md) —— 7 个 RESTful 路由、嵌套资源
- [URL 生成](./url_gen.md) —— 命名路由、反向生成 URL

## 控制器

- [控制器定义](./controller.md) —— 方法签名、参数绑定、自动路由
- [基础控制器](./controller_base.md) —— Success/Error/Result/Redirect/Bind/Initialize/完整示例
- [控制器中间件](./controller_middleware.md) —— Middleware 声明、排除方法

## 请求

- [请求参数](./request.md) —— Param/Get/Post/JSON body/All/Only/Exclude/Input
- [请求信息](./request_info.md) —— 方法/URL/Header/路径参数/原始请求体

## 响应

- [响应](./response.md) —— JSON/HTML/重定向/文件下载/全局格式化/响应绑定

## 数据库

- [数据库](./database.md) —— 连接配置/查询构造器/条件/JOIN/分页/CRUD/事务/安全护栏/SQL 逃生舱
- [数据库事务](./database_transaction.md) —— Tx 回调模式/完整示例/原生 SQL 事务

## 模型

- [模型](./model.md) —— 泛型 Model/查询/新增/更新/删除/表名推断/列名标签/完整示例
- [读写分离](./read_write_split.md) —— ReadDSNs 配置/Master 强制主库/事务内强一致

## 视图

- [视图](./view.md) —— 模板语法/布局渲染/控制器快捷渲染

## 错误和日志

- [错误和日志](./error_log.md) —— 错误创建/链式派生/异常处理/日志

## 验证

- [验证](./validate.md) —— 规则定义/内置规则/自定义规则/控制器集成

## 杂项

- [缓存](./cache.md) —— 泛型缓存/过期/Take/原子操作/Keys
- [Session](./session.md) —— 内存存储/数据库存储/会话中间件
- [Cookie](./cookie.md) —— 设置/读取/删除
- [多语言](./lang.md) —— 语言文件/翻译/语言检测
- [文件上传](./upload.md) —— 单文件/多文件/验证

## 工具

- [字符串工具](./string.md) —— 脱敏/转义/相似度/版本比较/随机串
- [文件工具](./file.md) —— 主目录/大小格式化/内容替换/文件排序
- [结构体工具](./structs.md) —— Tag 解析/字段遍历/Tag 常量
- [构建信息](./build.md) —— 版本号/Git 提交/编译时间（ldflags）
- [Context 工具](./context.md) —— 泛型上下文读写
- [路径搜索](./path.md) —— 文件搜索/常用路径快捷方法

## 命令行

- [CLI 命令](./cli.md) —— version/init/run/build/gen/make/app
- [控制台开发](./console.md) —— Parser 参数解析/CommandNode 指令树/自定义命令

## 扩展库

- [contrib 组件](./contrib.md) —— CORS/JWT/限流/Prometheus/Trace/CSRF/调试页面

## 附录

- [助手函数](./helper.md) —— tapp/t 门面/日志快捷/环境变量/缓存/会话
