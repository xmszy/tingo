package core

import "testing"

type configuredApplication struct{ config AppConfig }

func (a configuredApplication) Config() AppConfig { return a.config }
func (configuredApplication) Routes(Router)       {}

func TestConfigureApplicationMergesConventionAndExplicitConfig(t *testing.T) {
	app := NewApp()
	app.RegisterApplication("admin", configuredApplication{config: AppConfig{Domain: "admin.example.com", Priority: 20}})
	if err := app.ConfigureApplication("admin", AppConfig{Prefix: "/backend", Disabled: true, Priority: 10}); err != nil {
		t.Fatal(err)
	}
	info, ok := app.LookupApplication("admin")
	if !ok {
		t.Fatal("application was not registered")
	}
	if info.Config.Domain != "admin.example.com" || info.Config.Prefix != "/backend" {
		t.Fatalf("merged config = %+v", info.Config)
	}
	if info.Config.Priority != 20 {
		t.Fatalf("explicit priority did not win: %+v", info.Config)
	}
	if !info.Config.Disabled {
		t.Fatal("convention disabled flag was not preserved")
	}
}

func TestConfigureApplicationAppliesDefaultPrefix(t *testing.T) {
	app := NewApp()
	app.RegisterApplication("api", configuredApplication{})
	if err := app.ConfigureApplication("api", AppConfig{}); err != nil {
		t.Fatal(err)
	}
	info, _ := app.LookupApplication("api")
	if info.Config.Prefix != "/api" {
		t.Fatalf("default prefix = %q", info.Config.Prefix)
	}
}
