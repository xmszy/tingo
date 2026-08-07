package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/errors"
)

/* ------------------------------------------------------------------ */
/* 多签名适配                                                           */
/* ------------------------------------------------------------------ */

// Adapt 将多种 handler 签名统一适配为 Handler。
//
// 支持的签名（按性能从高到低排列，前三种为零反射快路径）：
//
//	func(*Ctx)                            原生，零开销
//	func(*gin.Context)                   gin 原生，零开销
//	func(*Ctx) error                     带错误，零开销
//	func(*Ctx) (T, error)                泛型返回，需用 W 包装
//	func(ctx context.Context, req *Req) (*Res, error)   反射适配
//
// 反射仅发生在注册期，运行时通过预编译的闭包执行，
// 每请求成本为一次 sync.Pool 取用 + 反射 Call。
// 对性能敏感的路由请使用前三种签名或 codegen 生成的适配器。
func Adapt(h any) Handler {
	switch f := h.(type) {
	case Handler:
		return f
	case func(*Ctx):
		return f
	case gin.HandlerFunc:
		return HandlerOf(f)
	case func(*gin.Context):
		return HandlerOf(f)
	case HandlerE:
		return wrapE(f)
	case func(*Ctx) error:
		return wrapE(f)
	}
	return adaptReflect(h)
}

// wrapE 包装带错误返回的 handler。
func wrapE(f func(*Ctx) error) Handler {
	return func(c *Ctx) {
		if err := f(c); err != nil {
			responder.Fail(c, err)
		}
	}
}

/* ------------------------------------------------------------------ */
/* 泛型包装：零反射，编译期确定类型                                       */
/* ------------------------------------------------------------------ */

// W 将业务方法包装为 Handler，全程零反射。
//
//	r.GET("/users", core.W(ctrl.List))
//
// 其中 ctrl.List 的签名为：
//
//	func(c *Ctx, req *ListReq) (*ListRes, error)
//
// Req 结构体通过 sync.Pool 复用，绑定后清零，实现每请求零分配。
func W[Req any, Res any](f func(*Ctx, *Req) (Res, error)) Handler {
	pool := &sync.Pool{New: func() any { return new(Req) }}
	return func(c *Ctx) {
		req := pool.Get().(*Req)
		var zero Req
		*req = zero // 清零，避免上次请求的数据残留（zero 为栈上零值，无堆分配）
		defer pool.Put(req)

		if err := c.BindAllAndValidate(req); err != nil {
			responder.Fail(c, errors.ErrValidation.Wrap(err))
			return
		}
		res, err := f(c, req)
		if err != nil {
			responder.Fail(c, err)
			return
		}
		responder.Reply(c, res)
	}
}

// WN 包装无入参的业务方法。
//
//	func(c *Ctx) (Res, error)
func WN[Res any](f func(*Ctx) (Res, error)) Handler {
	return func(c *Ctx) {
		res, err := f(c)
		if err != nil {
			responder.Fail(c, err)
			return
		}
		responder.Reply(c, res)
	}
}

/* ------------------------------------------------------------------ */
/* 反射适配：兜底路径，仅注册期做反射分析                                  */
/* ------------------------------------------------------------------ */

var (
	typeCtx    = reflect.TypeFor[*Ctx]()
	typeGinCtx = reflect.TypeFor[*gin.Context]()
	typeStdCtx = reflect.TypeFor[context.Context]()
	typeError  = reflect.TypeFor[error]()
)

// adaptReflect 在注册期分析 handler 签名，返回预编译的闭包。
func adaptReflect(h any) Handler {
	v := reflect.ValueOf(h)
	t := v.Type()
	if t.Kind() != reflect.Func {
		panic(errors.ErrInternal.WithMessagef("tingo: handler must be a function, got %s", t.Kind()))
	}

	plan, err := buildPlan(t)
	if err != nil {
		panic(err)
	}

	return func(c *Ctx) { plan.invoke(v, c) }
}

