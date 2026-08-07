package tview

import (
	"os"
	"path/filepath"
	"testing"
)

func writeT(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderBasic(t *testing.T) {
	dir := t.TempDir()
	writeT(t, dir, "hello.html", `<h1>{{.name}}</h1>`)
	e := New(dir)
	got, err := e.Render("hello", map[string]any{"name": "tingo"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "<h1>tingo</h1>" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderXSS(t *testing.T) {
	dir := t.TempDir()
	writeT(t, dir, "x.html", `{{.v}}`)
	e := New(dir)
	got, _ := e.Render("x", map[string]any{"v": "<script>alert(1)</script>"})
	want := "&lt;script&gt;alert(1)&lt;/script&gt;"
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestRenderRaw(t *testing.T) {
	dir := t.TempDir()
	writeT(t, dir, "r.html", `{{raw .v}}`)
	e := New(dir)
	got, _ := e.Render("r", map[string]any{"v": "<b>bold</b>"})
	if got != "<b>bold</b>" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderInLayout(t *testing.T) {
	dir := t.TempDir()
	writeT(t, dir, "layout.html", `<html><body>{{.content}}</body></html>`)
	writeT(t, dir, "page.html", `<p>{{.title}}</p>`)
	e := New(dir)
	got, err := e.RenderIn("layout", "page", map[string]any{"title": "Home"})
	if err != nil {
		t.Fatal(err)
	}
	want := "<html><body><p>Home</p></body></html>"
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestShare(t *testing.T) {
	dir := t.TempDir()
	writeT(t, dir, "s.html", `{{.app}}:{{.name}}`)
	e := New(dir)
	e.Share("app", "tingo")
	got, _ := e.Render("s", map[string]any{"name": "x"})
	if got != "tingo:x" {
		t.Fatalf("got %q", got)
	}
}
