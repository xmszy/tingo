# 错误和日志

## 错误处理

Tingo 的错误基于 `*errors.Error`，携带状态码、业务码和元数据。

### 创建错误

~~~go
import "github.com/xmszy/tingo/errors"

// 新建错误
err := errors.New(400, "REQUEST_INVALID", "请求参数无效")
err := errors.Newf(400, "USER_NOT_FOUND", "用户 %d 不存在", id)

// 从普通 error 包装
berr := errors.From(err)
berr := errors.Wrap(err, 500, "INTERNAL_ERROR", "内部错误")

// 预定义包级错误
var (
    ErrNotFound  = errors.New(404, "NOT_FOUND", "资源不存在")
    ErrForbidden = errors.New(403, "FORBIDDEN", "无权访问")
    ErrValidate  = errors.New(422, "VALIDATE_FAIL", "数据校验失败")
)
~~~

### 链式派生

错误方法是返回副本的（不修改原错误），适合预定义包级错误的安全派生：

~~~go
// 修改消息（返回副本）
err := ErrNotFound.WithMessage("用户不存在")
err := ErrNotFound.Messagef("用户 %d 不存在", id)

// 附加元数据
err := ErrNotFound.WithMeta(t.Map{"user_id": id})

// 包装底层错误
err := ErrForbidden.Wrap(dbErr)

// 修改状态码
err := ErrForbidden.WithStatus(401)
~~~

### 读取错误信息

~~~go
err := errors.New(422, "VALIDATE_FAIL", "校验失败")

err.Status()   // 422 (HTTP 状态码)
err.Code()     // "VALIDATE_FAIL" (业务码)
err.Message()  // "校验失败"
err.Meta()     // map[string]any
err.Error()    // 完整错误字符串
err.Unwrap()   // 底层错误（如果有）
~~~

## 异常处理

### 应用异常处理

`app/exception.go` 是全局异常处理入口：

~~~go
type ExceptionHandle struct {
    tapp.ExceptionHandle
}

// IgnoreReport 决定是否记录日志
func (e *ExceptionHandle) IgnoreReport(err error) bool {
    // 401/403/404 不记录日志
    if te, ok := err.(*errors.Error); ok {
        return te.Status == 401 || te.Status == 403 || te.Status == 404
    }
    return false
}

// Report 自定义日志记录
func (e *ExceptionHandle) Report(err error) {
    t.Log().Errorw("exception", "err", err)
}

// Render 生成响应
func (e *ExceptionHandle) Render(c *core.Ctx, err error) {
    if te, ok := err.(*errors.Error); ok {
        c.JSON(te.Status, t.Map{
            "code":    te.Code,
            "message": te.Message,
            "data":    te.Meta,
        })
        return
    }
    c.JSON(500, t.Map{"code": "SYSTEM_ERROR", "message": "系统错误"})
}
~~~

### Abort —— 抛出异常

`tapp.Abort` 通过 panic 机制抛出错误，由全局 `Recover` 中间件捕获后交给 `Render`：

~~~go
func (u *User) Save(c *t.Ctx) {
    if name == "" {
        tapp.Abort(400, "用户名不能为空")
    }
    // ...
}
~~~

Abort 的变体：

~~~go
tapp.Abort(403, "无权访问")
tapp.AbortIf(name == "", 400, "用户名不能为空")
tapp.AbortUnless(user != nil, 404, "用户不存在")
~~~

## 日志

Tingo 的日志组件 `tlog` 是无外部依赖的结构化日志：

### 基本用法

~~~go
import "github.com/xmszy/tingo/frame"

// 包级便捷函数
t.LogInfow("user created", t.LogF("id", 1), t.LogF("name", "张三"))
t.LogDebugw("debug message")
t.LogWarnw("warning message")
t.LogErrorw("error occurred", "err", err)
t.LogFatalw("fatal error")

// 获取 Logger 实例
logger := t.Log()
logger.Infow("request", "path", c.Path(), "method", c.Method())
~~~

### 请求上下文日志

~~~go
func handler(c *t.Ctx) {
    // 自动注入 TraceID 和应用名
    logger := t.LogFrom(c)
    logger.Infow("processing", "user_id", 1)
}
~~~

### 自定义 Logger

~~~go
logger := t.LogNew(t.LogConfig{
    Level:  "debug",
    Output: "/var/log/app.log",
    Async:  true,
})
logger.Infow("custom logger")
~~~

### 异步日志

配置 `async = true` 启用异步写入（推荐生产环境使用）：

~~~toml
# log.toml
async = true
buffer_size = 4096
~~~

异步模式下日志通过缓冲 channel 写入，不阻塞请求处理。
