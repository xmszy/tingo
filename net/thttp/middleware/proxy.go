package middleware

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
)

// proxyCtxKey 用于在请求 context 中透传 *core.Ctx 给 ReverseProxy.ErrorHandler。
const proxyCtxKey = "__tingo_proxy_ctx__"

// ProxyConfig 是反向代理中间件的配置。
type ProxyConfig struct {
	// Target 是上游服务地址，如 "http://127.0.0.1:9000" 或 "https://api.example.com"。
	Target string
	// Rewrite 可选：在转发前改写请求路径（如去掉前缀 /api）。
	// 形如 func(path string) string，返回改写后的路径。
	Rewrite func(path string) string
	// Transport 可选：自定义 http.Transport（如 TLS 配置、超时）。
	Transport http.RoundTripper
	// ModifyResponse 可选：在收到上游响应后、写回客户端前做处理。
	ModifyResponse func(*http.Response) error
	// ErrorHandler 可选：上游出错时的处理（默认写 502）。
	ErrorHandler func(c *core.Ctx, err error)
}

// Proxy 返回一个反向代理中间件，将匹配到的请求转发到 Target。
//
// 用法：
//
//	r := tingo.NewEngine()
//	r.Use(middleware.Proxy(
//	    middleware.ProxyConfig{Target: "http://127.0.0.1:9000"},
//	))
//	// 或仅对特定路由组生效：
//	api := r.Group("/api")
//	api.Use(middleware.Proxy(middleware.ProxyConfig{
//	    Target: "http://127.0.0.1:9000",
//	    Rewrite: func(p string) string { return strings.TrimPrefix(p, "/api") },
//	}))
func Proxy(opts ProxyConfig) core.Handler {
	target, err := url.Parse(opts.Target)
	if err != nil {
		panic("middleware.Proxy: invalid Target: " + err.Error())
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			if opts.Rewrite != nil {
				req.URL.Path = opts.Rewrite(req.URL.Path)
			}
			// 若重写后路径为空，回退到根路径。
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		},
		Transport: opts.Transport,
	}
	if opts.ModifyResponse != nil {
		proxy.ModifyResponse = opts.ModifyResponse
	}
	// 默认错误处理：返回 502。
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		if opts.ErrorHandler != nil {
			if ctx, ok := r.Context().Value(proxyCtxKey).(*core.Ctx); ok && ctx != nil {
				opts.ErrorHandler(ctx, e)
				return
			}
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway: " + e.Error()))
	}

	return func(c *core.Ctx) {
		g := c.G()
		// 将 *core.Ctx 透传到请求 context，供 ErrorHandler 取回。
		g.Request = g.Request.WithContext(contextWithProxyCtx(g, c))
		proxy.ServeHTTP(g.Writer, g.Request)
	}
}

// contextWithProxyCtx 返回一个包含 *core.Ctx 的请求 context（委托 gin.Context 的 Value）。
func contextWithProxyCtx(parent *gin.Context, c *core.Ctx) *ginProxyCtx {
	return &ginProxyCtx{parent: parent, ctx: c}
}

// ginProxyCtx 让 *gin.Context 既作为 context.Context，又能取回 *core.Ctx。
type ginProxyCtx struct {
	parent *gin.Context
	ctx    *core.Ctx
}

func (x *ginProxyCtx) Deadline() (deadline time.Time, ok bool)        { return x.parent.Deadline() }
func (x *ginProxyCtx) Done() <-chan struct{}                          { return x.parent.Done() }
func (x *ginProxyCtx) Err() error                                     { return x.parent.Err() }
func (x *ginProxyCtx) Value(key any) any {
	if key == proxyCtxKey {
		return x.ctx
	}
	return x.parent.Value(key)
}
