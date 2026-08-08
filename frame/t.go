// Package t 是 tingo 的门面包，业务代码通常只需 import 这一个包。
//
// 设计取向：
//   - 全部为类型别名与函数变量，不引入任何间接层，因此零运行时开销；
//   - 别名而非包装，意味着 t.Ctx 与 core.Ctx 是同一类型，可自由互换；
//   - 与传统 Facade 不同，这里没有反射与容器查找。
package t

import (
	"context"
	"time"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/database/tdb"
	"github.com/xmszy/tingo/errors"
	"github.com/xmszy/tingo/net/thttp"
	"github.com/xmszy/tingo/os/tcache"
	"github.com/xmszy/tingo/os/tcfg"
	"github.com/xmszy/tingo/os/tconsole"
	"github.com/xmszy/tingo/os/tcookie"
	"github.com/xmszy/tingo/os/tcron"
	"github.com/xmszy/tingo/os/tenv"
	"github.com/xmszy/tingo/os/tevent"
	"github.com/xmszy/tingo/os/tfilesystem"
	"github.com/xmszy/tingo/os/tlang"
	"github.com/xmszy/tingo/os/tlog"
	"github.com/xmszy/tingo/os/tqueue"
	"github.com/xmszy/tingo/os/tsession"
	"github.com/xmszy/tingo/os/ttrace"
	"github.com/xmszy/tingo/os/tview"
	"github.com/xmszy/tingo/os/tcrypto/aes"
	"github.com/xmszy/tingo/os/tcrypto/crc32"
	"github.com/xmszy/tingo/os/tcrypto/des"
	"github.com/xmszy/tingo/os/tcrypto/md5"
	"github.com/xmszy/tingo/os/tcrypto/rsa"
	"github.com/xmszy/tingo/os/tcrypto/sha1"
	"github.com/xmszy/tingo/os/tcrypto/sha256"
	"github.com/xmszy/tingo/os/tcrypto/sha512"

	// 新增：encoding
	tencBase64 "github.com/xmszy/tingo/os/tencoding/base64"
	"github.com/xmszy/tingo/os/tencoding/compress"
	"github.com/xmszy/tingo/os/tencoding/hash"
	"github.com/xmszy/tingo/os/tencoding/html"
	"github.com/xmszy/tingo/os/tencoding/json"
	"github.com/xmszy/tingo/os/tencoding/toml"
	"github.com/xmszy/tingo/os/tencoding/url"
	"github.com/xmszy/tingo/os/tencoding/xml"
	"github.com/xmszy/tingo/os/tencoding/yaml"

	// 新增：容器
	"github.com/xmszy/tingo/os/tcontainer"

	// 新增：工具
	"github.com/xmszy/tingo/os/tconv"
	"github.com/xmszy/tingo/os/tdebug"
	"github.com/xmszy/tingo/os/tfile"
	"github.com/xmszy/tingo/os/tpage"
	"github.com/xmszy/tingo/os/tpool"
	"github.com/xmszy/tingo/os/tproc"
	"github.com/xmszy/tingo/os/trand"
	"github.com/xmszy/tingo/os/tregex"
	"github.com/xmszy/tingo/os/tres"
	"github.com/xmszy/tingo/os/tstr"
	"github.com/xmszy/tingo/os/tstructs"
	"github.com/xmszy/tingo/os/ttime"
	"github.com/xmszy/tingo/os/ttimer"
	_ "github.com/xmszy/tingo/os/tutil" // 泛型工具函数需带类型参数直接调用
	"github.com/xmszy/tingo/os/tuuid"

	// 新增：net/client & validation & mutex & pipeline
	"github.com/xmszy/tingo/net/tclient"
	"github.com/xmszy/tingo/os/tbuild"
	"github.com/xmszy/tingo/os/tctx"
	"github.com/xmszy/tingo/os/tmutex"
	"github.com/xmszy/tingo/os/tpipeline"
	"github.com/xmszy/tingo/os/tspath"
	"github.com/xmszy/tingo/os/tvalid"
)

/* ------------------------------------------------------------------ */
/* 核心类型别名                                                          */
/* ------------------------------------------------------------------ */

type (
	// Ctx 是请求上下文。
	Ctx = core.Ctx
	// Handler 是请求处理器。
	Handler = core.Handler
	// HandlerE 是可返回错误的处理器。
	HandlerE = core.HandlerE
	// Middleware 是中间件。
	Middleware = core.Middleware
	// Router 是路由注册器。
	Router = core.Router
	// Application 是应用契约。
	Application = core.Application
	// AppConfig 是应用配置。
	AppConfig = core.AppConfig
	// Container 是服务容器。
	Container = core.Container
	// Responder 是响应协议。
	Responder = core.Responder
	// Engine 是 HTTP 引擎。
	Engine = thttp.Engine
	// Option 是引擎配置项。
	Option = thttp.Option
	// Error 是结构化错误。
	Error = errors.Error
)

// Map 是通用的字符串键映射。
type Map = map[string]any

/* ------------------------------------------------------------------ */
/* 应用与路由                                                           */
/* ------------------------------------------------------------------ */

// App 注册一个应用，通常在应用包的 init 中调用。
//
//	func init() { t.App("admin", &App{}) }
var App = core.RegisterApp

// Apps 返回全部已注册的应用。
var Apps = core.Apps

/* ------------------------------------------------------------------ */
/* Handler 包装                                                        */
/* ------------------------------------------------------------------ */

// W 将 func(*Ctx, *Req) (Res, error) 包装为 Handler，零反射。
func W[Req any, Res any](f func(*Ctx, *Req) (Res, error)) Handler {
	return core.W(f)
}

// WN 将 func(*Ctx) (Res, error) 包装为 Handler，零反射。
func WN[Res any](f func(*Ctx) (Res, error)) Handler {
	return core.WN(f)
}

// Adapt 将任意受支持签名的函数适配为 Handler。
var Adapt = core.Adapt

/* ------------------------------------------------------------------ */
/* 容器                                                                */
/* ------------------------------------------------------------------ */

// Bind 绑定一个懒加载单例服务。
func Bind[T any](f func(*Container) (T, error)) { core.Bind(core.Default(), f) }

// BindValue 绑定一个已构造的实例。
func BindValue[T any](v T) { core.BindValue(core.Default(), v) }

// Get 从默认容器解析服务。
func Get[T any]() (T, error) { return core.Resolve[T](core.Default()) }

// MustGet 从默认容器解析服务，失败时 panic。
func MustGet[T any]() T { return core.MustResolve[T](core.Default()) }

/* ------------------------------------------------------------------ */
/* 请求级类型安全键                                                      */
/* ------------------------------------------------------------------ */

// Key 创建一个类型安全的请求级键。
func Key[T any](name string) core.CtxKey[T] { return core.NewCtxKey[T](name) }

/* ------------------------------------------------------------------ */
/* 错误                                                                */
/* ------------------------------------------------------------------ */

