// Package tapp 提供应用层约定。
//
// 这里的一切都是编译期确定的：没有反射调用、没有字符串类名解析，
// 因此不会给请求热路径带来任何额外开销。
package tapp

import (
	"github.com/xmszy/tingo/core"
)

/* ------------------------------------------------------------------ */
/* 控制器基类                                                            */
/* ------------------------------------------------------------------ */

// Initializer 是控制器的初始化钩子接口。
//
// 控制器实现该接口后，每次请求在进入具体动作前会先调用 Initialize。
// 返回 error 将中断动作执行并交由异常处理器渲染。
type Initializer interface {
	// Initialize 在控制器动作执行前调用。
	Initialize(c *core.Ctx) error
}

// MiddlewareDeclarer 是控制器中间件声明接口。
//
// 控制器实现该接口即可声明作用于自身全部动作的中间件，
// 这些中间件在注册期展开进路由树，运行时无动态查找。
type MiddlewareDeclarer interface {
	// Middleware 返回作用于该控制器全部动作的中间件。
	Middleware() []core.Handler
}

// Controller 是控制器基类。
//
// 业务控制器通过内嵌获得基础能力：
//
//	type Index struct {
//	    tapp.Controller
//	}
//
//	func (i *Index) Index(c *core.Ctx) error {
//	    return i.Success(c, "hello")
//	}
//
// 说明：Controller 不持有请求状态。控制器是单例复用的，
// 请求相关信息一律通过 *core.Ctx 显式传入，天然并发安全且零分配。
type Controller struct {
	// validator 是控制器级校验器，由 SetValidator 注入。
	validator Validator
}

// SetValidator 注入控制器使用的校验器，通常在应用装配期调用一次。
func (ctrl *Controller) SetValidator(v Validator) { ctrl.validator = v }

// Validator 返回控制器当前使用的校验器，未注入时返回全局默认校验器。
func (ctrl *Controller) Validator() Validator {
	if ctrl.validator != nil {
		return ctrl.validator
	}
	return DefaultValidator()
}

/* ------------------------------------------------------------------ */
/* 校验                                                                */
/* ------------------------------------------------------------------ */

// Validate 对数据执行规则校验。
//
// 校验失败时返回 errors.ErrValidation 派生的结构化错误，
// 其中 Meta["fields"] 携带字段级错误信息，可直接交给异常处理器渲染。
func (ctrl *Controller) Validate(data any, rules any) error {
	return ctrl.Validator().Validate(data, rules)
}

// BindValidate 绑定请求数据到 obj 并执行规则校验，
// 是控制器中最常用的入参处理方式。
//
//	var req LoginReq
//	if err := ctrl.BindValidate(c, &req, rules); err != nil {
//	    return err
//	}
func (ctrl *Controller) BindValidate(c *core.Ctx, obj any, rules any) error {
	if err := c.BindAll(obj); err != nil {
		return BindError(err)
	}
	if rules == nil {
		return nil
	}
	return ctrl.Validate(obj, rules)
}

// Bind 绑定请求数据（uri < query < body）。
// 绑定失败返回统一的参数错误，便于异常处理器归类。
func (ctrl *Controller) Bind(c *core.Ctx, obj any) error {
	if err := c.BindAll(obj); err != nil {
		return BindError(err)
	}
	return nil
}
