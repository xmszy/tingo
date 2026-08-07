package tqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestPublishConsume(t *testing.T) {
	q := NewMemory[string](false, 0)
	var mu sync.Mutex
	var got []string
	q.Subscribe(func(_ context.Context, m Message[string]) error {
		mu.Lock()
		got = append(got, m.Payload)
		mu.Unlock()
		return nil
	})
	if err := q.Publish(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("got %v", got)
	}
}

func TestRetry(t *testing.T) {
	q := NewMemory[int](false, 2)
	var mu sync.Mutex
	attempts := 0
	q.Subscribe(func(_ context.Context, m Message[int]) error {
		mu.Lock()
		attempts++
		mu.Unlock()
		return errors.New("fail")
	})
	_ = q.Publish(context.Background(), 1)
	mu.Lock()
	defer mu.Unlock()
	// 初次 + 2 次重试 = 3 次。dispatch 同步重投，故最终 attempts=3。
	if attempts != 3 {
		t.Fatalf("attempts=%d want 3", attempts)
	}
}

func TestDeadLetter(t *testing.T) {
	q := NewMemory[int](false, 1)
	dead := make(chan int, 1)
	q.OnDeadLetter(func(_ context.Context, m Message[int], _ error) {
		dead <- m.Payload
	})
	q.Subscribe(func(_ context.Context, _ Message[int]) error {
		return errors.New("boom")
	})
	_ = q.Publish(context.Background(), 7)
	select {
	case v := <-dead:
		if v != 7 {
			t.Fatalf("dead letter payload=%d", v)
		}
	default:
		t.Fatal("no dead letter")
	}
}

func TestMessageHeaders(t *testing.T) {
	msg := Message[string]{Payload: "test"}
	msg.SetHeader("trace-id", "abc123")
	msg.SetHeader("user-id", "42")

	if msg.GetHeader("trace-id") != "abc123" {
		t.Fatalf("trace-id: got %q", msg.GetHeader("trace-id"))
	}
	if msg.GetHeader("user-id") != "42" {
		t.Fatalf("user-id: got %q", msg.GetHeader("user-id"))
	}
	if msg.GetHeader("missing") != "" {
		t.Fatalf("missing key should return empty")
	}
}

func TestPublishWithHeaders(t *testing.T) {
	q := NewMemory[string](false, 0)
	var mu sync.Mutex
	var gotMsg Message[string]
	q.Subscribe(func(_ context.Context, m Message[string]) error {
		mu.Lock()
		gotMsg = m
		mu.Unlock()
		return nil
	})

	msg := Message[string]{Payload: "hello"}
	msg.SetHeader("x-request-id", "req-001")
	if err := q.PublishMessage(context.Background(), msg); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotMsg.Payload != "hello" {
		t.Fatalf("payload: got %q", gotMsg.Payload)
	}
	if gotMsg.GetHeader("x-request-id") != "req-001" {
		t.Fatalf("header: got %q", gotMsg.GetHeader("x-request-id"))
	}
}
