package t

import (
	"context"
	"testing"
)

func TestViewFacade(t *testing.T) {
	v := ViewNew(t.TempDir())
	if v == nil {
		t.Fatal("view engine nil")
	}
}

func TestLangFacade(t *testing.T) {
	tr := LangNew("zh", "en")
	tr.Add("zh", map[string]string{"hi": "你好 {name}"})
	if got := tr.Translate("hi", map[string]any{"name": "x"}); got != "你好 x" {
		t.Fatalf("got %q", got)
	}
}

func TestEventFacade(t *testing.T) {
	b := BusNew(false)
	ev := EventNew[int]("n")
	got := 0
	BusSubscribe(b, ev, func(_ context.Context, p int) error { got = p; return nil })
	if err := BusDispatch(b, context.Background(), ev, 5); err != nil {
		t.Fatal(err)
	}
	if got != 5 {
		t.Fatalf("got %d", got)
	}
}

func TestQueueFacade(t *testing.T) {
	q := QueueNew[int](false, 0)
	q.Subscribe(func(_ context.Context, m QueueMessage[int]) error { return nil })
	if err := q.Publish(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestCronFacade(t *testing.T) {
	c := CronNew(nil)
	if err := c.Add("t", "0 * * * *", func() {}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionFacade(t *testing.T) {
	m := SessionNew(SessionConfig{})
	s, err := m.LoadOrCreate("")
	if err != nil {
		t.Fatal(err)
	}
	s.Set("k", "v")
	if err := m.Save(s); err != nil {
		t.Fatal(err)
	}
	if v, ok := SessionGet[string](s, "k"); !ok || v != "v" {
		t.Fatalf("v=%q ok=%v", v, ok)
	}
}
