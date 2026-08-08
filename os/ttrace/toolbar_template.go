package ttrace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// toolbarCSS 是调试工具栏的默认样式（纯 CSS 内容，不含 <style> 标签；
// <style> 由模板负责包裹，避免与模板里的 <style>{{.CSS}}</style> 重复嵌套）。
const toolbarCSS = `
    #tingo_page_trace {position:fixed; bottom:0; right:0; font-size:14px; width:100%; z-index:999999; color:#000; text-align:left; font-family:'微软雅黑';}
    #tingo_page_trace_tab {display:none; background:white; margin:0; height: 250px;}
    #tingo_page_trace_tab_tit {height:30px; padding: 6px 12px 0; border-bottom:1px solid #ececec; border-top:1px solid #ececec; font-size:16px;}
    #tingo_page_trace_tab_tit>span {color:#000; padding-right:12px; height:30px; line-height:30px; display:inline-block; margin-right:3px; cursor:pointer; font-weight:700;}
    #tingo_page_trace_tab_cont {overflow:auto; height:212px; padding:0; line-height:24px;}
    #tingo_page_trace_tab_cont>div {display:none;}
    #tingo_page_trace_tab_cont>div>ol {padding:0; margin:0;}
    #tingo_page_trace_tab_cont>div>ol>li {border-bottom:1px solid #eee; font-size:14px; padding:0 12px;}
    #tingo_page_trace_close {display:none; text-align:right; height:18px; position:absolute; top:10px; right:12px; cursor:pointer; z-index: 999999;}
    #tingo_page_trace_close>svg {width:18px; height:18px; vertical-align:top;}
    #tingo_page_trace_open {height:30px; float:right; text-align:right; overflow:hidden; position:fixed; bottom:0; right:0; color:#000; line-height:30px; cursor:pointer; z-index:999999;}
    #tingo_page_trace_open>svg {width:30px; height:30px; float:right; vertical-align:top;}
    #tingo_page_trace_open>div {background:#232323; color:#fff; padding:0 6px; float:right; line-height:30px; font-size:14px;}
`

// openIcon / closeIcon 是工具栏的「显示 / 关闭」按钮图标（内联 SVG）。
const openIcon = `<svg t="1786172872999" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1084" width="200" height="200"><path d="M512 960A448 448 0 1 1 512 64a448 448 0 0 1 0 896z m0-64A384 384 0 1 0 512 128a384 384 0 0 0 0 768z" fill="#E9534B" p-id="1085"></path><path d="M662.912 208L272 529.792l283.136 44.352L415.36 848l368.576-357.952-262.016-34.048z" fill="#E9534B" p-id="1086"></path></svg>`

const closeIcon = `<svg t="1786172890226" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1230" width="200" height="200"><path d="M512 570.88l196.864 196.8 58.88-58.88L570.752 512l196.864-196.864-58.816-58.88L512 453.248 315.136 256.32l-58.88 58.88L453.248 512l-196.864 196.864 58.88 58.88z" fill="#FF0000" p-id="1231"></path></svg>`

// traceTab 是一个分区的渲染数据（标题 + 行列表）。
type traceTab struct {
	Title string   // 分区标题（基本 / 文件 / 流程 / 错误 / SQL / 调试）
	Items []string // 每个 <li> 的文本（已转义，模板中原样输出）
}

// traceData 是渲染面板时传给模板的数据结构。
//
// 面板为框架可信内容，使用 text/template 原样渲染（不做 HTML 上下文转义），
// 以保证 <style>/<script>/base64 图片等完整无篡改，浮层脚本可正常点开。
//
// 自定义模板（仿 ThinkPHP 的 'file' 参数）可使用以下字段：
//
//	{{.CSS}}        —— 默认 <style> 块（可直接替换为自己的样式）
//	{{.OpenIcon}}   —— 「显示」图标（内联 SVG）
//	{{.CloseIcon}}  —— 「关闭」图标（内联 SVG）
//	{{.Script}}     —— 默认交互脚本
//	{{.Open}}       —— 耗时字符串（如 "0.196s"）
//	{{range .Tabs}} —— 各分区：{{.Title}}、{{range .Items}}<li>{{.}}</li>{{end}}
type traceData struct {
	CSS       string
	OpenIcon  string
	CloseIcon string
	Script    string
	Open      string
	Tabs      []traceTab
}

