package tevent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type UserCreated struct {
	ID int
}

func TestSubscribeDispatch(t *testing.T) {
	b := NewBus(false)
	ev := New[UserCreated]("user.created")
	var got int
	id := Subscribe(b, ev, func(_ context.Context, p UserCreated) error {
		got = p.ID
		return nil
	})
	if b.Len("user.created") != 1 {
		t.Fatal("should have 1 subscriber")
	}
	if err := Dispatch(b, context.Background(), ev, UserCreated{ID: 42}); err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("got %d", got)
	}
	Unsubscribe(b, "user.created", id)
	if b.Len("user.created") != 0 {
		t.Fatal("should have 0 subscribers")
	}
}

func TestOnce(t *testing.T) {
	b := NewBus(false)
	ev := New[int]("tick")
	count := 0
	Once(b, ev, func(_ context.Context, _ int) error { count++; return nil })
	_ = Dispatch(b, context.Background(), ev, 1)
	_ = Dispatch(b, context.Background(), ev, 2)
	if count != 1 {
		t.Fatalf("once should fire 1 time, got %d", count)
	}
}

func TestErrorPropagation(t *testing.T) {
	b := NewBus(false)
	ev := New[string]("e")
	Subscribe(b, ev, func(_ context.Context, _ string) error { return errors.New("boom") })
	Subscribe(b, ev, func(_ context.Context, _ string) error { return nil })
	if err := Dispatch(b, context.Background(), ev, "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAsync(t *testing.T) {
	b := NewBus(true)
	ev := New[int]("a")
	var mu sync.Mutex
	var sum int
	var wg sync.WaitGroup
	wg.Add(1)
	Subscribe(b, ev, func(_ context.Context, p int) error {
		mu.Lock()
		sum += p
		mu.Unlock()
		wg.Done()
		return nil
	})
	DispatchAsync(b, context.Background(), ev, 5)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("async handler not called")
	}
	b.Wait()
	if sum != 5 {
		t.Fatalf("sum=%d", sum)
	}
}
