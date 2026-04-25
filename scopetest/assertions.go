package scopetest

import (
	"errors"
	"testing"

	"github.com/cagatay35/conquer"
)

// AssertPanic asserts that the error contains a recovered panic.
func AssertPanic(t testing.TB, err error) *conquer.PanicError {
	t.Helper()
	var pe *conquer.PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got: %T: %v", err, err)
	}
	return pe
}

// AssertNoPanic asserts that the error does not contain a panic.
func AssertNoPanic(t testing.TB, err error) {
	t.Helper()
	var pe *conquer.PanicError
	if errors.As(err, &pe) {
		t.Fatalf("unexpected PanicError: %v", pe)
	}
}

// AssertMultiError asserts that the error is a MultiError with the
// expected number of contained errors.
func AssertMultiError(t testing.TB, err error, expectedCount int) *conquer.MultiError {
	t.Helper()
	var me *conquer.MultiError
	if !errors.As(err, &me) {
		t.Fatalf("expected MultiError, got: %T: %v", err, err)
	}
	if len(me.Errors) != expectedCount {
		t.Fatalf("expected %d errors in MultiError, got %d", expectedCount, len(me.Errors))
	}
	return me
}