// NewError 创建结构化错误。
var NewError = errors.NewError

// Errorf 创建消息带格式化的结构化错误。
var Errorf = errors.Newf

// Is 判断错误链中是否存在目标错误。
var Is = errors.Is

// As 从错误链中提取指定类型的错误。
var As = errors.As

// ErrorOf 从任意 error 提取结构化错误。
var ErrorOf = errors.From

// ErrorHasCode 检查错误链中是否存在指定业务码（递归 Unwrap）。
var ErrorHasCode = errors.HasCode

// ErrorCodeOf 返回错误链中第一个 *Error 的业务码。
var ErrorCodeOf = errors.CodeOf

// ErrorStatusOf 返回错误链中第一个 *Error 的 HTTP 状态码。
var ErrorStatusOf = errors.StatusOf

// 常用预置错误。
var (
	ErrBadRequest   = errors.ErrBadRequest
	ErrUnauthorized = errors.ErrUnauthorized
	ErrForbidden    = errors.ErrForbidden
	ErrNotFound     = errors.ErrNotFound
	ErrValidation   = errors.ErrValidation
	ErrInternal     = errors.ErrInternal
	ErrConflict     = errors.ErrConflict
)

/* ------------------------------------------------------------------ */
/* 基础组件 os：tenv / tcfg / tlog / tcache                            */
/* ------------------------------------------------------------------ */

type (
	// LogLevel 是日志级别。
	LogLevel = tlog.Level
	// Logger 是日志实例。
	Logger = tlog.Logger
	// LogField 是结构化字段。
	LogField = tlog.Field
	// Cache 是并发缓存。
	Cache = tcache.Cache
)

// 日志级别别名。
const (
	LogDebug = tlog.LevelDebug
	LogInfo  = tlog.LevelInfo
	LogWarn  = tlog.LevelWarn
	LogError = tlog.LevelError
	LogFatal = tlog.LevelFatal
)

// 日志标志别名。
const (
	LogTime = tlog.FTime
	LogFile = tlog.FFile
	LogFunc = tlog.FFunc
	LogStd  = tlog.FStd
)

// LogConfig 是日志配置别名。
type LogConfig = tlog.Config

// Log 包级默认 logger 实例。
var Log = tlog.New()

// 日志便捷函数（转发到 tlog 包级默认 logger）。
var (
	LogDebugf = tlog.Debugf
	LogInfof  = tlog.Infof
	LogWarnf  = tlog.Warnf
	LogErrorf = tlog.Errorf
	LogDebugw = tlog.Debugw
	LogInfow  = tlog.Infow
	LogWarnw  = tlog.Warnw
	LogErrorw = tlog.Errorw
)

// LogF 构造结构化日志字段。
var LogF = tlog.F

// NewLogger 创建自定义日志实例。
var NewLogger = tlog.NewWithConfig

// LogConfigured 从全局约定配置创建日志实例。
func LogConfigured() (*tlog.Logger, error) { return tlog.NewFromTree(Config()) }

// LogConfiguredFor 从指定应用配置视图创建日志实例。
func LogConfiguredFor(application string) (*tlog.Logger, error) {
	return tlog.NewFromTree(ConfigFor(application))
}

// LogConfiguredFrom 从当前请求所属应用配置创建日志实例。
func LogConfiguredFrom(ctx *Ctx) (*tlog.Logger, error) { return tlog.NewFromTree(ConfigFrom(ctx)) }

// Env 环境变量读取（泛型）。
func Env[T any](key string, def T) T { return tenv.Get(key, def) }

// EnvMust 强制读取环境变量，缺失 panic。
func EnvMust[T any](key string) T { return tenv.MustGet[T](key) }

// EnvSet 写入环境变量。
var EnvSet = tenv.Set

// EnvHas 判断环境变量是否存在，包括值为空的变量。
var EnvHas = tenv.Has

// EnvUnset 删除环境变量。
var EnvUnset = tenv.Unset

// EnvLookup 读取原始环境变量，并区分缺失与空值。
var EnvLookup = tenv.Lookup

// EnvValue 按特殊值语义读取动态环境变量。
var EnvValue = tenv.Value

// EnvAll 返回环境变量副本，可按前缀过滤。
var EnvAll = tenv.All

// EnvMap 读取逗号分隔的键值映射。
var EnvMap = tenv.GetMap

// EnvExpand 展开字符串中的环境变量占位符。
var EnvExpand = tenv.Expand

// EnvLoad 加载一个或多个 dotenv 文件。
var EnvLoad = tenv.Load

// Config 返回默认框架实例的全局只读配置。
func Config() *tcfg.Config {
	return core.MustResolve[*tcfg.Registry](core.Default()).Global()
}

// ConfigFor 返回默认框架实例中指定应用的只读配置。
func ConfigFor(application string) *tcfg.Config {
	return core.MustResolve[*tcfg.Registry](core.Default()).Application(application)
}

// ConfigFrom 返回当前请求所属应用的只读配置。
func ConfigFrom(ctx *Ctx) *tcfg.Config {
	registry := core.MustResolve[*tcfg.Registry](ctx.Framework().Container())
	return registry.ForContext(ctx)
}

// ConfigValue 按点路径读取配置值，不存在或类型不匹配时返回默认值。
func ConfigValue[T any](reader tcfg.Reader, path string, def T) T {
	value, err := tcfg.Value[T](reader, path)
	if err != nil {
		return def
	}
	return value
}

// ConfigWithAdapter 从自定义只读来源创建配置。
func ConfigWithAdapter(adapter tcfg.Adapter) *tcfg.Config { return tcfg.New(adapter) }

// ConfigWithTree 从内存配置树创建配置。
func ConfigWithTree(tree tcfg.Tree) *tcfg.Config { return tcfg.NewFromTree(tree) }

// ConfigWithBytes 从 TOML、YAML、JSON 或 INI 内容创建配置。
func ConfigWithBytes(format string, data []byte) (*tcfg.Config, error) {
	return tcfg.NewFromBytes(format, data)
}

// ConfigLoader 将配置路径绑定为强类型快照。
func ConfigLoader[T any](cfg *tcfg.Config, path string, target ...*T) (*tcfg.Loader[T], error) {
	return tcfg.NewLoader(cfg, path, target...)
}

// ConfigLoaderWithAdapter 从自定义来源创建强类型 Loader。
func ConfigLoaderWithAdapter[T any](adapter tcfg.Adapter, path string, target ...*T) (*tcfg.Loader[T], error) {
	return tcfg.NewLoaderWithAdapter(adapter, path, target...)
}

// ConfigDiscover 在指定目录自动发现配置文件（config.toml/yaml/json 等）。
// dir 为空时默认当前目录。
func ConfigDiscover(dir string) (tcfg.Tree, bool, error) { return tcfg.Discover(dir) }

