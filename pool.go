package conquer

import (
	"context"
	"sync"
)

// Pool is a bounded worker pool that manages goroutine lifecycles.
// It is a convenience wrapper around Scope with a Submit/Wait workflow,
// useful when tasks are submitted in a loop rather than all at once.
//
// All goroutines are guaranteed to complete when Wait returns.
type Pool struct {
	scope  *Scope
	cancel context.CancelCauseFunc
}

// NewPool creates a new worker pool. The pool's goroutines respect the
// given context for cancellation. Call Wait() to block until all
// submitted tasks complete.
//
// Common options: WithMaxGoroutines, WithErrorStrategy, WithPanicHandler.
func NewPool(ctx context.Context, opts ...Option) *Pool {
	o := resolveOptions(opts)

	m := o.metrics
	hasObs := m != nil || o.panicHandler != nil
	if m == nil {
		m = defaultMetrics
	}

	scopeCtx, cancel := context.WithCancelCause(ctx)

	s := &Scope{
		ctx:     scopeCtx,
		cancel:  cancel,
		opts:    o,
		metrics: m,
		hasObs:  hasObs,
	}

	if o.maxGoroutines > 0 {
		s.sem = newInternalSemaphore(o.maxGoroutines)
	}

	return &Pool{scope: s, cancel: cancel}
}

// Go submits a task to the pool. It may block if WithMaxGoroutines
// is set and all slots are occupied. Returns immediately if the pool
// has already been waited on.
func (p *Pool) Go(fn func(ctx context.Context) error) {
	p.scope.Go(fn)
}

// Wait blocks until all submitted goroutines complete and returns
// any errors. After Wait returns, further calls to Go are no-ops.
// Wait must be called exactly once.
func (p *Pool) Wait() error {
	p.scope.state.Store(scopeClosing)
	p.scope.wg.Wait()
	p.scope.state.Store(scopeClosed)
	p.cancel(nil)

	p.scope.mu.Lock()
	errs := p.scope.errs
	p.scope.mu.Unlock()

	return joinErrors(errs)
}

// ResultPool is a generic worker pool that collects typed results.
// Results are returned in submission order.
type ResultPool[T any] struct {
	pool    *Pool
	mu      sync.Mutex
	results []result[T]
}

type result[T any] struct {
	value T
	err   error
}

// NewResultPool creates a pool that collects typed return values.
func NewResultPool[T any](ctx context.Context, opts ...Option) *ResultPool[T] {
	return &ResultPool[T]{
		pool: NewPool(ctx, opts...),
	}
}

// Go submits a task that produces a value. The result is stored
// in submission order so that Wait returns results in the same
// order they were submitted.
func (rp *ResultPool[T]) Go(fn func(ctx context.Context) (T, error)) {
	rp.mu.Lock()
	idx := len(rp.results)
	rp.results = append(rp.results, result[T]{})
	rp.mu.Unlock()

	rp.pool.Go(func(ctx context.Context) error {
		val, err := fn(ctx)
		rp.mu.Lock()
		rp.results[idx] = result[T]{value: val, err: err}
		rp.mu.Unlock()
		return err
	})
}

// Wait blocks until all tasks complete and returns the collected
// results in submission order along with any errors.
func (rp *ResultPool[T]) Wait() ([]T, error) {
	err := rp.pool.Wait()

	values := make([]T, len(rp.results))
	for i, r := range rp.results {
		values[i] = r.value
	}
	return values, err
}
