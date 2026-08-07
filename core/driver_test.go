package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

type driverTestConfig struct {
	Type string
	ID   string
}

type driverTestDriver struct {
	id     string
	closed *atomic.Int32
}

func (d *driverTestDriver) Close() error {
	d.closed.Add(1)
	return nil
}

func TestDriverManagerCachesNamedDriverConcurrently(t *testing.T) {
	registry := NewDriverRegistry[driverTestConfig, *driverTestDriver]()
	var created atomic.Int32
	closed := &atomic.Int32{}
	if err := registry.Register("memory", func(_ context.Context, cfg driverTestConfig) (*driverTestDriver, error) {
		created.Add(1)
		return &driverTestDriver{id: cfg.ID, closed: closed}, nil
	}); err != nil {
		t.Fatal(err)
	}
	manager := NewDriverManager(DriverManagerConfig[driverTestConfig, *driverTestDriver]{
		Registry: registry,
		Default:  func() string { return "main" },
		Resolve: func(name string) (driverTestConfig, error) {
			return driverTestConfig{Type: "memory", ID: name}, nil
		},
		Type: func(cfg driverTestConfig) string { return cfg.Type },
	})

	var wg sync.WaitGroup
	values := make(chan *driverTestDriver, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := manager.Connection(context.Background())
			if err != nil {
				t.Errorf("connection: %v", err)
				return
			}
			values <- d
		}()
	}
	wg.Wait()
	close(values)
	var first *driverTestDriver
	for value := range values {
		if first == nil {
			first = value
		}
		if value != first {
			t.Fatal("manager returned different cached instances")
		}
	}
	if created.Load() != 1 {
		t.Fatalf("created = %d", created.Load())
	}
	if err := manager.Forget(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	if closed.Load() != 1 {
		t.Fatalf("closed = %d", closed.Load())
	}
}

func TestDriverRegistryRejectsDuplicateFactory(t *testing.T) {
	registry := NewDriverRegistry[driverTestConfig, *driverTestDriver]()
	factory := func(context.Context, driverTestConfig) (*driverTestDriver, error) { return &driverTestDriver{}, nil }
	if err := registry.Register("memory", factory); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("memory", factory); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}
