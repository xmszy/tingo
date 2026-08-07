# 请求流程

## 完整生命周期

从请求到响应，Tingo 的完整处理流程：

~~~
1. HTTP 请求到达
2. gin 路由树匹配
3. 全局中间件链（按注册顺序）
   ├─ recovery  → panic 恢复
   ├─ trace     → TraceID 注入
   ├─ logger    → 访问日志
   └─ ...
4. 应用中间件链
   ├─ auth      → 身份验证
   ├─ cors      → 跨域处理
   └─ ...
5. 路由级中间件
6. handler 适配（core.Adapt）
   ├─ 参数绑定（uri → query → body）
   ├─ 参数校验（binding tag）
   └─ 控制器方法调用
7. 控制器处理
   ├─ 服务层调用
   ├─ 模型查询
   └─ 返回结果
8. 响应封装
   ├─ 成功 → responder.Reply → JSON/HTML
   └─ 异常 → ExceptionHandle.Render → JSON/HTML
9. 响应发送给客户端
~~~

## 入口文件

`main.go` 是应用入口，极简：

~~~go
package main

import (
    "log"

    "github.com/xmszy/tingo"
    _ "myproject/app"  // 匿名导入触发 init() 注册
)

func main() {
    if err := tingo.Run(); err != nil {
        log.Fatal(err)
    }
}
~~~

`tingo.Run()` 内部执行：

~~~go
func Run(opts ...Option) error {
    // 1. 加载全局配置
    // 2. 初始化日志
    // 3. 调用应用 Boot()
    // 4. 构建路由树（所有应用的路由全展开）
    // 5. 启动 HTTP 服务
    // 6. 监听信号优雅退出
}
~~~

## 异常处理流程

Tingo 的全局异常处理通过 `ExceptionHandle` 实现：

~~~go
type ExceptionHandle struct {
    tapp.ExceptionHandle
}

func (e *ExceptionHandle) Render(c *core.Ctx, err error) {
    if te, ok := err.(*errors.Error); ok {
        c.JSON(te.Status, t.Map{
            "code":    te.Code,
            "message": te.Message,
        })
        return
    }
    c.JSON(500, t.Map{"code": "SYSTEM_ERROR", "message": "系统错误"})
}
~~~

通过 Abort 抛出业务异常：

~~~go
func (c *User) Save(ctx *core.Ctx) {
    if name == "" {
        tapp.Abort(400, "用户名不能为空")
    }
    // ...
}
~~~

`Abort` 实际是通过 panic 机制抛出 `*errors.Error`，由 `Recover` 中间件
捕获后交给 `ExceptionHandle.Render` 输出。