// ConfigLoadInto 自动发现配置并解码到 target（需传入指针）。
func ConfigLoadInto(dir string, target any) (bool, error) { return tcfg.LoadInto(dir, target) }

// CacheNew 创建缓存。
var CacheNew = tcache.New

// CacheGlobal 返回进程级默认缓存。
var CacheGlobal = tcache.Global

// CacheGet 泛型读取缓存。
func CacheGet[T any](c *tcache.Cache, key string) (T, bool) { return tcache.Get[T](c, key) }

// CacheSet 泛型写入缓存。
func CacheSet[T any](c *tcache.Cache, key string, value T, ttl time.Duration) {
	tcache.SetT(c, key, value, ttl)
}

// CacheTagNew 创建与缓存关联的 TagSet。
func CacheTagNew(c *tcache.Cache) *tcache.TagSet { return tcache.NewTagSet(c) }

// CacheGlobalTag 全局缓存的 TagSet。
var CacheGlobalTag = &tcache.GlobalTag

// CacheRemember 读缓存；未命中时加锁执行 fn，防止缓存击穿。
func CacheRemember[V any](c *tcache.Cache, key string, ttl time.Duration, fn func() (V, error)) (V, error) {
	return tcache.RememberFunc(c, key, ttl, fn)
}

// CacheIncr 原子自增全局缓存数字值。
var CacheIncr = tcache.Global.Increment

// CacheDecr 原子自减全局缓存数字值。
var CacheDecr = tcache.Global.Decrement

// CachePull 获取并删除缓存值。
func CachePull[V any](c *tcache.Cache, key string) (V, bool) {
	return tcache.PullFunc[V](c, key)
}

/* ------------------------------------------------------------------ */
/* 数据库门面（tdb 泛型 ORM）                                          */
/* ------------------------------------------------------------------ */

// DB 是数据库连接句柄别名。
type DB = tdb.DB

// Tx 是事务句柄别名。
type Tx = tdb.Tx

// Model 是泛型模型别名。
type Model[T any] = tdb.Model[T]

// DBConfig 是数据库配置别名。
type DBConfig = tdb.Config

// 数据库错误别名。
var (
	DBErrReadOnly = tdb.ErrReadOnly
	DBErrNoWhere  = tdb.ErrNoWhere
	DBErrNoRows   = tdb.ErrNoRows
)

// Database 返回约定配置中的默认或命名连接，失败时 panic。
// 通常由生成模型内部调用；高级场景可继续使用 DBOpen。
func Database(names ...string) *tdb.DB { return tdb.MustConnection(names...) }

// DatabaseE 返回约定配置中的默认或命名连接。
func DatabaseE(names ...string) (*tdb.DB, error) { return tdb.Connection(names...) }

// DatabaseFor 返回指定业务应用作用域的默认或命名连接。
func DatabaseFor(application string, names ...string) (*tdb.DB, error) {
	return tdb.ConnectionForApplication(context.Background(), core.DefaultApp(), application, names...)
}

// DatabaseFrom 返回当前请求所属应用作用域的默认或命名连接。
func DatabaseFrom(ctx *Ctx, names ...string) (*tdb.DB, error) {
	return tdb.ConnectionForContext(ctx, names...)
}

// DBOpen 打开数据库连接。
func DBOpen(cfg tdb.Config) (*tdb.DB, error) { return tdb.Open(cfg) }

// DBMustOpen 打开数据库连接，失败 panic。
func DBMustOpen(cfg tdb.Config) *tdb.DB { return tdb.MustOpen(cfg) }

// DBModel 构造绑定到 DB 的泛型模型。table 可省略（从 T 推断）。
func DBModel[T any](db *tdb.DB, table ...string) *tdb.Model[T] {
	return tdb.NewModel[T](db, table...)
}

/* ------------------------------------------------------------------ */
/* 视图门面（tview）                                                   */
/* ------------------------------------------------------------------ */

// ViewEngine 是模板引擎别名。
type ViewEngine = tview.Engine

// ViewConfig 是约定式视图配置。
type ViewConfig = tview.Config

// ViewNew 创建模板引擎。root 为模板根目录。
func ViewNew(root string, opts ...tview.Option) *tview.Engine { return tview.New(root, opts...) }

// ViewConfigured 从全局约定配置创建视图引擎。
func ViewConfigured() *tview.Engine { return tview.NewFromTree(Config()) }

// ViewConfiguredFor 从指定应用配置视图创建视图引擎。
func ViewConfiguredFor(application string) *tview.Engine {
	return tview.NewFromTree(ConfigFor(application))
}

// ViewConfiguredFrom 从当前请求所属应用配置创建视图引擎。
func ViewConfiguredFrom(ctx *Ctx) *tview.Engine { return tview.NewFromTree(ConfigFrom(ctx)) }

// 视图配置项别名。
var (
	ViewWithExt    = tview.WithExt
	ViewWithDelims = tview.WithDelims
	ViewWithFuncs  = tview.WithFuncs
)

/* ------------------------------------------------------------------ */
/* 会话门面（tsession）                                               */
/* ------------------------------------------------------------------ */

// Session 是会话别名。
type Session = tsession.Session

// SessionManager 是会话管理器别名。
type SessionManager = tsession.Manager

// SessionConfig 是会话配置别名。
type SessionConfig = tsession.Config

// SessionNew 创建会话管理器。
func SessionNew(cfg tsession.Config) *tsession.Manager { return tsession.New(cfg) }

// SessionConfigured 从全局约定配置创建会话管理器。
func SessionConfigured() (*tsession.Manager, error) { return tsession.NewFromTree(Config()) }

// SessionConfiguredFor 从指定应用配置视图创建会话管理器。
func SessionConfiguredFor(application string) (*tsession.Manager, error) {
	return tsession.NewFromTree(ConfigFor(application))
}

// SessionConfiguredFrom 从当前请求所属应用配置创建会话管理器。
func SessionConfiguredFrom(ctx *Ctx) (*tsession.Manager, error) {
	return tsession.NewFromTree(ConfigFrom(ctx))
}

// SessionGet 读取会话中泛型值。
func SessionGet[T any](s *tsession.Session, key string) (T, bool) { return tsession.Get[T](s, key) }

// SessionMemoryStore 创建内存存储。
var SessionMemoryStore = tsession.NewMemoryStore

// SessionDBStore 创建数据库存储。
var SessionDBStore = tsession.NewDBStore

/* ------------------------------------------------------------------ */
/* 国际化门面（tlang）                                                */
/* ------------------------------------------------------------------ */

// Translator 是翻译器别名。
type Translator = tlang.Translator

// PluralRule 复数规则别名。
type PluralRule = tlang.PluralRule

// PluralForm 复数形式别名。
type PluralForm = tlang.PluralForm

// 内置复数规则。
var (
	ChinesePluralRule = tlang.ChinesePluralRule
	EnglishPluralRule = tlang.EnglishPluralRule
	FrenchPluralRule  = tlang.FrenchPluralRule
)

