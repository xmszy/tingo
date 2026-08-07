// Package tcache tag 测试
package tcache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTagSet(t *testing.T) {
	c := New()
	ts := NewTagSet(c)

	ts.SetTag("user:1", "alice", 5*time.Minute, "users", "user_1")
	ts.SetTag("user:2", "bob", 5*time.Minute, "users")
	ts.SetTag("post:1", "hello", 5*time.Minute, "posts", "user_1")

	// 验证可以读取
	if v, ok := c.Get("user:1"); !ok || v != "alice" {
		t.Fatalf("expected alice, got %v", v)
	}
	if v, ok := c.Get("user:2"); !ok || v != "bob" {
		t.Fatalf("expected bob, got %v", v)
	}
	if v, ok := c.Get("post:1"); !ok || v != "hello" {
		t.Fatalf("expected hello, got %v", v)
	}

	// 清除 users 标签
	// user:1 同时属于 users 和 user_1 → 保留
	// user:2 仅属于 users → 删除
	n := ts.FlushTag("users")
	if n != 1 {
		t.Fatalf("expected 1 key deleted (user:2 only), got %d", n)
	}

	// user:1 还在（被 user_1 标签保护）
	if _, ok := c.Get("user:1"); !ok {
		t.Fatal("user:1 should still exist via user_1 tag")
	}
	// user:2 应该被删除
	if _, ok := c.Get("user:2"); ok {
		t.Fatal("user:2 should be deleted")
	}
	// post:1 还在（不在 users 标签中）
	if _, ok := c.Get("post:1"); !ok {
		t.Fatal("post:1 should still exist")
	}

	// 清除 user_1 标签
	// user:1 仅剩 user_1 → 删除
	// post:1 同时属于 posts 和 user_1 → 保留
	n = ts.FlushTag("user_1")
	if n != 1 {
		t.Fatalf("expected 1 key deleted (user:1 only), got %d", n)
	}
	if _, ok := c.Get("user:1"); ok {
		t.Fatal("user:1 should be deleted after user_1 flush")
	}
	if _, ok := c.Get("post:1"); !ok {
		t.Fatal("post:1 should still exist via posts tag")
	}
}

func TestTagSetOverride(t *testing.T) {
	c := New()
	ts := NewTagSet(c)

	// 先写入，再覆盖
	ts.SetTag("key", "v1", 5*time.Minute, "tag_a")
	ts.SetTag("key", "v2", 5*time.Minute, "tag_b")

	// 清除 tag_a：key 依然在 tag_b 中
	ts.FlushTag("tag_a")
	if _, ok := c.Get("key"); !ok {
		t.Fatal("key should still exist via tag_b")
	}

	// 清除 tag_b
	ts.FlushTag("tag_b")
	if _, ok := c.Get("key"); ok {
		t.Fatal("key should be deleted after tag_b flush")
	}
}

func TestAppendTag(t *testing.T) {
	c := New()
	ts := NewTagSet(c)

	ts.SetTag("key", "val", 5*time.Minute, "tag_a")
	ts.AppendTag("key", "tag_b", "tag_c")

	// 清除 tag_a
	ts.FlushTag("tag_a")
	if _, ok := c.Get("key"); !ok {
		t.Fatal("key should still exist via tag_b/tag_c")
	}

	// 清除 tag_b
	ts.FlushTag("tag_b")
	if _, ok := c.Get("key"); !ok {
		t.Fatal("key should still exist via tag_c")
	}

	ts.FlushTag("tag_c")
	if _, ok := c.Get("key"); ok {
		t.Fatal("key should be deleted after all tags flushed")
	}
}

