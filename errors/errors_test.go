package errors

import (
	"errors"
	"testing"
)

func TestHasCode(t *testing.T) {
	// 直接匹配预设错误
	if !HasCode(ErrNotFound, CodeNotFound) {
		t.Fatal("HasCode should match ErrNotFound")
	}
	if HasCode(ErrNotFound, CodeInternal) {
		t.Fatal("HasCode should not mismatch")
	}
	// nil / empty
	if HasCode(nil, "X") {
		t.Fatal("nil error should return false")
	}
	if HasCode(ErrNotFound, "") {
		t.Fatal("empty code should return false")
	}

	// Wrap 链：HasCode 遍历整条错误链
	chained := ErrInternal.Wrap(ErrNotFound.Wrap(errors.New("root cause")))
	if !HasCode(chained, CodeInternal) {
		t.Fatal("HasCode should match outermost INTERNAL_ERROR")
	}
	if !HasCode(chained, CodeNotFound) {
		t.Fatal("HasCode should walk chain and match NOT_FOUND")
	}
	if HasCode(chained, "UNKNOWN_CODE") {
		t.Fatal("HasCode should not match unknown code")
	}

	// 非 *Error 也递归 Unwrap
	plainWrapped := errors.New("plain") // 普通 error
	if HasCode(plainWrapped, "X") {
		t.Fatal("plain error should return false")
	}

	// Wrap plain error
	wrapped := ErrValidation.Wrap(plainWrapped)
	if !HasCode(wrapped, CodeValidation) {
		t.Fatal("HasCode should find code even with plain cause")
	}
}

func TestCodeOf(t *testing.T) {
	if CodeOf(nil) != "" {
		t.Fatal("nil → empty string")
	}
	if CodeOf(ErrNotFound) != CodeNotFound {
		t.Fatalf("got %q", CodeOf(ErrNotFound))
	}
	chained := ErrInternal.Wrap(ErrNotFound)
	if CodeOf(chained) != CodeInternal {
		t.Fatalf("should return outermost code, got %q", CodeOf(chained))
	}
}

func TestStatusOf(t *testing.T) {
	if StatusOf(nil) != 200 {
		t.Fatal("nil → 200")
	}
	if StatusOf(ErrNotFound) != 404 {
		t.Fatalf("got %d", StatusOf(ErrNotFound))
	}
	if StatusOf(NewError(0, "X", "msg")) != 500 {
		t.Fatal("zero status → 500")
	}
}

func TestChain(t *testing.T) {
	inner := errors.New("inner error")
	chained := ErrValidation.Wrap(ErrNotFound.Wrap(inner))

	// errors.Is 基于 Code 比较
	if !errors.Is(chained, ErrNotFound) {
		t.Fatal("chained should Is ErrNotFound")
	}
	if !errors.Is(chained, ErrValidation) {
		t.Fatal("chained should Is ErrValidation")
	}

	// errors.As 提取
	var e *Error
	if !errors.As(chained, &e) {
		t.Fatal("should extract *Error")
	}
	if e.Code != CodeValidation {
		t.Fatalf("outer Code should be VALIDATION_FAILED, got %q", e.Code)
	}

	// Error() 字符串包含完整链
	msg := chained.Error()
	if msg == "" {
		t.Fatal("empty error string")
	}
}

func TestWrapPreservesOriginal(t *testing.T) {
	// 包级变量不应被 Wrap 污染
	wrapped := ErrNotFound.Wrap(errors.New("cause"))
	if ErrNotFound.cause != nil {
		t.Fatal("original ErrNotFound should not have cause")
	}
	_ = wrapped
}
