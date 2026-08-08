package thttp

import (
	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

// ──────────────── HTTP 请求生命周期钩子 ────────────────
//
// 钩子按注册顺序执行，若任意钩子返回 error 则后续均不执行。

// Hook 是生命周期钩子函数类型。
type Hook func(ctx *core.Ctx) error

// HookType 定义钩子触发时机。
type HookType int

const (
	// HookBeforeServe 请求进入前执行（路由匹配后、handler 前）。
	HookBeforeServe HookType = iota + 1

	// HookAfterServe 请求处理完成后执行（handler 返回后、响应写出前）。
	HookAfterServe
)

// hookEntry 内部分组存储。
type hookEntry struct {
	hooks []Hook
}

// registerHook 注册一个生命周期钩子。
func (e *Engine) registerHook(ht HookType, hook Hook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.hooks == nil {
		e.hooks = make(map[HookType]*hookEntry)
	}
	entry, ok := e.hooks[ht]
	if !ok {
		entry = &hookEntry{}
		e.hooks[ht] = entry
	}
	entry.hooks = append(entry.hooks, hook)
}

// HookBeforeServe 注册一个前置钩子（路由匹配后、handler 执行前）。
// 典型场景：统一鉴权、请求日志、RequestID 注入、跨域预检。
func (e *Engine) HookBeforeServe(hook Hook) *Engine {
	e.registerHook(HookBeforeServe, hook)
	return e
}

// HookAfterServe 注册一个后置钩子（handler 返回后、响应写出前）。
// 典型场景：响应头追加、统一内容转换、缓存写入、性能监控。
func (e *Engine) HookAfterServe(hook Hook) *Engine {
	e.registerHook(HookAfterServe, hook)
	return e
}

// executeHooks 执行指定类型的所有钩子。任意钩子返回非 nil 则停止。
func (e *Engine) executeHooks(ht HookType, ctx *core.Ctx) {
	e.mu.Lock()
	entry := e.hooks[ht]
	e.mu.Unlock()
	if entry == nil {
		return
	}
	for _, h := range entry.hooks {
		if err := h(ctx); err != nil {
			// 钩子返回 error → 写入响应并停止后续执行
			if !ctx.Writer.Written() {
				ctx.JSON(map[string]string{"error": err.Error()})
			}
			ctx.Abort()
			return
		}
	}
}

// ──────────────── 自动安装 ────────────────
// Hook 通过注册全局中间件的方式注入到 gin 管道中。
// 中间件在路由匹配之前执行，将 before 钩子作为中间件一部分运行。

// installHookMiddleware 安装钩子中间件。在 Boot() 时调用一次。
//
// 未注册任何钩子时直接跳过，避免给每个请求叠加一次「加锁 + map 查找」
// 的中间件开销。
func (e *Engine) installHookMiddleware() {
	if len(e.hooks) == 0 {
		return
	}
	e.gin.Use(func(ginCtx *gin.Context) {
		ctx := core.FromGin(ginCtx)

		// BeforeServe: 在 handler 执行前调用
		e.executeHooks(HookBeforeServe, ctx)

		// 若已被 Abort，跳过后续 handler
		if ctx.IsAborted() {
			return
		}

		// 继续执行后续 handler（包括业务 handler）
		ginCtx.Next()

		// AfterServe: 在 handler 执行后调用
		e.executeHooks(HookAfterServe, ctx)
	})
}
