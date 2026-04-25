// Package panicutil provides utilities for capturing and formatting
// panic stack traces within conquer's goroutine management.
package panicutil

import "runtime"

const maxStackSize = 4096

// CaptureStack returns the current goroutine's stack trace,
// skipping the specified number of frames from the top.
// The returned byte slice is a snapshot that can be safely stored.
func CaptureStack(skip int) []byte {
	buf := make([]byte, maxStackSize)
	n := runtime.Stack(buf, false)
	return buf[:n]
}
