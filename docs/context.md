# Context 工具

`tctx` 提供 `context.Context` 的泛型读写工具，零外部依赖。

## 类型安全的上下文值

相比于标准库 `context.WithValue` 需要手动类型断言，`tctx` 提供泛型版本的读写：

~~~go
import "github.com/xmszy/tingo/frame"

// 定义带类型的 key
var userKey = t.Key[User]("user")

ctx := context.Background()

// 写入
ctx = t.CtxWithValue(ctx, userKey, currentUser)
~~~

### 读取

~~~go
// Value —— 返回 (T, bool)
user, ok := t.CtxValue(ctx, userKey)
if !ok {
    // 上下文中没有当前用户
}

// MustValue —— 返回 T，不存在返回零值
user := t.CtxMustValue(ctx, userKey)
if user.ID == 0 {
    // 零值表示不存在
}
~~~

### 批量设置

~~~go
ctx = t.CtxWithValues(ctx, map[any]any{
    userKey:    currentUser,
    tenantKey:  "tenant-001",
    traceIDKey: "abc123",
})
~~~

> `WithValues` 内部循环调用 `context.WithValue`，每对值产生一个新 Context 层。

## 与 Core Ctx 的关系

`tctx` 操作的是 **标准 `context.Context`**，与框架的 `*core.Ctx`（HTTP 请求上下文）是不同层次的概念：

- `*core.Ctx` — 携带 HTTP 请求信息（路径、方法、Headers 等），通过 `c.Context` 访问底层 `context.Context`
- `context.Context` — Go 标准库的上下文传递机制，用于取消控制、超时、跨链路传值

如果需要跨中间件/服务层传递值，使用 `tctx` 操作 `c.Context`：

~~~go
func MyMiddleware(c *core.Ctx) {
    var user User
    // ... 获取用户 ...
    c.Context = t.CtxWithValue(c.Context, userKey, user)
    c.Next()
}
~~~

## 完整函数表

| 函数 | 签名 | 说明 |
|---|---|---|
| `t.CtxWithValue` | `(ctx, key, val)` | 设置带类型 key 的值 |
| `t.CtxValue` | `(ctx, key) (T, bool)` | 泛型读取 |
| `t.CtxMustValue` | `(ctx, key) T` | 读取或零值 |
| `t.CtxWithValues` | `(ctx, map)` | 批量设置 |
