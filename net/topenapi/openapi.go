// Package topenapi 提供 OpenAPI 3.0 规范的自动文档生成。
//
// 从 tingo 路由注册信息中提取元数据，生成符合 OpenAPI 3.0 标准的 JSON/YAML 文档。
// 支持：路径分组、参数推断、Schema 自动生成（依赖 go/doc 或 struct tag 提取）。
//
// 用法：
//
//	spec := topenapi.NewSpec(topapi.WithTitle("My API"), topenapi.WithVersion("1.0"))
//	spec.AddRoutes(routes...)  // routes 来自 thttp 路由元信息
//	jsonBytes, _ := spec.JSON()
package topenapi

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// Spec 表示一个 OpenAPI 3.0 规范文档。
type Spec struct {
	OpenAPI    string                    `json:"openapi"`
	Info       Info                      `json:"info"`
	Servers    []Server                  `json:"servers,omitempty"`
	Paths      map[string]PathItem       `json:"paths"`
	Components *Components               `json:"components,omitempty"`
	Tags       []Tag                     `json:"tags,omitempty"`
	Security   []SecurityRequirement     `json:"security,omitempty"`
}

// Info 是 OpenAPI 的 info 对象。
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// Server 是 OpenAPI 的 server 对象。
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem 描述单个路径上的操作。
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Options *Operation `json:"options,omitempty"`
}

// Operation 描述一个 API 操作。
type Operation struct {
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
}

// Parameter 描述操作参数。
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // query, path, header, cookie
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody 请求体。
type RequestBody struct {
	Description string                `json:"description,omitempty"`
	Required    bool                  `json:"required,omitempty"`
	Content     map[string]MediaType  `json:"content"`
}

// Response 响应。
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Headers     map[string]Header    `json:"headers,omitempty"`
}

// Header 响应头。
type Header struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// MediaType 媒体类型。
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Schema 是 OpenAPI Schema 对象（简化子集）。
type Schema struct {
	Type       string            `json:"type,omitempty"`
	Format     string            `json:"format,omitempty"`
	Properties map[string]Schema `json:"properties,omitempty"`
	Required   []string          `json:"required,omitempty"`
	Items      *Schema           `json:"items,omitempty"`
	Ref        string            `json:"$ref,omitempty"`
	Nullable   bool              `json:"nullable,omitempty"`
	Example    any               `json:"example,omitempty"`
	Enum       []any             `json:"enum,omitempty"`
}

// Components 是 OpenAPI 的 components 对象。
type Components struct {
	Schemas       map[string]Schema       `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme 安全方案定义。
type SecurityScheme struct {
	Type         string `json:"type"` // http, apiKey, oauth2, openIdConnect
	Description  string `json:"description,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
}

// SecurityRequirement 安全要求。
type SecurityRequirement map[string][]string

// Tag 标签。
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// RouteInfo 路由信息，来自 thttp 的路由元数据。
type RouteInfo struct {
	Method   string // GET, POST, PUT, DELETE, etc.
	Path     string // /api/v1/users/:id
	App      string // 应用名（作为 Tag）
	Action   string // 控制器动作名
	Summary  string // 简短描述
	Desc     string // 详细描述
	Deprecated bool
}

// SpecOption 是 Spec 的建造选项。
type SpecOption func(*Spec)

