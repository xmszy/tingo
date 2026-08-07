package tapp

import "sync"

/* ------------------------------------------------------------------ */
/* 校验器抽象                                                            */
/* ------------------------------------------------------------------ */

// Validator 是校验器契约。
//
// 主模块只定义契约不绑定实现，具体校验器（如 contrib/validate）
// 由业务方在装配期通过 SetDefaultValidator 注入，
// 从而避免主模块依赖 contrib 子模块。
type Validator interface {
	// Validate 校验 data 是否满足 rules。
	// rules 的具体类型由实现约定，通常是 map[string]string 形式的规则表。
	// 校验失败应返回 errors.ErrValidation 派生的结构化错误。
	Validate(data any, rules any) error
}

// ValidatorFunc 让普通函数可作为 Validator 使用。
type ValidatorFunc func(data any, rules any) error

// Validate 实现 Validator 接口。
func (f ValidatorFunc) Validate(data any, rules any) error { return f(data, rules) }

var (
	validatorMu      sync.RWMutex
	defaultValidator Validator = noopValidator{}
)

// SetDefaultValidator 注入全局默认校验器，通常在应用装配期调用一次。
//
//	tapp.SetDefaultValidator(validate.New())
func SetDefaultValidator(v Validator) {
	if v == nil {
		v = noopValidator{}
	}
	validatorMu.Lock()
	defaultValidator = v
	validatorMu.Unlock()
}

// DefaultValidator 返回全局默认校验器。
func DefaultValidator() Validator {
	validatorMu.RLock()
	v := defaultValidator
	validatorMu.RUnlock()
	return v
}

// Validate 使用全局默认校验器执行校验。
func Validate(data any, rules any) error { return DefaultValidator().Validate(data, rules) }

// noopValidator 是未注入实现时的占位校验器，直接放行。
// 这样未使用校验能力的项目无需引入任何校验依赖。
type noopValidator struct{}

func (noopValidator) Validate(any, any) error { return nil }
