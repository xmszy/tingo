# 服务

Tingo 有两层 Service 概念，各司其职。

## KernelService — 应用层装配服务

对应 Tingo 的 `app/service.php`，由 `tapp.Kernel.Boot()` 驱动。
服务需实现 `tapp.KernelService` 接口：

~~~go
type KernelService interface {
    Register(c *core.Container) error  // 装配期绑定容器
}
~~~

### Register

`Register` 在服务注册阶段调用，适合向容器绑定接口实现：

~~~go
type AppService struct{}

func (s *AppService) Register(c *core.Container) error {
    core.BindValue(c, &userRepo{})
    return nil
}
~~~

### BootableKernelService

需要在所有 Register 完成后初始化的服务实现 `BootableKernelService`：

~~~go
type BootableKernelService interface {
    Boot(c *core.Container) error
}
~~~

### Prioritized

需要控制装配顺序的服务实现 `Prioritized`：

~~~go
type Prioritized interface {
    Priority() int  // 越小越先执行
}
~~~

## core.Service — 框架生命周期服务

框架基础设施（tcfg、tdb 等）实现 `core.Service` 接口，
由 `core.App` 的启动流程按拓扑排序调用：

~~~go
type Service interface {
    Name() string                // 注册名称
    DependsOn() []string         // 依赖的其他服务
    Register(app *App) error     // 注册期
    Boot(ctx context.Context, app *App) error  // 启动期
    Shutdown(ctx context.Context) error         // 关闭期
}
~~~

## 注册服务

在 `app/kernel.go` 中注册：

~~~go
k := tapp.NewKernel()
k.Register(
    &AppService{},
    &EventService{},
)
~~~

## 启动流程

~~~
core.App.Register()          → 注册框架生命周期服务
    ↓
tapp.Kernel.Boot(container)  → 装配应用层服务
    ↓
    KernelService → Register()
    ↓
    BootableKernelService → Boot()
    ↓
core.App.Boot(ctx)           → 启动框架服务
    ↓
core.App.Start()             → HTTP 引擎启动
    ↓
信号退出 → core.App.Shutdown(ctx)
~~~