// LangNew 创建翻译器。locale 默认语言，fallback 回退语言。
func LangNew(locale, fallback string) *tlang.Translator { return tlang.New(locale, fallback) }

// LangSetLocale 切换翻译器当前语言。
var LangSetLocale = (*tlang.Translator).SetLocale

// LangSetPluralRule 设置翻译器复数规则。
var LangSetPluralRule = (*tlang.Translator).SetPluralRule

// LangTranslatePlural 翻译复数消息，count 选择复数形式。
func LangTranslatePlural(tr *tlang.Translator, key string, count int, params ...any) string {
	return tr.TranslatePlural(key, count, params...)
}

// LangTranslatePluralFor 按指定语言翻译复数消息。
func LangTranslatePluralFor(tr *tlang.Translator, locale, key string, count int, params ...any) string {
	return tr.TranslatePluralFor(locale, key, count, params...)
}

// LangTranslateCtx 从 context 读取 locale 翻译。
func LangTranslateCtx(tr *tlang.Translator, ctx context.Context, key string, params ...any) string {
	return tr.TranslateCtx(ctx, key, params...)
}

// LangTranslatePluralCtx 从 context 读取 locale 翻译复数消息。
func LangTranslatePluralCtx(tr *tlang.Translator, ctx context.Context, key string, count int, params ...any) string {
	return tr.TranslatePluralCtx(ctx, key, count, params...)
}

// LangTranslateTpl 使用 Go text/template 渲染译文。
func LangTranslateTpl(tr *tlang.Translator, key string, params ...any) string {
	return tr.TranslateTpl(key, params...)
}

// LangTranslatePluralTpl 使用 Go text/template 渲染复数译文。
func LangTranslatePluralTpl(tr *tlang.Translator, key string, count int, params ...any) string {
	return tr.TranslatePluralTpl(key, count, params...)
}

// LangSetLocaleCtx 设置 context 中的 locale。
var LangSetLocaleCtx = tlang.SetLocaleCtx

// LangLocaleFromCtx 从 context 提取 locale。
var LangLocaleFromCtx = tlang.LocaleFromCtx

/* ------------------------------------------------------------------ */
/* 事件总线门面（tevent）                                             */
/* ------------------------------------------------------------------ */

// Event 是事件类型别名。
type Event[T any] = tevent.Event[T]

// Bus 是事件总线别名。
type Bus = tevent.Bus

// EventNew 声明一个事件。
func EventNew[T any](name string) tevent.Event[T] { return tevent.New[T](name) }

// BusNew 创建事件总线。async=true 时异步分发。
func BusNew(async bool) *tevent.Bus { return tevent.NewBus(async) }

// BusSubscribe 注册监听器。
func BusSubscribe[T any](b *tevent.Bus, ev tevent.Event[T], h tevent.Handler[T]) uint64 {
	return tevent.Subscribe(b, ev, h)
}

// BusOnce 注册一次性监听器。
func BusOnce[T any](b *tevent.Bus, ev tevent.Event[T], h tevent.Handler[T]) uint64 {
	return tevent.Once(b, ev, h)
}

// BusSubscribePattern 注册前缀通配订阅。
// pattern 为事件名前缀（如 "user."），当分发 "user.login" 时该监听器也会触发。
// 若要监听所有事件，使用 "" 作为 pattern，handler 使用 Handler[any]。
func BusSubscribePattern[T any](b *tevent.Bus, pattern string, h tevent.Handler[T]) uint64 {
	return tevent.SubscribePattern(b, pattern, h)
}

// BusDispatch 同步分发。
func BusDispatch[T any](b *tevent.Bus, ctx context.Context, ev tevent.Event[T], p T) error {
	return tevent.Dispatch(b, ctx, ev, p)
}

// BusDispatchAsync 异步分发。
func BusDispatchAsync[T any](b *tevent.Bus, ctx context.Context, ev tevent.Event[T], p T) {
	tevent.DispatchAsync(b, ctx, ev, p)
}

/* ------------------------------------------------------------------ */
/* 任务队列门面（tqueue）                                             */
/* ------------------------------------------------------------------ */

// QueueMessage 是队列消息别名。
type QueueMessage[T any] = tqueue.Message[T]

// QueueHandler 是消费者函数别名。
type QueueHandler[T any] = tqueue.Handler[T]

// QueueMemory 是内存队列别名。
type QueueMemory[T any] = tqueue.MemoryQueue[T]

// QueueNew 创建内存队列。async 异步消费，maxRetry 最大重试次数。
func QueueNew[T any](async bool, maxRetry int) *tqueue.MemoryQueue[T] {
	return tqueue.NewMemory[T](async, maxRetry)
}

// QueuePublishMessage 投递携带 Headers 的完整消息。
func QueuePublishMessage[T any](q *tqueue.MemoryQueue[T], ctx context.Context, msg tqueue.Message[T]) error {
	return q.PublishMessage(ctx, msg)
}

// QueueGetHeader 安全读取消息 Header 值。
func QueueGetHeader[T any](msg *tqueue.Message[T], key string) string {
	return msg.GetHeader(key)
}

// QueueSetHeader 设置消息 Header 值。
func QueueSetHeader[T any](msg *tqueue.Message[T], key, value string) {
	msg.SetHeader(key, value)
}

// QueuePublishDelay 延迟投递消息（delaySeconds 秒后消费）。
func QueuePublishDelay[T any](q *tqueue.MemoryQueue[T], ctx context.Context, payload T, delaySeconds int64) {
	q.PublishDelay(ctx, payload, delaySeconds)
}

/* ------------------------------------------------------------------ */
/* 定时任务门面（tcron）                                              */
/* ------------------------------------------------------------------ */

// Cron 是调度器别名。
type Cron = tcron.Scheduler

// CronNew 创建调度器。loc 时区，传 nil 用本地时区。
func CronNew(loc *time.Location) *tcron.Scheduler { return tcron.New(loc) }

/* ------------------------------------------------------------------ */
/* 文件系统门面（tfilesystem）                                        */
/* ------------------------------------------------------------------ */

// FilesystemDisk 是文件系统磁盘别名。
type FilesystemDisk = tfilesystem.Disk

// FilesystemManager 是文件系统管理器别名。
type FilesystemManager = tfilesystem.Manager

// FilesystemConfig 是文件系统配置别名。
type FilesystemConfig = tfilesystem.Config

// FilesystemReady 标记已注册的驱动（local 已内置）。
var FilesystemDrivers = tfilesystem.Drivers

// FilesystemNew 从配置创建文件系统管理器。
func FilesystemNew(cfg tfilesystem.Config) (*tfilesystem.Manager, error) {
	return tfilesystem.New(cfg)
}

/* ------------------------------------------------------------------ */
/* 控制台门面（tconsole）                                             */
/* ------------------------------------------------------------------ */

// ConsoleCommand 是控制台指令接口别名。
type ConsoleCommand = tconsole.Command

