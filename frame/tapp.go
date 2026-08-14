package t

import (
	"strings"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/ttrace"
	"github.com/xmszy/tingo/tapp"
)

// init 把调试工具栏与多应用配置提供器挂到 tapp。
func init() {
	tapp.TraceConfigProvider = func() (ttrace.Config, bool) {
		// 配置键为 debug（config/app.toml 顶层）。
		if !Config().Bool("debug", false) {
			return ttrace.Config{}, false
		}
		cfg := ttrace.Default().Config
		// 用 config/trace.toml 覆盖细节（type / channel / [panels]）。
		_ = Config().DecodeAt("trace", &cfg)
		return cfg, true
	}

	// 多应用配置驱动：根据 config/app.toml [app] 段解析每个应用的路由前缀 / 域名 / 默认 / 禁用。
	// 配置项：default_app / app_map / domain_bind / deny_app。
	core.AppConfigProvider = func(name string, base core.AppConfig) core.AppConfig {
		cfg := base

		// default_app：该应用作为默认应用，挂载到根路径（无前缀）。
		if def := Config().String("default_app"); def != "" {
			if name == def {
				cfg.Default = true
				cfg.Prefix = "/"
			}
		}

		// app_map：url 段 -> 应用名。命中则把应用挂载到对应 url 段。
		var appMap map[string]string
		if Config().DecodeAt("app_map", &appMap) == nil {
			for seg, app := range appMap {
				if app == name {
					cfg.Prefix = "/" + seg
					cfg.Default = false
					break
				}
			}
		}

		// domain_bind：域名 -> 应用名。命中则按域名隔离，无路径前缀。
		var domainBind map[string]string
		if Config().DecodeAt("domain_bind", &domainBind) == nil {
			for domain, app := range domainBind {
				if app == name {
					cfg.Domain = domain
					cfg.Prefix = "/"
					cfg.Default = false
					break
				}
			}
		}

		// deny_app：禁止通过 url 直接访问的应用（仅作内部复用）。
		if denies := Config().Strings("deny_app"); len(denies) > 0 {
			for _, d := range denies {
				if strings.TrimSpace(d) == name {
					cfg.Disabled = true
					break
				}
			}
		}

		// 非默认、未绑定域名、且未通过 app_map 指定前缀的普通应用，
		// 按应用名自动挂载到 /应用名，避免与默认应用抢占总根路径 "/"。
		if !cfg.Default && cfg.Domain == "" && cfg.Prefix == "" {
			cfg.Prefix = "/" + name
		}

		return cfg
	}
}

/* ------------------------------------------------------------------ */
/* 应用层：类型别名                                                      */
/* ------------------------------------------------------------------ */

type (
	// Controller 是控制器基类。
	Controller = tapp.Controller
	// BaseApp 是业务应用基类，内嵌它即可获得配置/中间件/Boot 的默认实现。
	// 注意与注册函数 t.App() 区分。
	BaseApp = tapp.Base
	// Kernel 是应用装配内核。
	Kernel = tapp.Kernel
	// Request 是请求读取器。
	Request = tapp.Request
	// Result 是统一 JSON 响应结构。
	Result = tapp.Result
	// ExceptionHandle 是全局异常处理器。
	ExceptionHandle = tapp.ExceptionHandle

	// Service 是系统服务契约。
	Service = tapp.KernelService
	// BootableService 是可在注册完成后初始化的服务。
	BootableService = tapp.BootableKernelService
	// ServiceFunc 让普通函数充当系统服务。
	ServiceFunc = tapp.KernelServiceFunc

	// Validator 是校验器契约。
	Validator = tapp.Validator
	// ValidatorFunc 让普通函数充当校验器。
	ValidatorFunc = tapp.ValidatorFunc
	// Initializer 是控制器初始化钩子。
	Initializer = tapp.Initializer
	// Reporter 是异常上报契约。
	Reporter = tapp.Reporter
	// ReporterFunc 让普通函数充当异常上报器。
	ReporterFunc = tapp.ReporterFunc
	// LogReporter 是写入 tlog 的默认异常上报器。
	LogReporter = tapp.LogReporter
)

/* ------------------------------------------------------------------ */
/* 应用层：函数                                                          */
/* ------------------------------------------------------------------ */

var (
	// NewKernel 创建应用装配内核。
	NewKernel = tapp.NewKernel
	// NewExceptionHandle 创建全局异常处理器。
	NewExceptionHandle = tapp.NewExceptionHandle
	// NewLogReporter 创建日志异常上报器。
	NewLogReporter = tapp.NewLogReporter

	// Req 从上下文创建请求读取器。
	Req = tapp.Req

	// Recover 返回把 panic 交给异常处理器的中间件。
	Recover = tapp.Recover
	// NoRoute 返回交给异常处理器渲染的 404 处理器。
	NoRoute = tapp.NoRoute
	// NoMethod 返回交给异常处理器渲染的 405 处理器。
	NoMethod = tapp.NoMethod

	// Abort 中断请求并抛出指定状态码的异常。
	Abort = tapp.Abort
	// AbortIf 在条件成立时中断请求。
	AbortIf = tapp.AbortIf

	// Validate 使用全局默认校验器执行校验。
	Validate = tapp.Validate
	// SetValidator 注入全局默认校验器。
	SetValidator = tapp.SetDefaultValidator
	// ValidationError 构造带字段明细的校验错误。
	ValidationError = tapp.ValidationError

	// Snake 把驼峰转为下划线命名。
	Snake = tapp.Snake
	// Camel 把下划线转为大驼峰。
	Camel = tapp.Camel
	// LowerCamel 把下划线转为小驼峰。
	LowerCamel = tapp.LowerCamel

	// RegisterController 把一个控制器登记到自动路由表（通常在 init() 中调用）。
	RegisterController = tapp.RegisterController
	// AutoRoute 把自动路由表中登记的全部控制器按约定批量注册。
	AutoRoute = tapp.AutoRoute
)
