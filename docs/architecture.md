# 架构总览

## 分层架构

Tingo 采用分层架构，每层职责清晰：

~~~
请求 → 入口 → 中间件链 → 路由匹配 → 控制器 → 服务 → 模型 → 数据库
                      ↓
                   异常处理 → 响应
~~~

## 核心组件

| 组件 | 包 | 说明 |
|---|---|---|
| 引擎 | `net/thttp` | HTTP 服务器，基于 gin 内核 |
| 上下文 | `core` | 请求上下文 `Ctx`，类型定义为 `gin.Context` |
| 路由 | `net/thttp` | 资源路由、约定路由、自动路由 |
| 控制器 | `core/tapp` | TP 风格控制器基类 |
| 门面 | `frame/t` | 统一入口，一个字母 `t` |
| 错误 | `errors` | 结构化错误，携带状态码和业务码 |
| 配置 | `os/tcfg` | 多格式、强类型、Loader[T] |
| 日志 | `os/tlog` | 结构化、异步 |
| 缓存 | `os/tcache` | 泛型、过期、LRU |
| 数据库 | `database/tdb` | 泛型 ORM |

## 请求处理流程

1. **入口** `main.go` → `tingo.Run()`
2. **启动**：加载配置 → 初始化应用 → 注册路由 → 启动 HTTP 服务
3. **请求到达**：gin 路由树匹配 → 中间件链 → handler
4. **handler 执行**：`core.Adapt` 将多种函数签名适配为 gin 的 `HandlerFunc`
5. **异常处理**：handler 内 panic → `Recover` → `ExceptionHandle.Render`
6. **响应**：`responder.Reply` 生成 JSON/HTML 响应

## 零成本抽象

Tingo 的核心性能设计：

~~~go
// Ctx 是 gin.Context 的类型定义（非内嵌）
// 两者内存布局完全一致，指针转换是编译期 no-op
type Ctx gin.Context

// 零成本函数值转换（唯一的 unsafe 处）
func ginOf(c *Ctx) *gin.Context {
    return (*gin.Context)(unsafe.Pointer(c))
}
~~~

- `func(*Ctx)` 与 `func(*gin.Context)` 表示相同，无包装层
- 多应用/控制器在注册期全展开进 radix 树，运行时无动态分发
- 路由元信息走注册期只读查找表，不写 gin 的 Keys map

## 注册期 vs 运行期

| 操作 | 注册期 | 运行期 |
|---|---|---|
| 应用注册 | `t.App("name", &App{})` 在 `init()` 调用 | - |
| 路由注册 | `AutoRoute(r)` 遍历注册表 expand 进 gin | - |
| 控制器注册 | `RegisterController("/", &Ctrl{})` 在 `init()` | - |
| 参数绑定 | - | `core.Adapt` 反射分析 + 按预编译方案执行 |
| 配置读取 | - | `t.Config()` / `t.ConfigFrom(c)` |
