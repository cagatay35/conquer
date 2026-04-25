// Package semaphore provides a context-aware weighted semaphore used
// internally by conquer to limit goroutine concurrency.
package semaphore

import "context"

// Weighted is a counting semaphore that respects context cancellation.
// It is lighter than x/sync/semaphore because conquer only ever
// acquires/releases weight=1, allowing a simpler channel-based design.
type Weighted struct {
	ch chan struct{}
}

// New creates a Weighted semaphore with the given maximum capacity.
// Panics if n <= 0.
func New(n int) *Weighted {
	if n <= 0 {
		panic("semaphore: capacity must be positive")
	}
	return &Weighted{
		ch: make(chan struct{}, n),
	}
}

// Acquire blocks until a slot is available or the context is cancelled.
// Returns the context's error if cancelled while waiting.
func (s *Weighted) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	default:
	}

	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to acquire a slot without blocking.
// Returns true if successful, false otherwise.
func (s *Weighted) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases a slot back to the semaphore.
// Must be called exactly once for each successful Acquire.
func (s *Weighted) Release() {
	select {
	case <-s.ch:
	default:
		panic("semaphore: release without acquire")
	}
}
