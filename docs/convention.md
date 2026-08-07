# 开发规范

## 命名规范

Tingo 遵循 Go 社区命名规范：

| 类别 | 规范 | 示例 |
|---|---|---|
| 包名 | 小写、简洁、无下划线 | `controller`、`model`、`service` |
| 文件名 | 小写、下划线分隔 | `user_info.go`、`order_log.go` |
| 结构体 | 大驼峰导出、小驼峰私有 | `UserController`、`BaseController` |
| 方法 | 大驼峰导出、小驼峰私有 | `Index()`、`Read()`、`getUser()` |
| 常量 | 大驼峰导出 | `StatusActive`、`MaxPageSize` |

## 约定优于配置

Tingo 沿袭 Tingo 的约定优于配置理念：

### 目录约定

| 目录 | 用途 | 是否必须 |
|---|---|---|
| `app/controller/` | 控制器 | 必须 |
| `app/model/` | 模型 | 推荐 |
| `app/service/` | 业务逻辑 | 推荐 |
| `app/middleware/` | 中间件 | 可选 |
| `app/validate/` | 验证器 | 可选 |
| `app/view/` | 模板 | 可选 |
| `config/` | 全局配置 | 必须 |
| `route/` | 路由定义 | 推荐 |

### 控制器方法约定

| HTTP 方法 | 方法名 | 路由 |
|---|---|---|
| GET | `Index` | `/controller` |
| GET | `Create` | `/controller/create` |
| POST | `Save` | `/controller` |
| GET | `Read` | `/controller/:id` |
| GET | `Edit` | `/controller/:id/edit` |
| PUT | `Update` | `/controller/:id` |
| DELETE | `Delete` | `/controller/:id` |

### 表名约定

模型默认映射到蛇形复数表名：`UserInfo` → `user_info`。

## 控制器签名规范

控制器方法签名推荐：

~~~go
// 无入参无出参（最简单）
func (c *Index) Hello(ctx *core.Ctx)

// 有入参有出参（推荐，自动绑定+校验+响应封装）
func (c *Index) Index(ctx *core.Ctx, req *ListReq) (*ListRes, error)

// 仅出参 error
func (c *Index) Delete(ctx *core.Ctx) error
~~~

详情见 [控制器](./controller.md)。
