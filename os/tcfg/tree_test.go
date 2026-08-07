package tcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDirUsesFileNamespaces(t *testing.T) {
	dir := t.TempDir()
	mustWriteConfig(t, filepath.Join(dir, "app.toml"), "debug = true\n[server]\naddr = ':8080'\n")
	mustWriteConfig(t, filepath.Join(dir, "database.toml"), "default = 'mysql'\n")
	mustWriteConfig(t, filepath.Join(dir, "notes.txt"), "ignored")

	tree, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := tree.Lookup("app.server.addr"); got != ":8080" {
		t.Fatalf("app.server.addr = %#v", got)
	}
	if got, _ := tree.Lookup("database.default"); got != "mysql" {
		t.Fatalf("database.default = %#v", got)
	}
	if _, exists := tree.Lookup("notes"); exists {
		t.Fatal("unsupported config file was loaded")
	}
}

func TestReadDirWithExtensionFiltersFormats(t *testing.T) {
	dir := t.TempDir()
	mustWriteConfig(t, filepath.Join(dir, "app.toml"), "name = 'toml'\n")
	mustWriteConfig(t, filepath.Join(dir, "app.json"), `{"name":"json"}`)

	tree, err := ReadDirWithExtension(dir, ".json")
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.String("app.name"); got != "json" {
		t.Fatalf("app.name = %q", got)
	}
	if _, err := ReadDirWithExtension(dir, "unsupported"); err == nil {
		t.Fatal("unsupported extension was accepted")
	}
}

func TestReadINIWithNestedSectionsAndTypes(t *testing.T) {
	dir := t.TempDir()
	mustWriteConfig(t, filepath.Join(dir, "app.ini"), `debug = (true)
default_app = app
workers = 4
ratio = 1.5
empty = (empty)
nullable = (null)
hosts = ["a", "b"]

[server]
addr = :8080

[connections.mysql]
hostname = 127.0.0.1
hostport = 3306
`)

	tree, err := ReadDirWithExtension(dir, "ini")
	if err != nil {
		t.Fatal(err)
	}
	if !tree.Bool("app.debug") || tree.Int("app.workers") != 4 || tree.Float64("app.ratio") != 1.5 {
		t.Fatalf("INI scalar conversion failed: %#v", tree.Get("app"))
	}
	if tree.String("app.server.addr") != ":8080" || tree.String("app.connections.mysql.hostname") != "127.0.0.1" {
		t.Fatalf("INI nested section failed: %#v", tree.Get("app"))
	}
	if tree.Int("app.connections.mysql.hostport") != 3306 {
		t.Fatalf("INI integer conversion failed: %#v", tree.Get("app.connections.mysql.hostport"))
	}
	if tree.Get("app.nullable", "fallback") != nil || tree.String("app.empty", "fallback") != "" {
		t.Fatalf("INI empty/null conversion failed: %#v", tree.Get("app"))
	}
	if got := tree.Strings("app.hosts"); len(got) != 2 || got[1] != "b" {
		t.Fatalf("INI array conversion failed: %#v", got)
	}
}

func TestMergeTreesDeeplyOverridesWithoutDroppingSiblings(t *testing.T) {
	global := Tree{"app": map[string]any{
		"debug":  true,
		"server": map[string]any{"addr": ":8080", "print_routes": true},
	}}
	application := Tree{"app": map[string]any{
		"server": map[string]any{"print_routes": false},
	}}

	merged := MergeTrees(global, application)
	if got, _ := merged.Lookup("app.server.addr"); got != ":8080" {
		t.Fatalf("sibling field was dropped: %#v", got)
	}
	if got, _ := merged.Lookup("app.server.print_routes"); got != false {
		t.Fatalf("application override was not applied: %#v", got)
	}
	if got, _ := global.Lookup("app.server.print_routes"); got != true {
		t.Fatal("merge mutated its input")
	}
}

func TestTreeDecodeAt(t *testing.T) {
	tree := Tree{"app": map[string]any{"prefix": "/admin", "priority": 10}}
	var target struct {
		Prefix   string `json:"prefix"`
		Priority int    `json:"priority"`
	}
	if err := tree.DecodeAt("app", &target); err != nil {
		t.Fatal(err)
	}
	if target.Prefix != "/admin" || target.Priority != 10 {
		t.Fatalf("decoded config = %+v", target)
	}
}

func TestTreeIndexedPathAndWeakConversion(t *testing.T) {
	tree := Tree{
		"servers": []any{map[string]any{"host": "127.0.0.1", "port": "8080"}},
		"enabled": "true",
	}
	if got := tree.String("servers.0.host"); got != "127.0.0.1" {
		t.Fatalf("indexed host = %q", got)
	}
	if got := tree.Int("servers.0.port"); got != 8080 {
		t.Fatalf("converted port = %d", got)
	}
	if !tree.Bool("enabled") {
		t.Fatal("converted bool = false")
	}
	if _, ok := tree.Lookup("servers.1.host"); ok {
		t.Fatal("out-of-range path exists")
	}
}

func TestTreeTypedReads(t *testing.T) {
	tree := Tree{"app": map[string]any{
		"name":    "tingo",
		"debug":   true,
		"workers": int64(4),
		"ratio":   1.5,
		"hosts":   []any{"a", "b"},
	}}

	if got := tree.Get("app.name"); got != "tingo" {
		t.Fatalf("Get(app.name) = %#v", got)
	}
	if !tree.Has("app.debug") || tree.Has("app.missing") {
		t.Fatal("Has returned an invalid result")
	}
	if got := tree.String("app.name"); got != "tingo" {
		t.Fatalf("String(app.name) = %q", got)
	}
	if got := tree.Bool("app.debug"); !got {
		t.Fatal("Bool(app.debug) = false")
	}
	if got := tree.Int("app.workers"); got != 4 {
		t.Fatalf("Int(app.workers) = %d", got)
	}
	if got := tree.Float64("app.ratio"); got != 1.5 {
		t.Fatalf("Float64(app.ratio) = %v", got)
	}
	if got := tree.Strings("app.hosts"); len(got) != 2 || got[1] != "b" {
		t.Fatalf("Strings(app.hosts) = %#v", got)
	}
	if got := tree.String("app.missing", "fallback"); got != "fallback" {
		t.Fatalf("default string = %q", got)
	}
	if got := tree.Get("app.missing", "fallback"); got != "fallback" {
		t.Fatalf("default raw value = %#v", got)
	}
}

func mustWriteConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
