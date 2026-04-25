// Package scopetest provides test utilities for conquer-based code.
// It includes a goroutine leak detector and assertion helpers.
package scopetest

import (
	"runtime"
	"testing"
	"time"
)

// CheckLeaks verifies that no goroutines were leaked during a test.
// Call it with defer at the start of your test:
//
//	func TestSomething(t *testing.T) {
//	    defer scopetest.CheckLeaks(t)
//	    // ... test code using conquer ...
//	}
//
// It records the goroutine count before the test and checks it after,
// allowing a small tolerance for runtime goroutines.
func CheckLeaks(t testing.TB) {
	t.Helper()

	before := runtime.NumGoroutine()

	t.Cleanup(func() {
		t.Helper()

		deadline := time.Now().Add(2 * time.Second)

		for time.Now().Before(deadline) {
			runtime.GC()
			after := runtime.NumGoroutine()

			leaked := after - before
			if leaked <= 2 {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}

		after := runtime.NumGoroutine()
		leaked := after - before
		if leaked > 2 {
			buf := make([]byte, 1<<16)
			n := runtime.Stack(buf, true)
			t.Errorf("goroutine leak detected: %d goroutines leaked\n\nActive goroutines:\n%s", leaked, buf[:n])
		}
	})
}
