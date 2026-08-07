package tapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/xmszy/tingo/core"
)

/* ------------------------------------------------------------------ */
/* 请求对象扩展                                                          */
/* ------------------------------------------------------------------ */

// Request 是请求读取器，提供统一取值语义。
//
// 它是 *core.Ctx 的零成本视图，不复制任何请求数据，可放心在栈上创建。
//
// 自动检测 JSON 请求体：当 Content-Type 包含 application/json 时，
// Param/Post/All 等方法会自动从 JSON body 取值。
//
//	req := tapp.Req(c)
//	page := req.Int("page", 1)
//	kw := req.Param("keyword")
type Request struct {
	c          *core.Ctx
	jsonData   map[string]any // 懒解析的 JSON body 缓存
	jsonParsed bool           // 是否已尝试解析 JSON body
}

// Req 从上下文创建请求读取器。
//
//go:inline
func Req(c *core.Ctx) Request { return Request{c: c} }

// Ctx 返回底层上下文。
func (r Request) Ctx() *core.Ctx { return r.c }

// parseJSONBody 懒解析 JSON 请求体（仅首次调用时解析）。
//
// 检测 Content-Type 中是否包含 json，若是则用 json.Unmarshal 解析。
func (r *Request) parseJSONBody() {
	if r.jsonParsed {
		return
	}
	r.jsonParsed = true

	ct := r.c.ContentType()
	if !strings.Contains(ct, "json") {
		return
	}

	body, err := r.c.Body()
	if err != nil {
		return
	}
	if len(body) == 0 {
		return
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return
	}
	r.jsonData = data
}

// bodyValue 从请求体中取值：优先 JSON body，其次表单。
func (r *Request) bodyValue(key string) (string, bool) {
	r.parseJSONBody()
	if r.jsonData != nil {
		if v, ok := r.jsonData[key]; ok {
			return fmt.Sprint(v), true
		}
	}
	return r.c.G().GetPostForm(key)
}

/* ------------------------------------------------------------------ */
/* 取值：param / get / post / route                                     */
/* ------------------------------------------------------------------ */

// Param 按优先级取值：路由参数 < query < 请求体，后者覆盖前者。
//
// 支持 JSON 请求体：当 Content-Type 包含 json 时自动解析 JSON。
func (r Request) Param(key string, def ...string) string {
	// 先检查路由参数
	if v := r.c.Param(key); v != "" {
		if bv, ok := r.bodyValue(key); ok {
			return bv // 请求体覆盖路由参数
		}
		return v
	}
	// 再检查 query 参数
	if v, ok := r.c.G().GetQuery(key); ok {
		if bv, ok := r.bodyValue(key); ok {
			return bv // 请求体覆盖 query 参数
		}
		return v
	}
	// 最后尝试请求体
	if bv, ok := r.bodyValue(key); ok {
		return bv
	}
	return first(def)
}

// Get 取 query 参数。
func (r Request) Get(key string, def ...string) string {
	if v, ok := r.c.G().GetQuery(key); ok {
		return v
	}
	return first(def)
}

// Post 取请求体字段（支持 JSON body + 表单）。
//
// 优先级：JSON body > 表单。当 Content-Type 包含 json 时自动解析 JSON。
func (r Request) Post(key string, def ...string) string {
	if bv, ok := r.bodyValue(key); ok {
		return bv
	}
	return first(def)
}

// Route 取路由参数。
func (r Request) Route(key string, def ...string) string {
	if v := r.c.Param(key); v != "" {
		return v
	}
	return first(def)
}

// Header 取请求头。
func (r Request) Header(key string, def ...string) string {
	if v := r.c.Header(key); v != "" {
		return v
	}
	return first(def)
}

// Cookie 取 cookie。
func (r Request) Cookie(key string, def ...string) string {
	if v := r.c.Cookie(key); v != "" {
		return v
	}
	return first(def)
}

// Has 判断参数是否存在。
func (r Request) Has(key string) bool {
	if r.c.Param(key) != "" {
		return true
	}
	if _, ok := r.c.G().GetQuery(key); ok {
		return true
	}
	_, ok := r.bodyValue(key)
	return ok
}

/* ------------------------------------------------------------------ */
/* 类型转换取值                                                          */
/* ------------------------------------------------------------------ */

// Int 取整型参数。
func (r Request) Int(key string, def ...int) int {
	s := r.Param(key)
	if s == "" {
		return firstInt(def)
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return firstInt(def)
	}
	return v
}

// Int64 取 int64 参数。
func (r Request) Int64(key string, def ...int64) int64 {
	s := r.Param(key)
	if s == "" {
		return firstOf(def)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return firstOf(def)
	}
	return v
}