// defaultTemplate 是内置默认面板模板（仿 ThinkPHP page_trace.tpl，使用 Go html/template 语法）。
//
// 若想自定义样式，复制此模板到项目内任意 .html 文件，按需修改后，在配置里设置
// [toolbar] file = "你的模板路径.html" 即可（与 ThinkPHP 的 trace 'file' 参数等价）。
const defaultTemplate = `<style>{{.CSS}}</style>
<div id="tingo_page_trace">
    <div id="tingo_page_trace_tab">
        <div id="tingo_page_trace_tab_tit">{{range .Tabs}}<span>{{.Title}}</span>{{end}}</div>
        <div id="tingo_page_trace_tab_cont">{{range .Tabs}}<div><ol>{{if .Items}}{{range .Items}}<li>{{.}}</li>{{else}}<li>（空）</li>{{end}}{{end}}</ol></div>{{end}}</div>
    </div>
    <div id="tingo_page_trace_close">{{.CloseIcon}}</div>
</div>
<div id="tingo_page_trace_open"><div>{{.Open}}</div>{{.OpenIcon}}</div>
<script>{{.Script}}</script>`

// loadTemplate 惰性加载面板模板：若配置了 File 且文件存在，则用它；
// 否则使用内置默认模板。解析失败时回退到内置模板（不致命）。
func (tb *Toolbar) loadTemplate() (*template.Template, error) {
	tb.tplOnce.Do(func() {
		src := defaultTemplate
		used := "builtin"
		if f := strings.TrimSpace(tb.Config.File); f != "" {
			if data, err := os.ReadFile(f); err == nil {
				src = string(data)
				used = f
			} else {
				// 文件缺失：回退内置，记录错误但不影响启动。
				tb.tplErr = fmt.Errorf("ttrace: 自定义模板 %q 读取失败，已回退内置模板: %w", f, err)
			}
		}
		tpl, err := template.New("tingo_page_trace").Parse(src)
		if err != nil {
			// 自定义模板语法错误：回退内置，避免整页出错。
			tb.tplErr = fmt.Errorf("ttrace: 模板 %q 解析失败，已回退内置模板: %w", used, err)
			tpl, _ = template.New("tingo_page_trace").Parse(defaultTemplate)
		}
		tb.tpl = tpl
	})
	return tb.tpl, tb.tplErr
}

// render 用当前加载的模板渲染完整面板 HTML。
func (tb *Toolbar) render(elapsedSec string, tabs []traceTab) string {
	tpl, err := tb.loadTemplate()
	if tpl == nil || err != nil {
		// 极端情况下模板不可用，回退到内置字符串拼接，保证面板不消失。
		return renderFallback(elapsedSec, tabs)
	}
	data := traceData{
		CSS:       toolbarCSS,
		OpenIcon:  openIcon,
		CloseIcon: closeIcon,
		Script:    traceScript,
		Open:      elapsedSec,
		Tabs:      tabs,
	}
	var sb strings.Builder
	sb.WriteString("\n<!-- Tingo Page Trace -->\n")
	if err := tpl.Execute(&sb, data); err != nil {
		return renderFallback(elapsedSec, tabs)
	}
	sb.WriteString("\n<!-- /Tingo Page Trace -->\n")
	return sb.String()
}

