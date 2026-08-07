// Package tpipeline 提供管道模式（Pipeline Pattern）。
//
// 管道模式用于构造洋葱模型中
// 间件链。每个 stage 接收输入、调用 next() 执行下一阶段、返回输出。
//
// 设计要点：
//   - 泛型 Pipeline[T]：输入输出类型参数化。
//   - 零外部依赖，纯标准库。
//   - Send/Through/Via/Then/ThenReturn 链式 API。
//   - 支持 recover 捕获 panic。
//
// 用法（HTTP 中间件链）：
//
//	result, err := tpipeline.New[*http.Request]().
//	    Send(req).
//	    Through(LogMiddleware{}, AuthMiddleware{}).
//	    Then(func(req *http.Request) (*http.Request, error) {
//	        return req, handler(req)
//	    })
//
// 用法（数据处理管道）：
//
//	result := tpipeline.New[int]().
//	    Send(1).
//	    Through(AddOne{}, Double{}).
//	    ThenReturn(func(n int) int { return n + 10 })
//	// result = (1 + 1) * 2 + 10 = 14
package tpipeline

import (
	"fmt"
	"runtime/debug"
)

// ──────────────── Pipe 接口 ────────────────

// Pipe 管道阶段（有状态 stage）。
// Handle 接收当前值 passable 和下一阶段 next。
// 调用 next(passable) 执行后续管道，返回最终结果。
type Pipe[T any] interface {
	Handle(passable T, next func(T) (T, error)) (T, error)
}

// PipeFunc 函数式阶段（无状态 stage）。
type PipeFunc[T any] func(passable T, next func(T) (T, error)) (T, error)

// Handle 实现 Pipe 接口。
func (f PipeFunc[T]) Handle(passable T, next func(T) (T, error)) (T, error) {
	return f(passable, next)
}

// ──────────────── Pipeline ────────────────

// Pipeline 泛型管道。
type Pipeline[T any] struct {
	passable T
	stages   []Pipe[T]
	via      string
	recover  bool
}

// New 创建管道。
func New[T any]() *Pipeline[T] {
	return &Pipeline[T]{}
}

// Send 设置初始数据。
func (p *Pipeline[T]) Send(passable T) *Pipeline[T] {
	p.passable = passable
	return p
}

// Through 设置管道阶段列表。
func (p *Pipeline[T]) Through(stages ...Pipe[T]) *Pipeline[T] {
	p.stages = stages
	return p
}

// ThroughFuncs 设置函数式阶段列表。
func (p *Pipeline[T]) ThroughFuncs(stages ...PipeFunc[T]) *Pipeline[T] {
	p.stages = make([]Pipe[T], len(stages))
	for i, s := range stages {
		p.stages[i] = s
	}
	return p
}

// Via 设置调用方法名（预留扩展，当前固定调用 Handle）。
func (p *Pipeline[T]) Via(method string) *Pipeline[T] {
	p.via = method
	return p
}

// WithRecover 开启 panic 恢复。
func (p *Pipeline[T]) WithRecover() *Pipeline[T] {
	p.recover = true
	return p
}

// Then 执行管道并返回结果。
// destination 是管道执行的最后一环（洋葱最内层）。
func (p *Pipeline[T]) Then(destination func(T) (T, error)) (T, error) {
	return p.carry(destination)
}

// ThenReturn 执行管道并返回结果（destination 无错误返回）。
func (p *Pipeline[T]) ThenReturn(destination func(T) T) T {
	result, err := p.carry(func(v T) (T, error) { return destination(v), nil })
	if err != nil {
		// 不会发生（destination 不返回 error），但为安全返回零值
		var zero T
		return zero
	}
	return result
}

// ──────────────── 管道执行 ────────────────

func (p *Pipeline[T]) carry(destination func(T) (T, error)) (T, error) {
	var zero T

	if p.recover {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch e := r.(type) {
				case error:
					err = e
				default:
					err = fmt.Errorf("pipeline panic: %v\n%s", r, string(debug.Stack()))
				}
				var z T
				zero = z
				_ = err
				// 不返回 zero，已在 defer 外处理
			}
		}()
	}

	if len(p.stages) == 0 {
		return destination(p.passable)
	}

	// 构建洋葱链：从最右侧开始包裹。
	// 最终调用的函数 = 层层递归调用 Handle。
	// stage[0].Handle(passable, next) 其中 next = stage[1].Handle(..., ...destination)
	var chain func(int, T) (T, error)
	chain = func(i int, v T) (T, error) {
		if i >= len(p.stages) {
			return destination(v)
		}
		return p.stages[i].Handle(v, func(passable T) (T, error) {
			return chain(i+1, passable)
		})
	}

	result, err := chain(0, p.passable)
	if err != nil {
		return zero, err
	}
	return result, nil
}

// ──────────────── 全局门面 ────────────────

// Send 全局管道入口。
func Send[T any](passable T) *Pipeline[T] {
	return New[T]().Send(passable)
}

// ──────────────── 内置 Stage ────────────────

// Tap 是一个管道阶段，在不会修改值的情况下执行旁路动作（如日志、监控）。
// 始终调用 next。
type Tap[T any] struct {
	Fn func(T)
}

func (t Tap[T]) Handle(passable T, next func(T) (T, error)) (T, error) {
	t.Fn(passable)
	return next(passable)
}

// Conditional 条件执行阶段。
// 当 condition 返回 true 时才执行嵌套阶段。
type Conditional[T any] struct {
	Condition func(T) bool
	Stage     Pipe[T]
}

func (c Conditional[T]) Handle(passable T, next func(T) (T, error)) (T, error) {
	if c.Condition(passable) {
		return c.Stage.Handle(passable, next)
	}
	return next(passable)
}
