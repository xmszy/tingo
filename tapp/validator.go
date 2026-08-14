package tapp

import (
	"sync"

	"github.com/xmszy/tingo/os/tvalid"
)

/* ------------------------------------------------------------------ */
/* 校验器抽象                                                            */
/* ------------------------------------------------------------------ */

// Validator 是校验器契约。
//
// 主模块只定义契约不绑定实现，具体校验器（如 tvalid）
// 由业务方在装配期通过 SetDefaultValidator 注入，
// 从而避免主模块与特定校验实现强耦合。
type Validator interface {
	// Validate 校验 data 是否满足 rules。
	// rules 为 map[string]string 形式的规则表（tvalid.RuleSpec）。
	// 校验失败应返回 errors.ErrValidation 派生的结构化错误。
	Validate(data any, rules tvalid.RuleSpec) error

	// CheckStruct 按结构体字段的 valid tag 校验 v，可选 scene 指定场景规则。
	// 直接支撑 Request.Validate 与 BindAndValid 的场景化校验诉求。
	CheckStruct(v any, scene ...string) error
}

// ValidatorFunc 让普通函数可作为 Validator 使用。
type ValidatorFunc func(data any, rules tvalid.RuleSpec) error

// Validate 实现 Validator 接口。
func (f ValidatorFunc) Validate(data any, rules tvalid.RuleSpec) error { return f(data, rules) }

// CheckStruct 让普通函数满足 Validator 接口（默认放行）。
func (f ValidatorFunc) CheckStruct(any, ...string) error { return nil }

var (
	validatorMu      sync.RWMutex
	defaultValidator Validator = &tvalidValidator{}
)

// SetDefaultValidator 注入全局默认校验器，通常在应用装配期调用一次。
//
//	tapp.SetDefaultValidator(validate.New())
func SetDefaultValidator(v Validator) {
	if v == nil {
		v = &tvalidValidator{}
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
func Validate(data any, rules tvalid.RuleSpec) error { return DefaultValidator().Validate(data, rules) }

// tvalidValidator 是内置的默认校验器，基于 tvalid 实现结构体场景化校验。
type tvalidValidator struct{}

func (tvalidValidator) Validate(data any, rules tvalid.RuleSpec) error {
	// 通用规则表场景：把 data/rules 透传给 tvalid.Check。
	if dm, ok := data.(map[string]any); ok {
		return tvalid.Check(dm, rules)
	}
	return nil
}

func (tvalidValidator) CheckStruct(v any, scene ...string) error {
	if len(scene) > 0 && scene[0] != "" {
		return tvalid.CheckStructWithScene(v, scene[0])
	}
	return tvalid.CheckStruct(v)
}

// noopValidator 是未注入实现时的占位校验器，直接放行。
// 这样未使用校验能力的项目无需引入任何校验依赖。
type noopValidator struct{}

func (noopValidator) Validate(any, tvalid.RuleSpec) error { return nil }
func (noopValidator) CheckStruct(any, ...string) error    { return nil }