// renderFallback 是模板不可用时的最后兜底（字符串拼接，等价于旧实现）。
func renderFallback(elapsedSec string, tabs []traceTab) string {
	var sb strings.Builder
	sb.WriteString("\n<!-- Tingo Page Trace -->\n")
	sb.WriteString("<style>")
	sb.WriteString(toolbarCSS)
	sb.WriteString("</style>\n")
	sb.WriteString(`<div id="tingo_page_trace">
    <div id="tingo_page_trace_tab">
        <div id="tingo_page_trace_tab_tit">`)
	for _, t := range tabs {
		fmt.Fprintf(&sb, `<span>%s</span>`, template.HTMLEscapeString(t.Title))
	}
	sb.WriteString(`</div>
        <div id="tingo_page_trace_tab_cont">`)
	for _, t := range tabs {
		sb.WriteString(`<div><ol>`)
		if len(t.Items) == 0 {
			sb.WriteString(`<li>（空）</li>`)
		}
		for _, it := range t.Items {
			sb.WriteString("<li>")
			sb.WriteString(it)
			sb.WriteString("</li>")
		}
		sb.WriteString(`</ol></div>`)
	}
	sb.WriteString(`</div>
    </div>
    <div id="tingo_page_trace_close">
        ` + closeIcon + `
    </div>
</div>`)
	sb.WriteString(`<div id="tingo_page_trace_open"><div>`)
	sb.WriteString(elapsedSec)
	sb.WriteString(`</div>` + openIcon + `
</div>
`)
	sb.WriteString("<script type=\"text/javascript\">")
	sb.WriteString(traceScript)
	sb.WriteString("</script>\n")
	sb.WriteString("\n<!-- /Tingo Page Trace -->\n")
	return sb.String()
}

// traceScript 是 TP 原版脚本逻辑（cookie 记忆 + tab 切换），一比一保留。
// 仅含脚本体，不含 <script> 标签（<script> 由模板负责包裹，避免重复嵌套）。
const traceScript = `
    (function(){
        var tab_tit  = document.getElementById('tingo_page_trace_tab_tit').getElementsByTagName('span');
        var tab_cont = document.getElementById('tingo_page_trace_tab_cont').getElementsByTagName('div');
        var open     = document.getElementById('tingo_page_trace_open');
        var close    = document.getElementById('tingo_page_trace_close').children[0];
        var trace    = document.getElementById('tingo_page_trace_tab');
        var cookie   = document.cookie.match(/thinkphp_show_page_trace=(\d\|\d)/);
        var history  = (cookie && typeof cookie[1] != 'undefined' && cookie[1].split('|')) || [0,0];
        open.onclick = function(){
            trace.style.display = 'block';
            this.style.display = 'none';
            close.parentNode.style.display = 'block';
            history[0] = 1;
            document.cookie = 'thinkphp_show_page_trace='+history.join('|')
        }
        close.onclick = function(){
            trace.style.display = 'none';
            this.parentNode.style.display = 'none';
            open.style.display = 'block';
            history[0] = 0;
            document.cookie = 'thinkphp_show_page_trace='+history.join('|')
        }
        for(var i = 0; i < tab_tit.length; i++){
            tab_tit[i].onclick = (function(i){
                return function(){
                    for(var j = 0; j < tab_cont.length; j++){
                        tab_cont[j].style.display = 'none';
                        tab_tit[j].style.color = '#999';
                    }
                    tab_cont[i].style.display = 'block';
                    tab_tit[i].style.color = '#000';
                    history[1] = i;
                    document.cookie = 'thinkphp_show_page_trace='+history.join('|')
                }
            })(i)
        }
        parseInt(history[0]) && open.click();
        tab_tit[history[1]].click();
    })();
`

// formatElapsedSec 把耗时格式化为自适应单位字符串，避免「0.000000s」这类
// 极短耗时显示成 0 的假象：<1µs 显示 ns，<1ms 显示 µs，<1s 显示 ms，否则显示 s。
func formatElapsedSec(elapsed time.Duration) string {
	switch {
	case elapsed < time.Microsecond:
		return fmt.Sprintf("%.0fns", float64(elapsed))
	case elapsed < time.Millisecond:
		return fmt.Sprintf("%.2fµs", float64(elapsed)/float64(time.Microsecond))
	case elapsed < time.Second:
		return fmt.Sprintf("%.2fms", float64(elapsed)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.4fs", float64(elapsed)/float64(time.Second))
	}
}

// CopyBuiltinTemplate 把内置默认模板写到 dest（供脚手架生成示例模板文件使用）。
func CopyBuiltinTemplate(dest string) error {
	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(dest, []byte(defaultTemplate), 0o644)
}
