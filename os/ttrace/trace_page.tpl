<style>{{.CSS}}</style>
<div id="tingo_page_trace">
    <div id="tingo_page_trace_tab">
        <div id="tingo_page_trace_tab_tit">{{range .Tabs}}<span>{{.Title}}</span>{{end}}</div>
        <div id="tingo_page_trace_tab_cont">{{range .Tabs}}<div><ol>{{if .Items}}{{range .Items}}<li>{{.}}</li>{{else}}<li>（空）</li>{{end}}{{end}}</ol></div>{{end}}</div>
    </div>
    <div id="tingo_page_trace_close">{{.CloseIcon}}</div>
</div>
<div id="tingo_page_trace_open"><div>{{.Open}}</div>{{.OpenIcon}}</div>
<script>{{.Script}}</script>
