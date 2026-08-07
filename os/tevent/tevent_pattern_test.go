// Package tevent 通配订阅测试。
package tevent_test

import (
	"context"
	"sync"
	"testing"

	"github.com/xmszy/tingo/os/tevent"
)

func TestSubscribePattern(t *testing.T) {
	bus := tevent.NewBus(false)

	var called bool
	var mu sync.Mutex
	tevent.SubscribePattern(bus, "user.", func(ctx context.Context, payload any) error {
		mu.Lock()
		defer mu.Unlock()
		called = true
		return nil
	})

	tevent.Dispatch(bus, context.Background(), tevent.New[string]("user.login"), "hello")
	if !called {
		t.Error("pattern subscriber was not called for user.login")
	}

	called = false
	tevent.Dispatch(bus, context.Background(), tevent.New[int]("user.logout"), 42)
	if !called {
		t.Error("pattern subscriber was not called for user.logout")
	}

	called = false
	tevent.Dispatch(bus, context.Background(), tevent.New[string]("order.create"), "test")
	if called {
		t.Error("pattern subscriber was called for non-matching order.create")
	}
}

func TestSubscribePattern_TypedPayload(t *testing.T) {
	bus := tevent.NewBus(false)

	type UserEvent struct {
		UserID int
		Action string
	}

	var received any
	tevent.SubscribePattern(bus, "user.", func(ctx context.Context, payload any) error {
		received = payload
		return nil
	})

	evt := UserEvent{UserID: 1, Action: "login"}
	tevent.Dispatch(bus, context.Background(), tevent.New[UserEvent]("user.login"), evt)

	r, ok := received.(UserEvent)
	if !ok {
		t.Fatalf("payload type mismatch: %T", received)
	}
	if r.UserID != 1 || r.Action != "login" {
		t.Errorf("unexpected payload: %+v", r)
	}
}

func TestSubscribePattern_ExactMatchStillWorks(t *testing.T) {
	bus := tevent.NewBus(false)

	var exactCalled, patternCalled bool
	var mu sync.Mutex

	tevent.Subscribe(bus, tevent.New[string]("user.login"), func(ctx context.Context, payload string) error {
		mu.Lock()
		defer mu.Unlock()
		exactCalled = true
		return nil
	})
	tevent.SubscribePattern(bus, "user.", func(ctx context.Context, payload any) error {
		mu.Lock()
		defer mu.Unlock()
		patternCalled = true
		return nil
	})

	tevent.Dispatch(bus, context.Background(), tevent.New[string]("user.login"), "test")

	if !exactCalled {
		t.Error("exact subscriber was not called")
	}
	if !patternCalled {
		t.Error("pattern subscriber was not called")
	}
}

func TestSubscribePattern_Unsubscribe(t *testing.T) {
	bus := tevent.NewBus(false)

	id := tevent.SubscribePattern(bus, "user.", func(ctx context.Context, payload any) error {
		return nil
	})

	tevent.Unsubscribe(bus, "user.", id)

	var called bool
	var mu sync.Mutex
	tevent.Subscribe(bus, tevent.New[string]("user.login"), func(ctx context.Context, payload string) error {
		mu.Lock()
		defer mu.Unlock()
		called = true
		return nil
	})
	tevent.Dispatch(bus, context.Background(), tevent.New[string]("user.login"), "test")
	if !called {
		t.Error("exact subscriber was not called after pattern unsubscribe")
	}
}

func TestSubscribePattern_Async(t *testing.T) {
	bus := tevent.NewBus(true)
	var wg sync.WaitGroup
	wg.Add(2)

	tevent.SubscribePattern(bus, "user.", func(ctx context.Context, payload any) error {
		wg.Done()
		return nil
	})

	tevent.DispatchAsync(bus, context.Background(), tevent.New[string]("user.login"), "async1")
	tevent.DispatchAsync(bus, context.Background(), tevent.New[int]("user.logout"), 42)
	wg.Wait()
	bus.Wait()
}
