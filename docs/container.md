# 容器和依赖注入

Tingo 的容器基于泛型 + 反射，提供类型安全的依赖注入，避免字符串键导致的类型断言。

## 基本用法

### 绑定

~~~go
// 绑定接口到实现
t.Bind[LoggerInterface](&FileLogger{})

// 绑定实例
t.Instance[LoggerInterface](&FileLogger{Path: "/var/log/app.log"})

// 绑定单例工厂
t.Singleton[LoggerInterface](func() LoggerInterface {
    return &FileLogger{Path: "/var/log/app.log"}
})
~~~

### 获取

~~~go
// 泛型取值，无需类型断言
logger := t.Make[LoggerInterface]()
logger.Info("hello")
~~~

### 判断

~~~go
if t.Bound[LoggerInterface]() {
    logger := t.Make[LoggerInterface]()
}
~~~

## 类型安全的上下文键

使用 `t.Key[T]` 实现类型安全的上下文值存储：

~~~go
// 定义上下文键
var CurrentUser = t.Key[*model.User]("auth.user")

// 设置
CurrentUser.Set(c, user)

// 获取（无需类型断言）
user, ok := CurrentUser.Get(c)
~~~

对比传统方式：

~~~go
// ❌ 传统方式：需要类型断言
c.Set("user", user)
u, ok := c.Get("user")
if ok {
    user := u.(*model.User)  // 类型断言，运行时可能 panic
}

// ✅ Tingo 泛型键：编译期类型安全
user, ok := CurrentUser.Get(c)
~~~

## 框架自动注入

框架在启动时自动注册核心组件到容器：

- `*tcfg.Config` → 全局配置
- `*tdb.DB` → 数据库连接
- `*tlog.Logger` → 日志
- `*tcache.Cache` → 缓存

业务代码可通过门面方法直接获取，无需手动绑定：

~~~go
db := t.Database()
logger := t.Log()
cache := t.Cache()
~~~