// Float 取浮点参数。
func (r Request) Float(key string, def ...float64) float64 {
	s := r.Param(key)
	if s == "" {
		return firstOf(def)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return firstOf(def)
	}
	return v
}

// Bool 取布尔参数。
// 支持 1/true/on/yes 等常见真值写法。
func (r Request) Bool(key string, def ...bool) bool {
	s := strings.ToLower(strings.TrimSpace(r.Param(key)))
	switch s {
	case "1", "true", "on", "yes", "y":
		return true
	case "0", "false", "off", "no", "n":
		return false
	}
	return firstOf(def)
}

// Strings 取同名多值参数。
//
// 支持 JSON body 数组字段：若 JSON body 中该字段为 []any，
// 则转换为 []string 返回。
func (r Request) Strings(key string) []string {
	if v := r.c.QueryArray(key); len(v) > 0 {
		return v
	}
	// 尝试 JSON body 数组
	r.parseJSONBody()
	if r.jsonData != nil {
		if arr, ok := r.jsonData[key].([]any); ok {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				out = append(out, fmt.Sprint(item))
			}
			return out
		}
	}
	return r.c.PostArray(key)
}

/* ------------------------------------------------------------------ */
/* 批量取值：only / exclude / all                                        */
/* ------------------------------------------------------------------ */

