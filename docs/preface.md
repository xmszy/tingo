# 序言

Tingo 是一个 Tingo 风格的 Go Web 框架，旨在让 Go 开发者以 Tingo 的开发范式
快速构建 Web 应用。

## Tingo 是什么

Tingo 融合了三个领域的最佳实践：

- **开发范式** 对齐 Tingo：单应用默认、多应用显式、配置驱动、约定优于配置
- **工程能力** 借鉴 GoFrame：组件化、门面包（`t`）、代码生成（`tingo` CLI）
- **性能内核** 复用 gin：radix 路由树、零成本上下文转换、allocs/op 与 gin 完全对齐

> Tingo 不是 Tingo 的跨语言移植，而是用 Go 的惯用方式践行 Tingo 的开发理念。

## 设计哲学

### 以开发体验为优先

Tingo 追求高程度封装、优雅高效的写法。Tingo 中的 `$request->param()`、`$this->success()`、
路由自动注册等范式，在 Tingo 中都有对应的 Go 惯用实现。

### 性能绝不妥协

性能是硬性约束——**allocs/op 不得劣于 gin**。框架核心通过类型定义、函数值重解释等
编译期优化实现零成本抽象，路由在注册期全展开进 gin radix 树，运行时无动态分发。

### 模块化但不过度分层

明确拒绝 GoFrame 的 api/logic/service 分层，采用 Tingo 的 controller/model/service
简洁结构。组件通过独立的 `tingo-contrib` 模块（仓库 `github.com/xmszy/tingo-contrib`）提供，核心无外部依赖。

## 适用的场景

- 从 PHP/Tingo 转向 Go 的团队：开发范式一致，学习成本最低
- 追求高性能的 API 服务：gin 级性能 + Tingo 级开发体验
- 需要 CLI 工具支撑的项目：`tingo init`、`tingo gen`、`tingo make` 等完整工具链

## 对比 Tingo

| Tingo | Tingo |
|---|---|
| `$request->param('name')` | `req.Param("name")` |
| `$this->success($data)` | `ctl.Success(c, data)` |
| `validate(User::class)` | `ctl.BindValidate(c, &input, rules)` |
| `Db::table('user')->where('id', 1)->find()` | `model.NewUser().WhereEQ("id", 1).First()` |
| `throw new HttpException(403)` | `tapp.Abort(403, "禁止访问")` |
| `php think run` | `tingo run` |
| `php think make:controller` | `tingo make controller` |
| 门面 `Cache::set()` | `t.CacheSet(...)` |

## 获得帮助

- 源代码：[github.com/xmszy/tingo](https://github.com/xmszy/tingo)
- 本文档覆盖 Tingo 完整功能，按 Tingo 手册结构组织
