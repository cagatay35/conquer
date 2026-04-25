// Package taskpool provides a global sync.Pool-based object recycler for
// goroutine task wrappers, reducing GC pressure in hot paths.
package taskpool

import (
	"context"
	"sync"
)

// Task represents a unit of work to be executed in a goroutine.
// Fields are set before launching and cleared on return to the pool.
type Task struct {
	Fn func(ctx context.Context) error
}

// Reset clears all fields to prevent stale data leaks when the
// task is returned to the pool. This is critical for security.
func (t *Task) Reset() {
	t.Fn = nil
}

// Global is the package-level singleton pool. Using a global pool avoids
// allocating a new sync.Pool per Scope, which was a major overhead source.
var Global = sync.Pool{
	New: func() any { return &Task{} },
}

// Get retrieves a zeroed Task from the global pool.
func Get() *Task {
	return Global.Get().(*Task)
}

// Put returns a Task to the global pool after resetting it.
func Put(t *Task) {
	t.Reset()
	Global.Put(t)
}
