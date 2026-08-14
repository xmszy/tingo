package properties

import "testing"

func TestParse(t *testing.T) {
	// 使用原始字符串字面量，反斜杠按字面处理（一个 \ 即一个 \）。
	data := `
# db config
db.url = jdbc://localhost
db.user: root
name value-with-space
escaped = a\:b
tab = a\tb
`
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if m["db.url"] != "jdbc://localhost" {
		t.Errorf("db.url = %q", m["db.url"])
	}
	if m["db.user"] != "root" {
		t.Errorf("db.user = %q", m["db.user"])
	}
	if m["name"] != "value-with-space" {
		t.Errorf("name = %q", m["name"])
	}
	// a\:b 转义为 a:b
	if m["escaped"] != "a:b" {
		t.Errorf("escaped = %q, want %q", m["escaped"], "a:b")
	}
	// a\tb 转义为 a<tab>b
	if m["tab"] != "a\tb" {
		t.Errorf("tab = %q, want %q", m["tab"], "a\tb")
	}
}

func TestContinuation(t *testing.T) {
	m, err := Parse("long = line1 \\\nline2")
	if err != nil {
		t.Fatal(err)
	}
	if m["long"] != "line1 line2" {
		t.Errorf("long = %q", m["long"])
	}
}

func TestUnmarshal(t *testing.T) {
	type Cfg struct {
		URL  string `prop:"db.url"`
		Port int    `prop:"db.port"`
	}
	var c Cfg
	if err := Unmarshal(`
db.url = http://x
db.port = 3306
`, &c); err != nil {
		t.Fatal(err)
	}
	if c.URL != "http://x" || c.Port != 3306 {
		t.Errorf("got %+v", c)
	}
}

func TestUnescape(t *testing.T) {
	cases := map[string]string{
		`a\:b`:  "a:b",
		`a\=b`:  "a=b",
		`a\tb`:  "a\tb",
		`plain`: "plain",
		`a\\b`:  `a\b`,
	}
	for in, want := range cases {
		if got := unescape(in); got != want {
			t.Errorf("unescape(%q) = %q, want %q", in, got, want)
		}
	}
}
