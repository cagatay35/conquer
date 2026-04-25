// Package conquer provides structured concurrency primitives for Go.
//
// Structured concurrency ensures that every goroutine has an owner (a Scope)
// and that the owner waits for all goroutines to complete before returning.
// This eliminates goroutine leaks by construction, not by detection.
//
// Core primitives:
//
//   - [Run] / [Scope] : Scoped goroutine lifecycle management
//   - [Pool] / [ResultPool] : Bounded worker pools
//   - [Map] / [ForEach] / [Filter] / [First] : Concurrent iteration
//   - [Async] / [Future] : Async/await pattern
//   - [Pipeline2] .. [Pipeline5] : Multi-stage processing pipelines
//   - [FanOut] / [FanIn] / [Merge] : Channel fan-out/fan-in
//   - [BatchProcessor] : Batch processing with flush triggers
//
// The fundamental building block is [Run], which creates a [Scope]:
//
//	err := conquer.Run(ctx, func(s *conquer.Scope) error {
//	    s.Go(func(ctx context.Context) error {
//	        // runs in a managed goroutine
//	        return doWork(ctx)
//	    })
//	    s.Go(func(ctx context.Context) error {
//	        return doMoreWork(ctx)
//	    })
//	    return nil
//	})
//	// All goroutines are guaranteed to have completed here.
//
// Key guarantees:
//   - Goroutines never outlive their scope
//   - Panics are recovered and wrapped as [PanicError] with stack traces
//   - Errors propagate according to the configured [ErrorStrategy]
//   - Child scopes are cancelled when parent scopes cancel
package conquer
