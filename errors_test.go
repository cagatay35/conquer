package conquer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPanicError_Error(t *testing.T) {
	pe := &PanicError{
		Value: "something went wrong",
		Stack: []byte("goroutine 1 [running]:\nmain.main()\n"),
	}

	msg := pe.Error()
	if !strings.Contains(msg, "something went wrong") {
		t.Errorf("expected panic value in message, got: %s", msg)
	}
	if !strings.Contains(msg, "goroutine 1") {
		t.Errorf("expected stack trace in message, got: %s", msg)
	}
}

func TestPanicError_Unwrap_WithError(t *testing.T) {
	inner := fmt.Errorf("inner error")
	pe := &PanicError{Value: inner, Stack: []byte("stack")}

	if !errors.Is(pe, inner) {
		t.Error("expected errors.Is to find inner error through PanicError")
	}

	var target *PanicError
	if !errors.As(pe, &target) {
		t.Error("errors.As should match PanicError")
	}
	if target.Value != inner {
		t.Error("errors.As should preserve the inner value")
	}
}

func TestPanicError_Unwrap_WithNonError(t *testing.T) {
	pe := &PanicError{Value: 42, Stack: []byte("stack")}

	if pe.Unwrap() != nil {
		t.Error("expected nil unwrap for non-error panic value")
	}
}

func TestMultiError_SingleError(t *testing.T) {
	inner := fmt.Errorf("single")
	me := &MultiError{Errors: []error{inner}}

	if me.Error() != "single" {
		t.Errorf("single error MultiError should delegate to inner, got: %s", me.Error())
	}
}

func TestMultiError_MultipleErrors(t *testing.T) {
	me := &MultiError{Errors: []error{
		fmt.Errorf("error 1"),
		fmt.Errorf("error 2"),
		fmt.Errorf("error 3"),
	}}

	msg := me.Error()
	if !strings.Contains(msg, "3 errors occurred") {
		t.Errorf("expected error count in message, got: %s", msg)
	}
	if !strings.Contains(msg, "error 1") || !strings.Contains(msg, "error 3") {
		t.Errorf("expected all errors in message, got: %s", msg)
	}
}

func TestMultiError_Unwrap(t *testing.T) {
	sentinel := fmt.Errorf("sentinel")
	me := &MultiError{Errors: []error{
		fmt.Errorf("other"),
		fmt.Errorf("wrap: %w", sentinel),
	}}

	if !errors.Is(me, sentinel) {
		t.Error("errors.Is should find sentinel through MultiError")
	}
}

func TestJoinErrors(t *testing.T) {
	t.Run("nil_errors", func(t *testing.T) {
		if err := joinErrors(nil); err != nil {
			t.Errorf("expected nil, got: %v", err)
		}
	})

	t.Run("all_nil", func(t *testing.T) {
		if err := joinErrors([]error{nil, nil}); err != nil {
			t.Errorf("expected nil, got: %v", err)
		}
	})

	t.Run("single_error", func(t *testing.T) {
		inner := fmt.Errorf("only")
		err := joinErrors([]error{nil, inner, nil})
		if err != inner {
			t.Errorf("expected inner error directly, got: %v", err)
		}
	})

	t.Run("multiple_errors", func(t *testing.T) {
		e1 := fmt.Errorf("e1")
		e2 := fmt.Errorf("e2")
		err := joinErrors([]error{nil, e1, nil, e2})

		var me *MultiError
		if !errors.As(err, &me) {
			t.Fatal("expected MultiError")
		}
		if len(me.Errors) != 2 {
			t.Errorf("expected 2 errors, got %d", len(me.Errors))
		}
	})
}