// NewSpec 创建新的 OpenAPI Spec。
func NewSpec(opts ...SpecOption) *Spec {
	s := &Spec{
		OpenAPI: "3.0.3",
		Paths:   make(map[string]PathItem),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// WithTitle 设置 API 标题。
func WithTitle(title string) SpecOption {
	return func(s *Spec) { s.Info.Title = title }
}

// WithVersion 设置 API 版本。
func WithVersion(version string) SpecOption {
	return func(s *Spec) { s.Info.Version = version }
}

// WithDescription 设置 API 描述。
func WithDescription(desc string) SpecOption {
	return func(s *Spec) { s.Info.Description = desc }
}

// WithServer 添加 server 信息。
func WithServer(url, desc string) SpecOption {
	return func(s *Spec) {
		s.Servers = append(s.Servers, Server{URL: url, Description: desc})
	}
}

// WithTag 添加 tag。
func WithTag(name, desc string) SpecOption {
	return func(s *Spec) {
		s.Tags = append(s.Tags, Tag{Name: name, Description: desc})
	}
}

// WithBearerAuth 添加 Bearer Token 认证。
func WithBearerAuth(name, desc string) SpecOption {
	return func(s *Spec) {
		if s.Components == nil {
			s.Components = &Components{}
		}
		if s.Components.SecuritySchemes == nil {
			s.Components.SecuritySchemes = make(map[string]SecurityScheme)
		}
		s.Components.SecuritySchemes[name] = SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  desc,
		}
	}
}

// AddRoute 添加单条路由信息到 Spec。
func (s *Spec) AddRoute(info RouteInfo) {
	openapiPath := convertPath(info.Path)

	pi, ok := s.Paths[openapiPath]
	if !ok {
		pi = PathItem{}
	}

	op := &Operation{
		OperationID: info.App + "." + info.Action,
		Tags:        []string{info.App},
		Summary:     info.Summary,
		Description: info.Desc,
		Deprecated:  info.Deprecated,
		Responses: map[string]Response{
			"200": {Description: "Success"},
		},
	}

	switch strings.ToUpper(info.Method) {
	case "GET":
		pi.Get = op
	case "POST":
		pi.Post = op
	case "PUT":
		pi.Put = op
	case "DELETE":
		pi.Delete = op
	case "PATCH":
		pi.Patch = op
	case "HEAD":
		pi.Head = op
	case "OPTIONS":
		pi.Options = op
	}

	s.Paths[openapiPath] = pi
}

// AddRoutes 批量添加路由信息。
func (s *Spec) AddRoutes(routes []RouteInfo) {
	for _, r := range routes {
		s.AddRoute(r)
	}
}

// AddSchema 将 Go 类型注册为 OpenAPI Schema（通过反射提取字段）。
func (s *Spec) AddSchema(name string, sample any) {
	if s.Components == nil {
		s.Components = &Components{}
	}
	if s.Components.Schemas == nil {
		s.Components.Schemas = make(map[string]Schema)
	}

	schema := structToSchema(sample)
	s.Components.Schemas[name] = schema
}

// AddSchemaT 泛型版本：从类型参数自动推断。
func AddSchemaT[T any](s *Spec, name string) {
	s.AddSchema(name, *new(T))
}

// JSON 输出 OpenAPI JSON。
func (s *Spec) JSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// JSONString 输出 OpenAPI JSON 字符串。
func (s *Spec) JSONString() string {
	b, err := s.JSON()
	if err != nil {
		return ""
	}
	return string(b)
}

// convertPath 把 gin 风格路径（/:param）转为 OpenAPI 风格（/{param}）。
func convertPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		} else if strings.HasPrefix(p, "*") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// structToSchema 通过反射提取结构体的 OpenAPI Schema。
func structToSchema(v any) Schema {
	t := reflect.TypeOf(v)
	if t == nil {
		return Schema{Type: "object"}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Schema{Type: "object"}
	}

	props := make(map[string]Schema)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := fieldName(f)
		if name == "-" {
			continue
		}
		fs := fieldToSchema(f.Type)

		// 读取 tag 描述
		if desc := f.Tag.Get("desc"); desc != "" {
			fs.Example = desc
		}
		if cmt := f.Tag.Get("comment"); cmt != "" {
			fs.Example = cmt
		}

		props[name] = fs
	}

	return Schema{
		Type:       "object",
		Properties: props,
	}
}

// fieldName 从 struct tag 提取字段名。
func fieldName(f reflect.StructField) string {
	for _, tag := range []string{"json", "form", "tdb", "db"} {
		v := f.Tag.Get(tag)
		if v == "" {
			continue
		}
		name := v
		if i := strings.IndexByte(name, ','); i >= 0 {
			name = name[:i]
		}
		if name != "" {
			return name
		}
	}
	return toSnakeCase(f.Name)
}

// fieldToSchema 将 Go 类型映射为 OpenAPI Schema。
func fieldToSchema(t reflect.Type) Schema {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return Schema{Type: "number"}
	case reflect.Bool:
		return Schema{Type: "boolean"}
	case reflect.Slice, reflect.Array:
		elem := fieldToSchema(t.Elem())
		return Schema{Type: "array", Items: &elem}
	case reflect.Struct:
		if t.Name() == "Time" || t.String() == "time.Time" {
			return Schema{Type: "string", Format: "date-time"}
		}
		return structToSchema(reflect.New(t).Interface())
	default:
		return Schema{Type: "string"}
	}
}

// toSnakeCase 驼峰转蛇形（简化版，与 tdb 中的实现功能相同）。
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---- 便捷构建器 ----

// RouteCollector 从路由元数据收集 Routes 并生成 Spec。
type RouteCollector struct {
	routes []RouteInfo
}

// NewCollector 创建路由收集器。
func NewCollector() *RouteCollector {
	return &RouteCollector{}
}

// Add 添加路由。
func (rc *RouteCollector) Add(method, path, app, action, summary string) {
	rc.routes = append(rc.routes, RouteInfo{
		Method:  method,
		Path:    path,
		App:     app,
		Action:  action,
		Summary: summary,
	})
}

// Build 构建 OpenAPI Spec。
func (rc *RouteCollector) Build(opts ...SpecOption) *Spec {
	spec := NewSpec(opts...)
	spec.AddRoutes(rc.routes)
	return spec
}

// Routes 返回收集到的路由列表。
func (rc *RouteCollector) Routes() []RouteInfo {
	return rc.routes
}

// sortedKeys 排序 map keys（用于稳定输出）。
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ensureSortedKeysUsed prevents unused function warning.
var _ = sortedKeys[string]