// ConsoleRegistry 是控制台注册表别名。
type ConsoleRegistry = tconsole.Registry

// ConsoleDefault 返回全局默认控制台注册表。
var ConsoleDefault = tconsole.DefaultRegistry

// ConsoleNew 创建新的控制台注册表。
var ConsoleNew = tconsole.NewRegistry

// ConsoleRegister 向全局默认注册表注册指令。
var ConsoleRegisterFn = tconsole.Register

// ConsoleRun 在全局注册表中运行指令。
var ConsoleRun = tconsole.DefaultRegistry().Run

/* ------------------------------------------------------------------ */
/* 调试工具栏门面（ttrace）                                           */
/* ------------------------------------------------------------------ */

// TraceToolbar 是调试工具栏别名。
type TraceToolbar = ttrace.Toolbar

// TraceConfig 是调试工具栏配置别名。
type TraceConfig = ttrace.Config

// TraceDefault 返回默认调试工具栏。
var TraceDefault = ttrace.Default

// TraceSQL 向工具栏记录一条 SQL 查询。
var TraceSQL = ttrace.LogSQL


// 本函数保留作为手动覆盖入口（例如仅对某些应用显式开启），通常不必要。
func EnableToolbar() Handler { return ttrace.Default().Handler() }

// TraceClearSQL 清空 SQL 记录。
var TraceClearSQL = ttrace.ClearSQL

// TraceBookmarklet 返回一段浏览器书签脚本，点击即可把任意 tingo 页面
// （含 JSON/API）响应头中的 X-Tingo-Trace 渲染成浮动调试条。
// 用法：把返回值保存为浏览器收藏夹书签的 URL。
var TraceBookmarklet = ttrace.Bookmarklet

/* ------------------------------------------------------------------ */
/* 加密门面（tcrypto）                                                  */
/* ------------------------------------------------------------------ */

// MD5Encrypt 计算 MD5 摘要（hex 字符串）。
var MD5Encrypt = md5.Encrypt

// MD5MustEncrypt 计算 MD5 摘要（hex 字符串），失败 panic。
var MD5MustEncrypt = md5.MustEncrypt

// MD5EncryptString 计算字符串的 MD5 摘要。
var MD5EncryptString = md5.EncryptString

// MD5EncryptFile 计算文件的 MD5 摘要。
var MD5EncryptFile = md5.EncryptFile

// SHA1Encrypt 计算 SHA-1 摘要。
var SHA1Encrypt = sha1.Encrypt

// SHA1EncryptString 计算字符串的 SHA-1 摘要。
var SHA1EncryptString = sha1.EncryptString

// SHA256Encrypt 计算 SHA-256 摘要。
var SHA256Encrypt = sha256.Encrypt

// SHA256EncryptString 计算字符串的 SHA-256 摘要。
var SHA256EncryptString = sha256.EncryptString

// SHA256EncryptFile 计算文件的 SHA-256 摘要。
var SHA256EncryptFile = sha256.EncryptFile

// SHA512Encrypt 计算 SHA-512 摘要。
var SHA512Encrypt = sha512.Encrypt

// SHA512Encrypt384 计算 SHA-384 摘要。
var SHA512Encrypt384 = sha512.Encrypt384

// CRC32Encrypt 计算 CRC-32 校验值。
var CRC32Encrypt = crc32.Encrypt

// CRC32EncryptBytes 计算字节的 CRC-32 校验值。
var CRC32EncryptBytes = crc32.EncryptBytes

// AESEncrypt 使用 AES-CBC 加密。
var AESEncrypt = aes.Encrypt

// AESDecrypt 使用 AES-CBC 解密。
var AESDecrypt = aes.Decrypt

// AESEncryptCBC AES-CBC 加密。
var AESEncryptCBC = aes.EncryptCBC

// AESDecryptCBC AES-CBC 解密。
var AESDecryptCBC = aes.DecryptCBC

// AESEncryptCFB AES-CFB 加密。
var AESEncryptCFB = aes.EncryptCFB

// AESDecryptCFB AES-CFB 解密。
var AESDecryptCFB = aes.DecryptCFB

// DESEncrypt 使用 DES-CBC 加密。
var DESEncrypt = des.Encrypt

// DESDecrypt 使用 DES-CBC 解密。
var DESDecrypt = des.Decrypt

// DESEncryptTriple 使用 3DES-CBC 加密。
var DESEncryptTriple = des.EncryptTripleCBC

// DESDecryptTriple 使用 3DES-CBC 解密。
var DESDecryptTriple = des.DecryptTripleCBC

// RSAEncrypt 使用 RSA 加密。
var RSAEncrypt = rsa.Encrypt

// RSADecrypt 使用 RSA 解密。
var RSADecrypt = rsa.Decrypt

// RSAEncryptOAEP 使用 RSA-OAEP 加密。
var RSAEncryptOAEP = rsa.EncryptOAEP

// RSADecryptOAEP 使用 RSA-OAEP 解密。
var RSADecryptOAEP = rsa.DecryptOAEP

// RSAGenerateKeyPair 生成 RSA 密钥对。
var RSAGenerateKeyPair = rsa.GenerateDefaultKeyPair

/* ------------------------------------------------------------------ */
/* 编解码门面（tencoding）                                              */
/* ------------------------------------------------------------------ */

// Base64Encode 标准 Base64 编码。
var Base64Encode = tencBase64.Encode

// Base64Decode 标准 Base64 解码。
var Base64Decode = tencBase64.Decode

// Base64EncodeURL URL 安全 Base64 编码。
var Base64EncodeURL = tencBase64.EncodeURL

// Base64DecodeURL URL 安全 Base64 解码。
var Base64DecodeURL = tencBase64.DecodeURL

// HtmlEscape HTML 转义。
var HtmlEscape = html.Escape

// HtmlUnescape HTML 反转义。
var HtmlUnescape = html.Unescape

// HtmlStripTags 移除 HTML 标签。
var HtmlStripTags = html.StripTags

// URLEncode URL 编码。
var URLEncode = url.Encode

// URLDecode URL 解码。
var URLDecode = url.Decode

// URLBuildQuery 构建 query string。
var URLBuildQuery = url.BuildQuery

// URLParseQuery 解析 query string。
var URLParseQuery = url.ParseQuery

// JSONMarshal JSON 序列化。
var JSONMarshal = json.Marshal

// JSONUnmarshal JSON 反序列化。
var JSONUnmarshal = json.Unmarshal

// JSONValid 判断 JSON 是否合法。
var JSONValid = json.Valid

// JSONNew 创建动态 JSON 对象。
var JSONNew = json.New

// XMLMarshal XML 序列化。
var XMLMarshal = xml.Marshal

// XMLUnmarshal XML 反序列化。
var XMLUnmarshal = xml.Unmarshal

// YAMLMarshal YAML 序列化。
var YAMLMarshal = yaml.Marshal

