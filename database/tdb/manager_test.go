package tdb

import (
	"context"
	"testing"
)

func TestManagerCachesNamedConnections(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Default: "main",
		Connections: map[string]Config{
			"main": {Driver: "tdb_mem", Dialect: "mysql", DSN: "manager-main"},
		},
	})
	first, err := manager.Connection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Connection(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("named connection was not cached")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRejectsUnknownConnection(t *testing.T) {
	manager := NewManager(ManagerConfig{Default: "missing", Connections: map[string]Config{}})
	if _, err := manager.Connection(context.Background()); err == nil {
		t.Fatal("expected missing connection error")
	}
}