// callPlan 是注册期分析出的调用方案，运行时直接按方案执行。
type callPlan struct {
	// argKinds 描述每个入参如何构造。
	argKinds []argKind
	// reqType 是需要绑定的请求结构体类型（指针指向的元素类型）。
	reqType reflect.Type
	// reqPool 复用请求结构体。
	reqPool *sync.Pool
	// numOut 返回值个数。
	numOut int
	// dataOutIdx 数据返回值下标，-1 表示无。
	dataOutIdx int
	// errOutIdx 错误返回值下标，-1 表示无。
	errOutIdx int
	// argsPool 复用参数切片。
	argsPool *sync.Pool
}

// argKind 描述一个入参的构造方式。
type argKind uint8

const (
	argCtx    argKind = iota // *core.Ctx
	argGinCtx                // *gin.Context
	argStdCtx                // context.Context
	argReq                   // *Req，需要绑定
)

// buildPlan 分析函数签名，构造调用方案。
func buildPlan(t reflect.Type) (*callPlan, error) {
	p := &callPlan{
		argKinds:   make([]argKind, 0, t.NumIn()),
		dataOutIdx: -1,
		errOutIdx:  -1,
	}

	for i := 0; i < t.NumIn(); i++ {
		in := t.In(i)
		switch {
		case in == typeCtx:
			p.argKinds = append(p.argKinds, argCtx)
		case in == typeGinCtx:
			p.argKinds = append(p.argKinds, argGinCtx)
		case in == typeStdCtx:
			p.argKinds = append(p.argKinds, argStdCtx)
		case in.Kind() == reflect.Pointer && in.Elem().Kind() == reflect.Struct:
			if p.reqType != nil {
				return nil, errors.ErrInternal.WithMessage("tingo: handler accepts more than one request struct")
			}
			p.reqType = in.Elem()
			rt := p.reqType
			p.reqPool = &sync.Pool{New: func() any { return reflect.New(rt).Interface() }}
			p.argKinds = append(p.argKinds, argReq)
		default:
			return nil, errors.ErrInternal.WithMessagef(
				"tingo: unsupported handler parameter #%d of type %s", i, in)
		}
	}

	p.numOut = t.NumOut()
	switch t.NumOut() {
	case 0:
	case 1:
		if t.Out(0) == typeError {
			p.errOutIdx = 0
		} else {
			p.dataOutIdx = 0
		}
	case 2:
		if t.Out(1) != typeError {
			return nil, errors.ErrInternal.WithMessage(
				"tingo: the second return value of a handler must be error")
		}
		p.dataOutIdx, p.errOutIdx = 0, 1
	default:
		return nil, errors.ErrInternal.WithMessagef(
			"tingo: handler returns too many values (%d)", t.NumOut())
	}

	n := len(p.argKinds)
	p.argsPool = &sync.Pool{New: func() any {
		s := make([]reflect.Value, n)
		return &s
	}}
	return p, nil
}

// invoke 按方案执行调用。
func (p *callPlan) invoke(fn reflect.Value, c *Ctx) {
	argsPtr := p.argsPool.Get().(*[]reflect.Value)
	args := *argsPtr
	defer p.argsPool.Put(argsPtr)

	var reqPtr any
	for i, k := range p.argKinds {
		switch k {
		case argCtx:
			args[i] = reflect.ValueOf(c)
		case argGinCtx:
			args[i] = reflect.ValueOf(c.G())
		case argStdCtx:
			args[i] = reflect.ValueOf(c.Request.Context())
		case argReq:
			reqPtr = p.reqPool.Get()
			rv := reflect.ValueOf(reqPtr)
			rv.Elem().SetZero()
			if err := c.BindAllAndValidate(reqPtr); err != nil {
				p.reqPool.Put(reqPtr)
				responder.Fail(c, errors.ErrValidation.Wrap(err))
				return
			}
			args[i] = rv
		}
	}
	if reqPtr != nil {
		defer p.reqPool.Put(reqPtr)
	}

	out := fn.Call(args)

	if p.errOutIdx >= 0 {
		if e := out[p.errOutIdx]; !e.IsNil() {
			responder.Fail(c, e.Interface().(error))
			return
		}
	}
	if p.dataOutIdx >= 0 {
		responder.Reply(c, out[p.dataOutIdx].Interface())
		return
	}
	if p.numOut > 0 {
		responder.Reply(c, nil)
	}
}