// YAMLUnmarshal YAML 反序列化。
var YAMLUnmarshal = yaml.Unmarshal

/* ------------------------------------------------------------------ */
/* Cookie 门面（tcookie）                                              */
/* ------------------------------------------------------------------ */

// CookieOptions Cookie 设置选项别名。
type CookieOptions = tcookie.Options

// CookieNew 创建 Cookie 选项。
var CookieNew = tcookie.New

// CookieSet 写入 Cookie。
var CookieSet = tcookie.Set

// CookieGet 读取 Cookie 值。
var CookieGet = tcookie.Get

// CookieHas 判断 Cookie 是否存在。
var CookieHas = tcookie.Has

// CookieForget 删除 Cookie。
var CookieForget = tcookie.Forget

// CookieFlash 一次性闪存（读取后删除）。
var CookieFlash = tcookie.Flash

// CookieDebugInfo 返回 Cookie 调试信息。
var CookieDebugInfo = tcookie.DebugInfo

// YAMLToJSON YAML 转 JSON。
var YAMLToJSON = yaml.ToJSON

// TOMLMarshal TOML 序列化。
var TOMLMarshal = toml.Marshal

// TOMLUnmarshal TOML 反序列化。
var TOMLUnmarshal = toml.Unmarshal

// Gzip 压缩。
var Gzip = compress.Gzip

// UnGzip 解压。
var UnGzip = compress.UnGzip

// Zlib 压缩。
var Zlib = compress.Zlib

// UnZlib 解压。
var UnZlib = compress.UnZlib

// HashFnv32 FNV-1a 32 位哈希。
var HashFnv32 = hash.Fnv32String

// HashFnv64 FNV-1a 64 位哈希。
var HashFnv64 = hash.Fnv64String

// HashBKDR BKDR 哈希。
var HashBKDR = hash.BKDR

/* ------------------------------------------------------------------ */
/* 字符串 & 正则门面（tstr / tregex）                                   */
/* ------------------------------------------------------------------ */

// StrIsEmpty 判断字符串是否为空。
var StrIsEmpty = tstr.IsEmpty

// StrIsNumeric 判断字符串是否全为数字。
var StrIsNumeric = tstr.IsNumeric

// StrSubStr 截取子串（支持中文）。
var StrSubStr = tstr.SubStr

// StrLimit 截取指定长度并追加省略号。
var StrLimit = tstr.Limit

// StrToLower 转小写。
var StrToLower = tstr.ToLower

// StrToUpper 转大写。
var StrToUpper = tstr.ToUpper

// StrUcFirst 首字母大写。
var StrUcFirst = tstr.UcFirst

// StrLcFirst 首字母小写。
var StrLcFirst = tstr.LcFirst

// StrSnake 驼峰转蛇形。
var StrSnake = tstr.Snake

// StrCamel 蛇形转驼峰。
var StrCamel = tstr.Camel

// StrLowerCamel 蛇形转小驼峰。
var StrLowerCamel = tstr.LowerCamel

// StrKebab 转短横线。
var StrKebab = tstr.Kebab

// StrLimitRune 按 rune 截断并追加省略号（UTF-8 安全）。
var StrLimitRune = tstr.StrLimitRune

// StrCaseSnakeScreaming 驼峰 → 全大写下划线。
var StrCaseSnakeScreaming = tstr.CaseSnakeScreaming

// StrCaseKebabScreaming 驼峰 → 全大写短横线。
var StrCaseKebabScreaming = tstr.CaseKebabScreaming

// StrTrim 去空白。
var StrTrim = tstr.Trim

// StrPos 查找子串位置。
var StrPos = tstr.Pos

// StrReplace 替换子串。
var StrReplace = tstr.Replace

// StrSplit 分割字符串。
var StrSplit = tstr.Split

// StrJoin 拼接字符串。
var StrJoin = tstr.Join

// StrLen 计算字符数（支持中文）。
var StrLen = tstr.Len

// StrReverse 反转字符串。
var StrReverse = tstr.Reverse

// StrShuffle 随机打乱。
var StrShuffle = tstr.Shuffle

// StrToInt 字符串转整数。
var StrToInt = tstr.ToInt

// RegexMatch 正则匹配。
var RegexMatch = tregex.Match

// RegexReplace 正则替换。
var RegexReplace = tregex.Replace

// RegexSplit 正则分割。
var RegexSplit = tregex.Split

/* ------------------------------------------------------------------ */
/* 类型转换门面（tconv）                                                */
/* ------------------------------------------------------------------ */

// ConvInt 转为 int。
var ConvInt = tconv.Int

// ConvInt64 转为 int64。
var ConvInt64 = tconv.Int64

// ConvUint 转为 uint。
var ConvUint = tconv.Uint

// ConvUint64 转为 uint64。
var ConvUint64 = tconv.Uint64

// ConvFloat64 转为 float64。
var ConvFloat64 = tconv.Float64

// ConvFloat32 转为 float32。
var ConvFloat32 = tconv.Float32

// ConvString 转为 string。
var ConvString = tconv.String

// ConvBool 转为 bool。
var ConvBool = tconv.Bool

// ConvBytes 转为 []byte。
var ConvBytes = tconv.Bytes

// ConvTime 转为 time.Time。
var ConvTime = tconv.Time

// ConvStrings 转为 []string。
var ConvStrings = tconv.Strings

// ConvInts 转为 []int。
var ConvInts = tconv.Ints

// ConvScanOption Map-to-Struct 转换选项类型别名。
type ConvScanOption = tconv.ScanOption

// ConvMapToStruct 将 map/struct 转换为目标结构体。
var ConvMapToStruct = tconv.MapToStruct

// ConvScanStruct 带选项的 Map-to-Struct 转换。
var ConvScanStruct = tconv.ScanStruct

// ConvPtrString 转 *string 指针。
var ConvPtrString = tconv.PtrString

// ConvPtrInt 转 *int 指针。
var ConvPtrInt = tconv.PtrInt

// ConvPtrInt64 转 *int64 指针。
var ConvPtrInt64 = tconv.PtrInt64

// ConvPtrFloat64 转 *float64 指针。
var ConvPtrFloat64 = tconv.PtrFloat64

// ConvPtrBool 转 *bool 指针。
var ConvPtrBool = tconv.PtrBool

// ConvMust 泛型 Must：err 非 nil 时 panic。
func ConvMust[T any](v T, err error) T { return tconv.Must(v, err) }

// ConvUnsafeStrToBytes 零拷贝 string → []byte（只读）。
var ConvUnsafeStrToBytes = tconv.UnsafeStrToBytes

// ConvUnsafeBytesToStr 零拷贝 []byte → string。
var ConvUnsafeBytesToStr = tconv.UnsafeBytesToStr

/* ------------------------------------------------------------------ */
/* 文件 & 时间门面（tfile / ttime）                                     */
/* ------------------------------------------------------------------ */

// FileExists 判断文件/目录是否存在。
var FileExists = tfile.Exists

