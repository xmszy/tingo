# 事件

Tingo 提供泛型事件总线（`tevent`），支持类型安全的事件订阅与分发。

## 基本用法

### 定义事件

~~~go
// 定义事件载荷类型
type UserCreated struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

// 创建事件
var EventUserCreated = t.EventNew[UserCreated]("user.created")
~~~

### 创建事件总线

~~~go
// 同步总线
bus := t.BusNew(false)

// 异步总线
bus := t.BusNew(true)
~~~

### 订阅事件

~~~go
t.BusSubscribe(bus, EventUserCreated, func(ctx context.Context, payload UserCreated) error {
    // 处理用户创建事件
    log.Printf("用户 %s (ID:%d) 已创建", payload.Name, payload.ID)
    return nil
})
~~~

### 分发事件

~~~go
t.BusDispatch(bus, ctx, EventUserCreated, UserCreated{ID: 1, Name: "张三"})
~~~

## 一次性监听

~~~go
t.BusOnce(bus, EventUserCreated, func(ctx context.Context, payload UserCreated) error {
    // 仅触发一次
    return nil
})
~~~

## 在应用级别管理事件

在 `kernel.go` 中集中管理事件订阅：

~~~go
func (k *Kernel) Subscribe() map[t.Event]t.EventHandler {
    return map[t.Event]t.EventHandler{
        EventUserCreated: handleUserCreated,
        EventOrderPaid:   handleOrderPaid,
        EventUserLogin:   handleUserLogin,
    }
}
~~~

## 任务队列（tqueue）

基于事件总线的事件队列，支持失败重试和死信：

~~~go
// 创建队列（最多重试 3 次）
q := t.QueueNew[EmailJob](false, 3)

// 订阅处理函数
q.Subscribe(func(ctx context.Context, msg t.QueueMessage[EmailJob]) error {
    return sendEmail(msg.Payload)
})

// 发布任务
q.Publish(ctx, EmailJob{To: "user@example.com", Subject: "欢迎注册"})
~~~

失败的任务会自动重试，超过重试次数后进入死信处理。
