// Package tpage 提供分页管理。
// 设计要点：
//   - 基于标准库，零外部依赖。
//   - 提供分页对象和 HTML 分页生成。
//   - 支持保留 query string、fragment 锚点、simple 模式。
package tpage

import (
	"fmt"
	"net/url"
	"strings"
)

// Page 分页对象。
type Page struct {
	Total    int        // 总记录数
	Size     int        // 每页条数
	Current  int        // 当前页码（从 1 开始）
	Query    url.Values // 额外的查询参数（自动拼接到分页链接）
	Fragment string     // URL 锚点
	Simple   bool       // 是否仅显示上一页/下一页（simple 模式）
	// PageName URL 中的页码参数名，默认 "page"
	PageName string
}

// NewPage 创建分页对象。
func NewPage(total, size, current int) *Page {
	if size <= 0 {
		size = 10
	}
	if current <= 0 {
		current = 1
	}
	return &Page{Total: total, Size: size, Current: current, PageName: "page"}
}

// TotalPages 总页数。
func (p *Page) TotalPages() int {
	if p.Size == 0 {
		return 1
	}
	pages := p.Total / p.Size
	if p.Total%p.Size > 0 {
		pages++
	}
	if pages == 0 {
		pages = 1
	}
	return pages
}

// Offset 返回 SQL offset。
func (p *Page) Offset() int { return (p.Current - 1) * p.Size }

// HasPrev 是否有上一页。
func (p *Page) HasPrev() bool { return p.Current > 1 }

// HasNext 是否有下一页。
func (p *Page) HasNext() bool { return p.Current < p.TotalPages() }

// PrevPage 上一页页码。
func (p *Page) PrevPage() int {
	if p.HasPrev() {
		return p.Current - 1
	}
	return 1
}

// NextPage 下一页页码。
func (p *Page) NextPage() int {
	if p.HasNext() {
		return p.Current + 1
	}
	return p.TotalPages()
}

// Range 生成页码范围 [start, end]。
func (p *Page) Range(count int) []int {
	total := p.TotalPages()
	half := count / 2
	start := p.Current - half
	end := p.Current + half
	if start < 1 {
		end += 1 - start
		start = 1
	}
	if end > total {
		start -= end - total
		end = total
	}
	if start < 1 {
		start = 1
	}
	result := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		result = append(result, i)
	}
	return result
}

// Appends 添加额外的查询参数（如搜索结果过滤条件）。
func (p *Page) Appends(params map[string]string) *Page {
	if p.Query == nil {
		p.Query = make(url.Values)
	}
	for k, v := range params {
		p.Query.Set(k, v)
	}
	return p
}

// SetFragment 设置 URL 锚点。
func (p *Page) SetFragment(fragment string) *Page {
	p.Fragment = fragment
	return p
}

// pageName 返回页码参数名。
func (p *Page) pageName() string {
	if p.PageName == "" {
		return "page"
	}
	return p.PageName
}

// buildPageURL 构造单页 URL，保留所有已有查询参数。
func (p *Page) buildPageURL(baseURL string, page int) string {
	var b strings.Builder
	b.WriteString(baseURL)

	// 从 baseURL 中剥离已有的 fragment 和 query
	base, existingQuery, existingFrag := parseURL(baseURL)

	// 重新写入 path
	b.Reset()
	b.WriteString(base)

	// 合并参数
	params := make(url.Values)
	// 先写入 baseURL 自带的参数
	for k, vs := range existingQuery {
		for _, v := range vs {
			params.Add(k, v)
		}
	}
	// 再写入 Page.Appends 添加的参数
	for k, vs := range p.Query {
		for _, v := range vs {
			params.Set(k, v)
		}
	}
	// 最后写入页码（覆盖可能的同名参数）
	params.Set(p.pageName(), fmt.Sprintf("%d", page))

	if len(params) > 0 {
		b.WriteByte('?')
		b.WriteString(params.Encode())
	}

	fragment := p.Fragment
	if fragment == "" {
		fragment = existingFrag
	}
	if fragment != "" {
		b.WriteByte('#')
		b.WriteString(fragment)
	}
	return b.String()
}

// parseURL 解析 URL 为基础路径、查询参数和 fragment（纯字符串操作，无需 net/url）。
func parseURL(raw string) (base string, query url.Values, fragment string) {
	s := raw
	// 提取 fragment
	if idx := strings.LastIndexByte(s, '#'); idx >= 0 {
		fragment = s[idx+1:]
		s = s[:idx]
	}
	// 提取 query
	if idx := strings.IndexByte(s, '?'); idx >= 0 {
		qStr := s[idx+1:]
		base = s[:idx]
		var err error
		query, err = url.ParseQuery(qStr)
		if err != nil {
			query = make(url.Values)
		}
	} else {
		base = s
		query = make(url.Values)
	}
	return
}

// ──────────────── HTML 分页 ────────────────

// HTML 生成简单的分页 HTML。url 为基础路径，查询参数通过 Appends() 设置。
func (p *Page) HTML(url string) string {
	total := p.TotalPages()
	if total <= 1 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav><ul style="display:flex;gap:4px;list-style:none;padding:0;">`)

	// 上一页
	if p.HasPrev() {
		b.WriteString(fmt.Sprintf(`<li><a href="%s" style="padding:4px 8px;background:#45475a;color:#cdd6f4;border-radius:3px;text-decoration:none;">&laquo;</a></li>`,
			p.buildPageURL(url, p.PrevPage())))
	}

	if p.Simple {
		// simple 模式：仅显示上一页/下一页，中间显示页码信息
		b.WriteString(fmt.Sprintf(`<li><span style="padding:4px 8px;color:#cdd6f4;">%d/%d</span></li>`, p.Current, total))
	} else {
		for _, i := range p.Range(7) {
			if i == p.Current {
				b.WriteString(fmt.Sprintf(`<li><span style="padding:4px 8px;background:#89b4fa;color:#1e1e2e;border-radius:3px;">%d</span></li>`, i))
			} else {
				b.WriteString(fmt.Sprintf(`<li><a href="%s" style="padding:4px 8px;background:#45475a;color:#cdd6f4;border-radius:3px;text-decoration:none;">%d</a></li>`,
					p.buildPageURL(url, i), i))
			}
		}
	}

	// 下一页
	if p.HasNext() {
		b.WriteString(fmt.Sprintf(`<li><a href="%s" style="padding:4px 8px;background:#45475a;color:#cdd6f4;border-radius:3px;text-decoration:none;">&raquo;</a></li>`,
			p.buildPageURL(url, p.NextPage())))
	}

	b.WriteString(`</ul></nav>`)
	return b.String()
}