// FileIsDir 判断是否为目录。
var FileIsDir = tfile.IsDir

// FileIsFile 判断是否为普通文件。
var FileIsFile = tfile.IsFile

// FileRead 读取文件内容为字符串。
var FileRead = tfile.ReadFile

// FileReadBytes 读取文件内容为字节。
var FileReadBytes = tfile.ReadBytes

// FilePut 写入字符串到文件。
var FilePut = tfile.PutContents

// FilePutBytes 写入字节到文件。
var FilePutBytes = tfile.PutBytes

// FileAppend 追加字符串到文件。
var FileAppend = tfile.AppendContents

// FileSize 文件大小。
var FileSize = tfile.Size

// FileMTime 文件修改时间戳。
var FileMTime = tfile.MTime

// FileMkdir 创建目录。
var FileMkdir = tfile.Mkdir

// FileRemove 删除文件。
var FileRemove = tfile.Remove

// FileCopy 复制文件。
var FileCopy = tfile.Copy

// FileRename 重命名/移动文件。
var FileRename = tfile.Rename

// FileGlob 匹配文件。
var FileGlob = tfile.Glob

// FileScanDir 扫描目录。
var FileScanDir = tfile.ScanDir

// FileBaseName 返回文件名。
var FileBaseName = tfile.Basename

// FileDirName 返回目录名。
var FileDirName = tfile.Dirname

// FileExt 返回扩展名。
var FileExt = tfile.Ext

// TimeNow 返回当前时间的 ttime.Time。
var TimeNow = ttime.Now

// TimeUnix 从时间戳创建 ttime.Time。
var TimeUnix = ttime.Unix

// TimeStrTo 字符串转 ttime.Time。
var TimeStrTo = ttime.StrToTime

// Timestamp 返回当前 Unix 时间戳（秒）。
var Timestamp = ttime.Timestamp

// TimestampMilli 返回当前毫秒时间戳。
var TimestampMilli = ttime.TimestampMilli

// Date 格式化当前时间。
var Date = ttime.Date

// Datetime 返回日期时间字符串。
var Datetime = ttime.Datetime

// Today 返回今天日期字符串。
var Today = ttime.Today

/* ------------------------------------------------------------------ */
/* 容器门面（tcontainer）                                               */
/* ------------------------------------------------------------------ */
// 泛型容器类型与函数请直接使用 tcontainer 包，无法以 var 别名。
// 例：arr := tcontainer.NewArray[int](1, 2, 3)
//     m := tcontainer.NewMap[string, int]()
// 类型别名：
type (
	TArray = tcontainer.Array[int] // 占位；实际使用需按需实例化
	TMap   = tcontainer.Map[string, any]
)

/* ------------------------------------------------------------------ */
/* 工具集门面                                                           */
/* ------------------------------------------------------------------ */

// RandInt 返回随机整数。
var RandInt = trand.Int

// RandIntRange 返回区间随机整数。
var RandIntRange = trand.IntRange

// RandString 随机字符串。
var RandString = trand.String

// RandDigits 随机数字串。
var RandDigits = trand.Digits

// UUID V4 UUID 生成。
var UUID = tuuid.V4

// UUIDShort 8 字符短 ID。
var UUIDShort = tuuid.Short

// SID 生成 SID。
var SID = tuuid.SID

// ModeGet 获取运行模式，由 APP_DEBUG 决定：true 为 "dev"，否则（含未设置）为 "prod"。
var ModeGet = func() string {
	if tenv.Get("APP_DEBUG", false) {
		return "dev"
	}
	return "prod"
}

// ModeIsDev 是否为开发模式。
var ModeIsDev = func() bool {
	return tenv.Get("APP_DEBUG", false)
}

// ModeIsProd 是否为生产模式。
var ModeIsProd = func() bool {
	return !tenv.Get("APP_DEBUG", false)
}

// PageNew 创建分页对象。
var PageNew = tpage.NewPage

// DebugStack 获取调用栈。
var DebugStack = tdebug.Stack

// DebugCaller 获取调用者信息。
var DebugCaller = tdebug.Caller

// DebugCallerFunc 获取调用者函数名。
var DebugCallerFunc = tdebug.CallerFunc

// DebugGoroutineID 获取当前 goroutine ID。
var DebugGoroutineID = tdebug.GoroutineID

// TimerNew 创建定时器。
var TimerNew = ttimer.New

// TimerAdd 添加周期任务。
var TimerAdd = ttimer.Add

// TimerAfter 延迟执行。
var TimerAfter = ttimer.After

// TimerGo 异步执行。
var TimerGo = ttimer.Go

// GoSubmit 协程池提交任务。
var GoSubmit = tpool.Submit

// GoWait 协程池等待。
var GoWait = tpool.Wait

// ProcPID 当前进程 ID。
var ProcPID = tproc.PID

// ProcCwd 当前工作目录。
var ProcCwd = tproc.Cwd

// ProcEnv 获取环境变量。
var ProcEnv = tproc.Env

// ProcSetEnv 设置环境变量。
var ProcSetEnv = tproc.SetEnv

// ProcRun 执行外部命令。
var ProcRun = tproc.Run

// ProcShell 通过 shell 执行命令。
var ProcShell = tproc.Shell

// ProcExit 退出进程。
var ProcExit = tproc.Exit

// ResNew 创建资源管理器。
var ResNew = tres.New

// StructsTagMap 返回 tag 映射。
var StructsTagMap = tstructs.TagMap

// StructsFields 返回字段名列表。
var StructsFields = tstructs.Fields

// StructsFieldMap 返回字段值映射。
var StructsFieldMap = tstructs.FieldMap

// 泛型工具函数（tutil.Map/Filter/Reduce/Contains/Unique/Ternary/Default）
// 需带类型参数直接调用，如 tutil.Map(slice, fn)。

// UtilDefault 默认值（泛型函数，需带类型参数使用）。
// 例：tutil.Default(v, def)

/* ------------------------------------------------------------------ */
/* HTTP 客户端门面（tclient）                                           */
/* ------------------------------------------------------------------ */

// HTTPClient 创建 HTTP 客户端别名。
var HTTPClient = tclient.New

// HTTPClientDefault 默认全局 HTTP 客户端。
var HTTPClientDefault = tclient.Default

/* ------------------------------------------------------------------ */
/* 校验门面（tvalid）                                                    */
/* ------------------------------------------------------------------ */

// DataValidator 数据校验器别名（因 tapp.Validator 已被用）。
type DataValidator = tvalid.Validator

// ValidError 校验错误别名。
type ValidError = tvalid.Error

// ValidErrors 校验错误集合别名。
type ValidErrors = tvalid.Errors

// ValidRuleSpec 规则别名。
type ValidRuleSpec = tvalid.RuleSpec

// DataValidatorNew 创建数据校验器。
var DataValidatorNew = tvalid.New

