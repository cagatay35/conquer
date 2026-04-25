package conquer

import "time"

// ErrorStrategy controls how a Scope handles errors from its goroutines.
type ErrorStrategy int

const (
	// FailFast cancels the scope's context on the first error,
	// signalling all other goroutines to stop.
	FailFast ErrorStrategy = iota

	// CollectAll collects all errors without cancelling the scope.
	// Useful when you want every goroutine to run to completion
	// regardless of individual failures.
	CollectAll
)

// Option configures a Scope, Pool, or concurrent operation.
type Option func(*options)

type options struct {
	maxGoroutines int
	errorStrategy ErrorStrategy
	timeout       time.Duration
	panicHandler  func(*PanicError)
	metrics       Metrics
	name          string
}

func defaultOptions() options {
	return options{
		maxGoroutines: 0, // unlimited
		errorStrategy: FailFast,
	}
}

func resolveOptions(opts []Option) options {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithMaxGoroutines limits the number of goroutines that can run
// concurrently within a scope. A value of 0 (default) means unlimited.
// Panics if n is negative.
func WithMaxGoroutines(n int) Option {
	if n < 0 {
		panic("conquer: WithMaxGoroutines requires a non-negative value")
	}
	return func(o *options) {
		o.maxGoroutines = n
	}
}

// WithErrorStrategy sets how the scope handles errors from goroutines.
// Default is FailFast.
func WithErrorStrategy(s ErrorStrategy) Option {
	return func(o *options) {
		o.errorStrategy = s
	}
}

// WithTimeout sets a deadline on the scope. If the timeout elapses,
// the scope's context is cancelled. All goroutines receive the
// cancellation signal through their context parameter.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
	}
}

// WithPanicHandler registers a callback invoked when a goroutine panics.
// The callback runs in the panicking goroutine after recovery, before
// error propagation. Useful for logging or alerting.
func WithPanicHandler(fn func(*PanicError)) Option {
	return func(o *options) {
		o.panicHandler = fn
	}
}

// WithMetrics attaches an observability sink to the scope.
func WithMetrics(m Metrics) Option {
	return func(o *options) {
		o.metrics = m
	}
}

// WithName sets a human-readable name for the scope, used in metrics
// and error messages.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}
