package main

import (
	"strings"
	"testing"

	"github.com/xmszy/tingo/os/tcfg"
)

func TestSingleApplicationScaffoldTemplates(t *testing.T) {
	for _, name := range []string{
		"APP_DEBUG", "SERVER_ADDR", "DB_DRIVER", "DB_TYPE", "DB_HOST",
		"DB_NAME", "DB_USER", "DB_PASS", "DB_PORT", "DB_CHARSET", "DB_PREFIX", "LOG_LEVEL",
	} {
		t.Setenv(name, "")
	}
	data := map[string]string{
		"Module":     "example.com/app",
		"Name":       "app",
		"Replace":    "",
		"LeftDelim":  "{{",
		"RightDelim": "}}",
	}
	mainFile, err := render(tplMain, data, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mainFile), "tingo.Addr") || !strings.Contains(string(mainFile), "tingo.Run()") {
		t.Fatalf("main entry is not convention based:\n%s", mainFile)
	}
	if !strings.Contains(tplDatabaseConfig, `type = "${DB_TYPE:-mysql}"`) || strings.Contains(tplDatabaseConfig, "mysql.New") {
		t.Fatalf("database template is not convention based:\n%s", tplDatabaseConfig)
	}
	application, err := render(tplApplication, data, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(application), `t.App("app", &App{})`) {
		t.Fatalf("default application was not generated:\n%s", application)
	}
	if !strings.Contains(string(application), `"example.com/app/app/route"`) || strings.Contains(string(application), "Config()") {
		t.Fatalf("application did not delegate config and routes to convention directories:\n%s", application)
	}
	if !strings.Contains(tplRoute, `t.AutoRoute(r)`) {
		t.Fatal("scaffold route should use TP-style auto routing")
	}
	if !strings.Contains(tplController, `t.RegisterController("/", &Index{})`) {
		t.Fatal("scaffold controller should self-register for auto routing")
	}
	// 控制器需为内嵌基类的结构体，而非散装函数。
	if !strings.Contains(tplController, "type Index struct") || !strings.Contains(tplController, "t.Controller") {
		t.Fatal("scaffold controller is not a tingo-style class")
	}
	// app/ 下需生成约定的装配文件。
	if !strings.Contains(tplKernel, "t.NewKernel()") {
		t.Fatal("app/kernel.go template is missing the assembly kernel")
	}
	if !strings.Contains(tplExceptionHandle, "t.NewExceptionHandle()") {
		t.Fatal("app/exception.go template is missing the exception handler")
	}
	if !strings.Contains(tplAppService, "func (*AppService) Register") {
		t.Fatal("app/service.go template is missing AppService")
	}
	if !strings.Contains(tplController, `c.Param("name")`) {
		t.Fatal("example handlers are missing from scaffold")
	}
	if !strings.Contains(tplApplicationConfig, `prefix = "/"`) || !strings.Contains(tplAppConfig, `default_app = "app"`) {
		t.Fatal("global and application config templates are not separated")
	}
	appConfig, err := render(tplAppConfig, data, false)
	if err != nil {
		t.Fatal(err)
	}
	parsedAppConfig, err := tcfg.NewFromBytes("toml", appConfig)
	if err != nil {
		t.Fatalf("app config must parse without a .env file: %v\n%s", err, appConfig)
	}
	if parsedAppConfig.Bool("debug") || !parsedAppConfig.Bool("server.print_routes") || parsedAppConfig.String("server.addr") != ":8080" {
		t.Fatalf("app config does not preserve runnable defaults (debug should default false): %#v", parsedAppConfig.Data())
	}
	databaseConfig, err := tcfg.NewFromBytes("toml", []byte(tplDatabaseConfig))
	if err != nil {
		t.Fatalf("database config must parse without a .env file: %v\n%s", err, tplDatabaseConfig)
	}
	if databaseConfig.String("default") != "mysql" || databaseConfig.String("connections.mysql.type") != "mysql" {
		t.Fatalf("database config does not preserve TP defaults: %#v", databaseConfig.Data())
	}
	viewConfig, err := render(tplViewConfig, data, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(viewConfig), `left_delim = "{{"`) || !strings.Contains(string(viewConfig), `right_delim = "}}"`) {
		t.Fatalf("view delimiters were not rendered literally:\n%s", viewConfig)
	}
	if !strings.Contains(tplEnvExample, "CONFIG_EXT = toml") || !strings.Contains(tplEnvExample, "DB_TYPE = mysql") || strings.Contains(tplEnvExample, "[DATABASE]") {
		t.Fatal("TP style environment template is missing")
	}
	for name, template := range map[string]string{
		"route": tplRouteConfig, "log": tplLogConfig, "session": tplSessionConfig, "view": tplViewConfig,
	} {
		if strings.TrimSpace(template) == "" {
			t.Fatalf("%s config template is empty", name)
		}
	}
}