// ValidCheck 使用默认校验器校验 map。
var ValidCheck = tvalid.Check

// ValidCheckStruct 使用默认校验器校验结构体。
var ValidCheckStruct = tvalid.CheckStruct

// ValidCheckStructScene 使用默认校验器按场景校验结构体。
var ValidCheckStructScene = tvalid.CheckStructWithScene

/* ------------------------------------------------------------------ */
/* 管道门面（tpipeline）                                                 */
/* ------------------------------------------------------------------ */

// Pipe 管道阶段接口别名。
type Pipe = tpipeline.Pipe[any]

// PipeFunc 函数式阶段别名。
type PipeFunc = tpipeline.PipeFunc[any]

// PipelineNew 创建管道。
var PipelineSend = tpipeline.Send[any]

/* ------------------------------------------------------------------ */
/* 增强锁门面（tmutex）                                                  */
/* ------------------------------------------------------------------ */

// NamedLock 命名锁（全局实例）。
var NamedLock = tmutex.MLockFunc

// NamedTryLock 命名尝试加锁。
var NamedTryLock = tmutex.MTryLockFunc

/* ------------------------------------------------------------------ */
/* 构建信息门面（tbuild）                                                */
/* ------------------------------------------------------------------ */

// BuildVersion 编译版本号。
var BuildVersion = tbuild.Version

// BuildGitCommit Git 提交哈希。
var BuildGitCommit = tbuild.GitCommit

// BuildTime 编译时间。
var BuildTimeInfo = tbuild.BuildTime

// BuildFullVersion 完整版本字符串。
var BuildFullVersion = tbuild.FullVersion

// BuildShortVersion 简短版本。
var BuildShortVersion = tbuild.ShortVersion

// BuildInfo 构建信息 map。
var BuildInfo = tbuild.Info

/* ------------------------------------------------------------------ */
/* Context 工具门面（tctx）                                              */
/* ------------------------------------------------------------------ */

// CtxWithValue 在 context 中设置值。
var CtxWithValue = tctx.WithValue

// CtxValue 从 context 读取泛型值。
var CtxValue = tctx.Value[any]

// CtxMustValue 从 context 读取值，失败返回零值。
var CtxMustValue = tctx.MustValue[any]

// CtxWithValues 批量设置值。
var CtxWithValues = tctx.WithValues

/* ------------------------------------------------------------------ */
/* 路径搜索门面（tspath）                                                */
/* ------------------------------------------------------------------ */

// PathSearch 在目录列表中搜索文件。
func PathSearch(paths []string, name string) (string, error) {
	return tspath.Search(paths, name)
}

// PathSearchGlob 在目录列表中搜索 glob 匹配的文件。
func PathSearchGlob(paths []string, pattern string) (string, error) {
	return tspath.SearchGlob(paths, pattern)
}

/* ------------------------------------------------------------------ */
/* tstr 新增门面                                                         */
/* ------------------------------------------------------------------ */

// StrHide 隐藏字符串中间部分。
var StrHide = tstr.HideStr

// StrHideEmail 隐藏邮箱地址中间部分。
var StrHideEmail = tstr.HideEmail

// StrAddSlashes 转义特殊字符。
var StrAddSlashes = tstr.AddSlashes

// StrStripSlashes 去除转义。
var StrStripSlashes = tstr.StripSlashes

// StrSimilarText 计算字符串相似度。
var StrSimilarText = tstr.SimilarText

// StrCompareVersion 比较版本号。
var StrCompareVersion = tstr.CompareVersion

// StrRandom 随机字符串。
var StrRandom = tstr.Random

// StrRandomNum 随机数字串。
var StrRandomNum = tstr.RandomNum

// StrRandomLetter 随机字母串。
var StrRandomLetter = tstr.RandomLetter

/* ------------------------------------------------------------------ */
/* tfile 新增门面                                                        */
/* ------------------------------------------------------------------ */

// FileHome 用户主目录。
var FileHome = tfile.Home

// FileHomeDir 用户主目录（含 fallback）。
var FileHomeDir = tfile.HomeDir

// FileFormatSize 字节数格式化。
var FileFormatSize = tfile.FormatSize

// FileReadableSize 文件可读大小。
var FileReadableSize = tfile.ReadableSize

// FileReplaceIn 正则内容替换。
var FileReplaceIn = tfile.ReplaceInFile

// FileReplaceStrIn 字符串内容替换。
var FileReplaceStrIn = tfile.ReplaceStrInFile

// FileSortFiles 文件排序。
var FileSortFiles = tfile.SortFiles

// FileSortByName / FileSortByTime / FileSortBySize 排序方式别名。
var (
	FileSortByName = tfile.SortByName
	FileSortByTime = tfile.SortByTime
	FileSortBySize = tfile.SortBySize
)

/* ------------------------------------------------------------------ */
/* tstructs 新增门面                                                     */
/* ------------------------------------------------------------------ */

// StructsField 字段信息类型别名。
type StructsField = tstructs.Field

// StructsFieldsInput 字段遍历输入类型别名。
type StructsFieldsInput = tstructs.FieldsInput

// StructsParseTag 解析 valid 风格 tag。
var StructsParseTag = tstructs.ParseTag

// StructsParseTagStruct 解析 struct tag 风格字符串。
var StructsParseTagStruct = tstructs.ParseTagStruct

// StructsFieldsInfo 获取字段信息列表。
func StructsFieldsInfo(in tstructs.FieldsInput) []tstructs.Field {
	return tstructs.FieldsInfo(in)
}

// StructsTagMapByName 按 tag 优先级返回 tag值→字段名 映射。
var StructsTagMapByName = tstructs.TagMapByName

// Tag 常量别名。
var (
	TagJson        = tstructs.TagJson
	TagValid       = tstructs.TagValid
	TagTdb         = tstructs.TagTdb
	TagDB          = tstructs.TagDB
	TagDescription = tstructs.TagDescription
	TagDefault     = tstructs.TagDefault
	TagParam       = tstructs.TagParam
	TagExample     = tstructs.TagExample
	TagIn          = tstructs.TagIn
	TagOut         = tstructs.TagOut
	TagSummary     = tstructs.TagSummary
)

/* ------------------------------------------------------------------ */
/* tconsole 新增门面                                                     */
/* ------------------------------------------------------------------ */

// ConsoleArg 参数定义别名。
type ConsoleArg = tconsole.Arg

// ConsoleParser 参数解析器别名。
type ConsoleParser = tconsole.Parser

// ConsoleCommandNode 指令树节点别名。
type ConsoleCommandNode = tconsole.CommandNode

// ConsoleParseArgs 解析命令行参数。
var ConsoleParseArgs = tconsole.ParseArgs

// ConsoleCommandNodeNew 创建树节点。
var ConsoleCommandNodeNew = tconsole.NewCommandNode

// ConsoleRootCommandNew 创建根节点。
var ConsoleRootCommandNew = tconsole.NewRootCommand
