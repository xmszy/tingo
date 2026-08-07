// Package tview 提供轻量、零外部依赖的 HTML 模板渲染引擎。
//
// 设计要点：
//   - 基于标准库 html/template，自动转义，防 XSS；
//   - 支持布局（layout）+ 区块（section）继承；
//   - 视图变量、共享数据（Share）、自定义模板函数；
//   - 模板在首次访问时编译并缓存（sync.Map），避免重复解析；
//   - 不引入任何第三方模板库，保持核心零依赖。
package tview

import (
	"bytes"
	"html/template"
	"maps"
	"path/filepath"
	"strings"
	"sync"
)

// Engine 是模板渲染引擎。
type Engine struct {
	// root 是模板根目录。
	root string
	// ext 是模板文件默认扩展名（含点），默认 ".html"。
	ext string
	// left/right 是模板定界符，默认 "{{" / "}}"。
	left, right string
	// funcs 是全局自定义模板函数。
	funcs template.FuncMap
	// shared 是向所有模板注入的共享变量。
	shared map[string]any
	sm     sync.RWMutex
	// cache 缓存已编译的模板。
	cache sync.Map // map[string]*template.Template
}

// Option 配置 Engine。
type Option func(*Engine)

// WithExt 设置模板扩展名（含点，如 ".gohtml"）。
func WithExt(ext string) Option { return func(e *Engine) { e.ext = ext } }

// WithDelims 设置模板定界符。
func WithDelims(left, right string) Option { return func(e *Engine) { e.left, e.right = left, right } }

// WithFuncs 注册全局模板函数。
func WithFuncs(f template.FuncMap) Option { return func(e *Engine) { maps.Copy(e.funcs, f) } }

// New 在 root 目录下创建一个引擎。
func New(root string, opts ...Option) *Engine {
	e := &Engine{
		root:   root,
		ext:    ".html",
		left:   "{{",
		right:  "}}",
		funcs:  template.FuncMap{},
		shared: map[string]any{},
	}
	for _, o := range opts {
		o(e)
	}
	e.registerBuiltins()
	return e
}

// registerBuiltins 注册内置模板函数。
func (e *Engine) registerBuiltins() {
	e.funcs["raw"] = func(s any) template.HTML {
		switch v := s.(type) {
		case template.HTML:
			return v
		case string:
			return template.HTML(v)
		default:
			return template.HTML(toStr(s))
		}
	}
	e.funcs["hasPrefix"] = strings.HasPrefix
	e.funcs["hasSuffix"] = strings.HasSuffix
	e.funcs["contains"] = strings.Contains
	e.funcs["replace"] = func(s, old, new string) string { return strings.ReplaceAll(s, old, new) }
	e.funcs["upper"] = strings.ToUpper
	e.funcs["lower"] = strings.ToLower
	e.funcs["title"] = titleCase
	e.funcs["trim"] = strings.TrimSpace
	e.funcs["default"] = func(def, v any) any {
		if isEmpty(v) {
			return def
		}
		return v
	}
	e.funcs["join"] = func(sep string, items []string) string { return strings.Join(items, sep) }
}

// Share 向所有模板注入共享变量（线程安全，可重复调用）。
func (e *Engine) Share(key string, value any) *Engine {
	e.sm.Lock()
	e.shared[key] = value
	e.sm.Unlock()
	return e
}

// Funcs 注册额外的模板函数（链式）。
func (e *Engine) Funcs(f template.FuncMap) *Engine {
	maps.Copy(e.funcs, f)
	return e
}

// pathOf 将视图名解析为磁盘上的完整路径。
// 视图名可以是 "user/profile" 或 "user/profile.html"，统一加扩展名。
func (e *Engine) pathOf(name string) string {
	if strings.HasSuffix(name, e.ext) {
		return filepath.Join(e.root, name)
	}
	return filepath.Join(e.root, name+e.ext)
}

// compile 编译单个模板并缓存。布局模式请使用 RenderIn。
func (e *Engine) compile(name string) (*template.Template, error) {
	path := e.pathOf(name)
	t := template.New(filepath.Base(path)).
		Delims(e.left, e.right).
		Funcs(e.funcs)
	t, err := t.ParseFiles(path)
	if err != nil {
		return nil, err
	}
	e.cache.Store(name, t)
	return t, nil
}

// Render 渲染指定视图名，返回 HTML 字符串。
func (e *Engine) Render(name string, data any) (string, error) {
	tv, err := e.lookup(name)
	if err != nil {
		return "", err
	}
	// 合并共享数据。
	merged := e.mergeData(data)
	var buf bytes.Buffer
	base := filepath.Base(e.pathOf(name))
	if err := tv.ExecuteTemplate(&buf, base, merged); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderIn 在布局模板 layout 中渲染子模板 name（子模板内容通过 content 变量注入）。
func (e *Engine) RenderIn(layout, name string, data any) (string, error) {
	lt, err := e.lookup(layout)
	if err != nil {
		return "", err
	}
	ct, err := e.lookup(name)
	if err != nil {
		return "", err
	}
	merged := e.mergeData(data)
	var childBuf bytes.Buffer
	if err := ct.Execute(&childBuf, merged); err != nil {
		return "", err
	}
	merged = withContent(merged, childBuf.String())
	var buf bytes.Buffer
	if err := lt.Execute(&buf, merged); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MustRender 同 Render，失败 panic（启动期或模板均由测试保证存在时使用）。
func (e *Engine) MustRender(name string, data any) string {
	s, err := e.Render(name, data)
	if err != nil {
		panic(err)
	}
	return s
}

// lookup 从缓存或重新编译取得模板。
func (e *Engine) lookup(name string) (*template.Template, error) {
	if v, ok := e.cache.Load(name); ok {
		return v.(*template.Template), nil
	}
	return e.compile(name)
}

// mergeData 将共享变量合并进用户数据。
// 用户数据优先；若用户数据是 map，则直接写入；否则包裹为 {"data": data}。
func (e *Engine) mergeData(data any) any {
	e.sm.RLock()
	defer e.sm.RUnlock()
	if len(e.shared) == 0 {
		return data
	}
	if m, ok := data.(map[string]any); ok {
		for k, v := range e.shared {
			if _, exists := m[k]; !exists {
				m[k] = v
			}
		}
		return m
	}
	out := map[string]any{}
	maps.Copy(out, e.shared)
	if data != nil {
		out["data"] = data
	}
	return out
}

// ClearCache 清空编译缓存（热重载场景调用）。
func (e *Engine) ClearCache() { e.cache.Range(func(k, _ any) bool { e.cache.Delete(k); return true }) }