// All 返回全部参数，优先级同 Param。
//
// 包含：路由参数、query、请求体（JSON body 或表单），
// 后者覆盖前者。支持 JSON 请求体自动解析。
//
// 注意：该方法会分配 map，仅适合非热点路径（如后台管理、调试接口）。
// 热点路径请使用结构体绑定（Controller.Bind）以保持零分配。
func (r Request) All() map[string]string {
	out := make(map[string]string, 8)
	// 1. 路由参数（最低优先级）
	for _, p := range r.c.Params {
		out[p.Key] = p.Value
	}
	// 2. query 参数
	for k, v := range r.c.G().Request.URL.Query() {
		if len(v) > 0 {
			out[k] = v[len(v)-1]
		}
	}
	// 3. 表单参数
	if err := r.c.G().Request.ParseForm(); err == nil {
		for k, v := range r.c.G().Request.PostForm {
			if len(v) > 0 {
				out[k] = v[len(v)-1]
			}
		}
	}
	// 4. JSON body（最高优先级，覆盖同名字段）
	r.parseJSONBody()
	if r.jsonData != nil {
		for k, v := range r.jsonData {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

// Only 仅返回指定字段。
func (r Request) Only(keys ...string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v := r.Param(k); v != "" {
			out[k] = v
		}
	}
	return out
}

// Exclude 返回排除指定字段后的全部参数。
func (r Request) Exclude(keys ...string) map[string]string {
	all := r.All()
	for _, k := range keys {
		delete(all, k)
	}
	return all
}

// Input 从指定数据中取值。
//
//	data 是取值的数据源；nil 表示自动从 All() 取值。
//	key 是字段名；"" 表示返回默认值。
//
//	// 从 All() 中取值
//	name := req.Input(req.All(), "name")
//	// 自动使用 All() 作为数据源
//	name := req.Input(nil, "name")
//	// 嵌套取值请使用 JSON() 绑定结构体
func (r Request) Input(data map[string]string, key string, def ...string) string {
	if data == nil {
		data = r.All()
	}
	if key == "" {
		return first(def)
	}
	if v, ok := data[key]; ok {
		return v
	}
	return first(def)
}

// Body 返回请求体的原始字节。
//
// 首次读取后会自动还原到 Request.Body，支持重复读取。
func (r Request) Body() ([]byte, error) {
	return r.c.Body()
}

// JSON 将 JSON 请求体解析到目标结构。
//
//	req := tapp.Req(c)
//	var user struct{ Name string `json:"name"` }
//	if err := req.JSON(&user); err != nil { ... }
func (r Request) JSON(v any) error {
	body, err := r.c.Body()
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

/* ------------------------------------------------------------------ */
/* 请求特征                                                              */
/* ------------------------------------------------------------------ */

// Method 返回当前请求方法（GET/POST/PUT/DELETE 等）。
func (r Request) Method() string { return r.c.Method() }

// IsGet 判断是否 GET 请求。
func (r Request) IsGet() bool { return r.c.Method() == "GET" }

// IsPost 判断是否 POST 请求。
func (r Request) IsPost() bool { return r.c.Method() == "POST" }

// IsPut 判断是否 PUT 请求。
func (r Request) IsPut() bool { return r.c.Method() == "PUT" }

// IsDelete 判断是否 DELETE 请求。
func (r Request) IsDelete() bool { return r.c.Method() == "DELETE" }

// IsPatch 判断是否 PATCH 请求。
func (r Request) IsPatch() bool { return r.c.Method() == "PATCH" }

// IsOptions 判断是否 OPTIONS 请求。
func (r Request) IsOptions() bool { return r.c.Method() == "OPTIONS" }

// IsHead 判断是否 HEAD 请求。
func (r Request) IsHead() bool { return r.c.Method() == "HEAD" }

// IsAjax 判断是否 Ajax 请求（X-Requested-With: XMLHttpRequest）。
func (r Request) IsAjax() bool { return r.c.IsAjax() }

// IsPjax 判断是否 PJAX 请求（X-PJAX 头）。
func (r Request) IsPjax() bool { return r.c.Header("X-PJAX") != "" || r.c.Header("X-PJAX-Container") != "" }

// IsJson 判断是否 JSON 请求（Accept 头或 Content-Type 包含 json）。
func (r Request) IsJson() bool {
	ct := r.ContentType()
	if strings.Contains(ct, "json") {
		return true
	}
	accept := r.c.Header("Accept")
	return strings.Contains(accept, "json")
}

// IsSSL 判断是否 HTTPS 请求。
func (r Request) IsSSL() bool { return r.c.Scheme() == "https" }

// IP 返回客户端 IP。
func (r Request) IP() string { return r.c.IP() }

// URL 返回请求路径（不含 query string）。
func (r Request) URL() string { return r.c.Path() }

// FullUrl 返回完整 URL（含 scheme + host + path + query string）。
func (r Request) FullUrl() string {
	u := r.c.Scheme() + "://" + r.Host() + r.c.Path()
	if raw := r.c.G().Request.URL.RawQuery; raw != "" {
		u += "?" + raw
	}
	return u
}

// BaseUrl 返回基础 URL（scheme + host）。
func (r Request) BaseUrl() string {
	return r.c.Scheme() + "://" + r.Host()
}

// Root 返回根路径（前缀）。
func (r Request) Root() string {
	return r.c.G().Request.URL.Path
}

// Host 返回请求的主机名（不含端口）。
func (r Request) Host() string {
	host := r.c.G().Request.Host
	if i := strings.Index(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}

// Port 返回请求端口。
func (r Request) Port() string {
	host := r.c.G().Request.Host
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[i+1:]
	}
	if r.IsSSL() {
		return "443"
	}
	return "80"
}

// Scheme 返回请求协议（http/https）。
func (r Request) Scheme() string { return r.c.Scheme() }

// Protocol 返回协议版本（如 HTTP/1.1）。
func (r Request) Protocol() string { return r.c.G().Request.Proto }

// Path 返回请求路径（同 URL()）。
func (r Request) Path() string { return r.c.Path() }

// ContentType 返回请求的 Content-Type。
func (r Request) ContentType() string { return r.c.ContentType() }

// Accept 返回 Accept 请求头内容。
func (r Request) Accept() string { return r.c.Header("Accept") }

// BearerToken 从 Authorization: Bearer xxx 中提取令牌。
func (r Request) BearerToken() string {
	auth := r.c.Header("Authorization")
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return auth[len(prefix):]
	}
	return ""
}

// Server 获取服务器环境变量（header/gin元数据）。
// key 为 HTTP_ 前缀的服务器变量名，如 "HTTP_HOST"、"SERVER_PORT"。
func (r Request) Server(key string, def ...string) string {
	// 尝试从请求头取
	if v := r.c.Header(key); v != "" {
		return v
	}
	// 特殊处理常见变量
	switch strings.ToUpper(key) {
	case "SERVER_PORT":
		return r.Port()
	case "HTTP_HOST", "SERVER_NAME":
		return r.Host()
	case "REQUEST_METHOD":
		return r.Method()
	case "REQUEST_URI":
		return r.c.G().Request.RequestURI
	case "QUERY_STRING":
		return r.c.G().Request.URL.RawQuery
	case "REMOTE_ADDR":
		return r.IP()
	case "HTTPS":
		if r.IsSSL() {
			return "on"
		}
		return ""
	}
	return first(def)
}

// Controller 返回当前控制器名。
func (r Request) Controller() string { return r.c.Controller() }

// Action 返回当前动作名。
func (r Request) Action() string { return r.c.Action() }

// App 返回当前应用名。
func (r Request) App() string { return r.c.App() }

/* ------------------------------------------------------------------ */
/* 内部助手                                                              */
/* ------------------------------------------------------------------ */

func first(def []string) string {
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

func firstInt(def []int) int {
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func firstOf[T any](def []T) T {
	if len(def) > 0 {
		return def[0]
	}
	var zero T
	return zero
}