func TestRemember(t *testing.T) {
	c := New()
	callCount := 0

	// 并发调用 Remember 100 次，fn 只应执行 1 次
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Remember("expensive_key", 5*time.Minute, func() (any, error) {
				callCount++
				time.Sleep(10 * time.Millisecond)
				return "result", nil
			})
			if err != nil {
				t.Errorf("Remember returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	// fn 只应执行 1 次
	if callCount > 1 {
		t.Fatalf("expected fn to be called once, got %d calls", callCount)
	}

	// 验证缓存存在
	if v, ok := c.Get("expensive_key"); !ok || v != "result" {
		t.Fatalf("expected cached 'result', got %v", v)
	}
}

func TestRememberFunc(t *testing.T) {
	c := New()
	callCount := 0

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := RememberFunc(c, "typed_key", 5*time.Minute, func() (string, error) {
				callCount++
				return "typed_result", nil
			})
			if err != nil {
				t.Errorf("RememberFunc returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	if callCount > 1 {
		t.Fatalf("expected fn to be called once, got %d calls", callCount)
	}

	if v, ok := Get[string](c, "typed_key"); !ok || v != "typed_result" {
		t.Fatalf("expected 'typed_result', got %v", v)
	}
}

func TestRememberError(t *testing.T) {
	c := New()

	_, err := c.Remember("fail_key", 5*time.Minute, func() (any, error) {
		return nil, fmt.Errorf("test error")
	})
	if err == nil {
		t.Fatal("expected error from fn")
	}
	// 失败不缓存
	if _, ok := c.Get("fail_key"); ok {
		t.Fatal("failed fn should not cache")
	}
}

func TestIncrementDecrement(t *testing.T) {
	c := New()

	// 初始不存在
	n, err := c.Increment("counter")
	if err != nil || n != 1 {
		t.Fatalf("expected 1, got %d (err=%v)", n, err)
	}

	n, err = c.Increment("counter")
	if err != nil || n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}

	n, err = c.IncrementBy("counter", 10)
	if err != nil || n != 12 {
		t.Fatalf("expected 12, got %d", n)
	}

	n, err = c.Decrement("counter")
	if err != nil || n != 11 {
		t.Fatalf("expected 11, got %d", n)
	}

	n, err = c.DecrementBy("counter", 5)
	if err != nil || n != 6 {
		t.Fatalf("expected 6, got %d", n)
	}
}

func TestIncrementExpired(t *testing.T) {
	c := New()
	c.Set("exp_key", int64(42), 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	n, err := c.Increment("exp_key")
	if err != nil || n != 1 {
		t.Fatalf("expected 1 (reset on expired), got %d", n)
	}
}

func TestPull(t *testing.T) {
	c := New()
	c.Set("key", "value", 5*time.Minute)

	v, ok := c.Pull("key")
	if !ok || v != "value" {
		t.Fatalf("expected 'value', got %v", v)
	}

	// Pull 后 key 应不存在
	if _, ok := c.Get("key"); ok {
		t.Fatal("key should not exist after Pull")
	}
}

func TestPullFunc(t *testing.T) {
	c := New()
	c.Set("key", "hello", 5*time.Minute)

	v, ok := PullFunc[string](c, "key")
	if !ok || v != "hello" {
		t.Fatalf("expected 'hello', got %v", v)
	}

	if _, ok := PullFunc[string](c, "key"); ok {
		t.Fatal("second Pull should return false")
	}
}

func TestPullMissing(t *testing.T) {
	c := New()
	if _, ok := c.Pull("no_such_key"); ok {
		t.Fatal("Pull on missing key should return false")
	}
}

func TestSetTagIfNotExist(t *testing.T) {
	c := New()
	ts := NewTagSet(c)

	ok := ts.SetTagIfNotExist("key", "v1", 5*time.Minute, "tag_a")
	if !ok {
		t.Fatal("first SetTagIfNotExist should succeed")
	}

	ok = ts.SetTagIfNotExist("key", "v2", 5*time.Minute, "tag_b")
	if ok {
		t.Fatal("second SetTagIfNotExist should fail")
	}

	// 值不变
	if v, _ := c.Get("key"); v != "v1" {
		t.Fatalf("expected v1, got %v", v)
	}

	// 但 AppendTag 可以加标签
	ts.AppendTag("key", "tag_b")
	ts.FlushTag("tag_a")
	if _, ok := c.Get("key"); !ok {
		t.Fatal("key should still exist via tag_b")
	}
}
