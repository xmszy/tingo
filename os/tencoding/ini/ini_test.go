package ini

import "testing"

func TestParse(t *testing.T) {
	data := `
# comment
[server]
host = 127.0.0.1
port = 8080

[db]
name = "tingo"
debug : true
`
	secs, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if secs["server"]["host"] != "127.0.0.1" {
		t.Errorf("host = %q", secs["server"]["host"])
	}
	if secs["server"]["port"] != "8080" {
		t.Errorf("port = %q", secs["server"]["port"])
	}
	if secs["db"]["name"] != "tingo" {
		t.Errorf("name = %q (quote not stripped)", secs["db"]["name"])
	}
	if secs["db"]["debug"] != "true" {
		t.Errorf("debug = %q", secs["db"]["debug"])
	}
}

func TestUnmarshal(t *testing.T) {
	type Conf struct {
		Host string `ini:"server.host"`
		Port int    `ini:"server.port"`
	}
	var c Conf
	if err := Unmarshal(`
[server]
host = localhost
port = 9000
`, &c); err != nil {
		t.Fatal(err)
	}
	if c.Host != "localhost" || c.Port != 9000 {
		t.Errorf("got %+v", c)
	}
}

func TestInterpolation(t *testing.T) {
	secs, err := Parse(`
[paths]
root = /var
logs = %(root)/log
`)
	if err != nil {
		t.Fatal(err)
	}
	if secs["paths"]["logs"] != "/var/log" {
		t.Errorf("interp = %q", secs["paths"]["logs"])
	}
}
