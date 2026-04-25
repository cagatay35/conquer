package conquer

import "time"

// Metrics is the observability interface for conquer scopes.
// Implement this interface to integrate with your metrics system
// (Prometheus, OpenTelemetry, etc.).
//
// All methods must be safe for concurrent use.
type Metrics interface {
	// GoroutineStarted is called when a goroutine begins execution.
	GoroutineStarted(scope string)

	// GoroutineCompleted is called when a goroutine finishes.
	// duration is wall-clock time. err is nil on success.
	GoroutineCompleted(scope string, duration time.Duration, err error)

	// GoroutinePanicked is called when a goroutine panics.
	// This is called before GoroutineCompleted.
	GoroutinePanicked(scope string, pe *PanicError)

	// ScopeCreated is called when a new scope is created via Run or Child.
	ScopeCreated(name string)

	// ScopeClosed is called when a scope completes and all goroutines
	// have finished. goroutineCount is the total number of goroutines
	// that were launched in this scope.
	ScopeClosed(name string, duration time.Duration, goroutineCount int64)
}

// noopMetrics is used when no metrics are configured. The compiler
// can inline and eliminate these empty method calls.
type noopMetrics struct{}

func (noopMetrics) GoroutineStarted(string)                             {}
func (noopMetrics) GoroutineCompleted(string, time.Duration, error)     {}
func (noopMetrics) GoroutinePanicked(string, *PanicError)               {}
func (noopMetrics) ScopeCreated(string)                                 {}
func (noopMetrics) ScopeClosed(string, time.Duration, int64)            {}

var defaultMetrics Metrics = noopMetrics{}
