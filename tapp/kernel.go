package tapp

import (
	"sort"
	"sync"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/ttrace"
)

/* ------------------------------------------------------------------ */
/* 应用装配内核                                                          */
/* ------------------------------------------------------------------ */

// Kernel 汇总一个项目的应用层装配。
//
// 登记的都是具体的 Go 值与函数，所有装配在编译期确定、启动期完成，运行时零查找。
type Kernel struct {
	mu sync.Mutex

	// exception 是全局异常处理器。
	exception *ExceptionHandle

	// middlewares 是全局中间件。
	middlewares []core.Handler

	// providers 是容器绑定回调。
	providers []func(*core.Container) error

	// subscribers 是事件订阅回调。
	subscribers []func() error

	// services 是系统服务。
	services []KernelService

	booted bool
}

// NewKernel 创建一个应用装配内核。
func NewKernel() *Kernel {
	return &Kernel{exception: NewExceptionHandle()}
}

/* ------------------------------------------------------------------ */
/* 调试工具栏自动注册                                                   */
/* ------------------------------------------------------------------ */

// TraceConfigProvider 由上层（frame/t）注入，从全局配置读取工具栏配置与启用开关。
// 返回 (配置, 是否启用)。在 Kernel.Boot 时调用；未注入则不启用工具栏。
var TraceConfigProvider func() (ttrace.Config, bool)

/* ------------------------------------------------------------------ */
/* 配置登记                                                              */
/* ------------------------------------------------------------------ */

// Use 追加全局中间件。
func (k *Kernel) Use(mws ...core.Handler) *Kernel {
	k.mu.Lock()
	k.middlewares = append(k.middlewares, mws...)
	k.mu.Unlock()
	return k
}

// Provide 追加一个容器绑定回调。
func (k *Kernel) Provide(fn func(*core.Container) error) *Kernel {
	if fn == nil {
		return k
	}
	k.mu.Lock()
	k.providers = append(k.providers, fn)
	k.mu.Unlock()
	return k
}

// Subscribe 追加一个事件订阅回调。
func (k *Kernel) Subscribe(fn func() error) *Kernel {
	if fn == nil {
		return k
	}
	k.mu.Lock()
	k.subscribers = append(k.subscribers, fn)
	k.mu.Unlock()
	return k
}

// Register 追加一个系统服务。
func (k *Kernel) Register(svcs ...KernelService) *Kernel {
	k.mu.Lock()
	k.services = append(k.services, svcs...)
	k.mu.Unlock()
	return k
}

// SetException 替换全局异常处理器。
func (k *Kernel) SetException(h *ExceptionHandle) *Kernel {
	if h == nil {
		return k
	}
	k.mu.Lock()
	k.exception = h
	k.mu.Unlock()
	return k
}

// Exception 返回全局异常处理器。
func (k *Kernel) Exception() *ExceptionHandle {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.exception
}

// Middlewares 返回已登记的全局中间件副本。
func (k *Kernel) Middlewares() []core.Handler {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]core.Handler(nil), k.middlewares...)
}

/* ------------------------------------------------------------------ */
/* 启动流程                                                              */
/* ------------------------------------------------------------------ */

// Boot 执行完整的应用装配流程：
//
//  1. 注册异常处理器（使其接管全部 handler 错误）；
//  2. 执行 provider 绑定容器服务；
//  3. 按优先级执行系统服务的 Register；
//  4. 执行事件订阅；
//  5. 按优先级执行系统服务的 Boot。
//
// 任一步骤出错立即返回，保证启动失败快速可见。
func (k *Kernel) Boot(container *core.Container) error {
	k.mu.Lock()
	if k.booted {
		k.mu.Unlock()
		return nil
	}
	k.booted = true
	exception := k.exception
	providers := append([]func(*core.Container) error(nil), k.providers...)
	subscribers := append([]func() error(nil), k.subscribers...)
	services := append([]KernelService(nil), k.services...)
	k.mu.Unlock()

	if exception != nil {
		exception.Register()
	}

	if container == nil {
		container = core.Default()
	}

	for _, provide := range providers {
		if err := provide(container); err != nil {
			return err
		}
	}

	// 容器服务绑定完成后，对全局登记的控制器执行字段依赖注入。
	// 放在 services 注册之前，使服务可注入已经装配好的依赖。
	for _, ctrl := range RegisteredControllers() {
		if err := Inject(container, ctrl); err != nil {
			return err
		}
	}

	// 按 Priority 升序执行，未实现 Prioritized 的服务视为 0。
	sortServices(services)

	for _, svc := range services {
		if err := svc.Register(container); err != nil {
			return err
		}
	}

	for _, subscribe := range subscribers {
		if err := subscribe(); err != nil {
			return err
		}
	}

	for _, svc := range services {
		if booter, ok := svc.(BootableKernelService); ok {
			if err := booter.Boot(container); err != nil {
				return err
			}
		}
	}

	// 调试工具栏：config/app.toml 的 debug=true 时自动挂到全局中间件链。
	// 必须在系统服务 Boot 之后注册——配置（tcfg.Service）在此阶段加载，
	// 工具栏才能读到 debug 与 trace.* 配置。
	if TraceConfigProvider != nil {
		if cfg, ok := TraceConfigProvider(); ok {
			k.Use(ttrace.NewWithConfig(cfg).Handler())
		}
	}
	return nil
}

// Booted 返回内核是否已完成装配。
func (k *Kernel) Booted() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.booted
}

/* ------------------------------------------------------------------ */
/* 系统服务                                                             */
/* ------------------------------------------------------------------ */

// KernelService 是系统服务契约。
// 与框架级 core.Service（生命周期服务）概念不同，KernelService 仅需
// 在装配期绑定容器，由 Kernel.Boot() 调用。
type KernelService interface {
	// Register 在装配期绑定服务到容器。
	Register(c *core.Container) error
}

// BootableKernelService 是可选接口，服务可实现它在全部 Register 完成后执行初始化。
type BootableKernelService interface {
	// Boot 在所有服务注册完毕后调用。
	Boot(c *core.Container) error
}

// Prioritized 是可选接口，服务可声明自己的装配优先级，值小者先执行。
type Prioritized interface {
	// Priority 返回装配优先级。
	Priority() int
}

// sortServices 按优先级稳定排序。
func sortServices(services []KernelService) {
	sort.SliceStable(services, func(i, j int) bool {
		return priorityOf(services[i]) < priorityOf(services[j])
	})
}

func priorityOf(s KernelService) int {
	if p, ok := s.(Prioritized); ok {
		return p.Priority()
	}
	return 0
}

// KernelServiceFunc 让普通函数可作为 KernelService 使用。
type KernelServiceFunc func(c *core.Container) error

// Register 实现 KernelService 接口。
func (f KernelServiceFunc) Register(c *core.Container) error { return f(c) }
