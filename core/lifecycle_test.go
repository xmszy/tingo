package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type lifecycleService struct {
	name      string
	dependsOn []string
	events    *[]string
	bootErr   error
}

func (s lifecycleService) Name() string        { return s.name }
func (s lifecycleService) DependsOn() []string { return s.dependsOn }
func (s lifecycleService) Register(*App) error {
	*s.events = append(*s.events, "register:"+s.name)
	return nil
}
func (s lifecycleService) Boot(context.Context, *App) error {
	*s.events = append(*s.events, "boot:"+s.name)
	return s.bootErr
}
func (s lifecycleService) Shutdown(context.Context) error {
	*s.events = append(*s.events, "shutdown:"+s.name)
	return nil
}

func TestAppLifecycleOrdersServicesAndShutdown(t *testing.T) {
	events := []string{}
	app := NewApp()
	if err := app.Register(
		lifecycleService{name: "http", dependsOn: []string{"config"}, events: &events},
		lifecycleService{name: "config", events: &events},
	); err != nil {
		t.Fatal(err)
	}
	if err := app.Boot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if app.State() != StateBooted {
		t.Fatalf("state = %s", app.State())
	}
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	if app.State() != StateRunning {
		t.Fatalf("running state = %s", app.State())
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"register:config", "register:http",
		"boot:config", "boot:http",
		"shutdown:http", "shutdown:config",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown must be idempotent: %v", err)
	}
}

func TestAppStartRequiresBoot(t *testing.T) {
	app := NewApp()
	if err := app.Start(); err == nil {
		t.Fatal("created app unexpectedly started")
	}
	if app.State() != StateCreated {
		t.Fatalf("state = %s", app.State())
	}
}

func TestAppBootFailureRollsBackServices(t *testing.T) {
	events := []string{}
	app := NewApp()
	boom := errors.New("boom")
	if err := app.Register(
		lifecycleService{name: "config", events: &events},
		lifecycleService{name: "http", dependsOn: []string{"config"}, events: &events, bootErr: boom},
	); err != nil {
		t.Fatal(err)
	}
	if err := app.Boot(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("boot error = %v", err)
	}
	if app.State() != StateFailed {
		t.Fatalf("state = %s", app.State())
	}
	wantTail := []string{"shutdown:http", "shutdown:config"}
	if !reflect.DeepEqual(events[len(events)-2:], wantTail) {
		t.Fatalf("rollback tail = %v", events)
	}
}

func TestAppRejectsMissingAndCyclicDependencies(t *testing.T) {
	for name, services := range map[string][]Service{
		"missing": {
			lifecycleService{name: "http", dependsOn: []string{"config"}, events: &[]string{}},
		},
		"cycle": {
			lifecycleService{name: "a", dependsOn: []string{"b"}, events: &[]string{}},
			lifecycleService{name: "b", dependsOn: []string{"a"}, events: &[]string{}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			app := NewApp()
			if err := app.Register(services...); err != nil {
				t.Fatal(err)
			}
			if err := app.Boot(context.Background()); err == nil {
				t.Fatal("expected dependency error")
			}
		})
	}
}
