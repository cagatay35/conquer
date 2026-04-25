package conquer

import (
	"context"
	"sync/atomic"
)

// Map applies fn to each element of items concurrently and returns the
// results in the same order as the input. If any invocation returns an
// error and the ErrorStrategy is FailFast (default), remaining goroutines
// are cancelled via context.
//
//	users, err := conquer.Map(ctx, userIDs, func(ctx context.Context, id int64) (*User, error) {
//	    return userService.Get(ctx, id)
//	}, conquer.WithMaxGoroutines(10))
func Map[T, R any](ctx context.Context, items []T, fn func(ctx context.Context, item T) (R, error), opts ...Option) ([]R, error) {
	if len(items) == 0 {
		return nil, nil
	}

	results := make([]R, len(items))

	err := Run(ctx, func(s *Scope) error {
		for i, item := range items {
			i, item := i, item
			s.Go(func(ctx context.Context) error {
				result, err := fn(ctx, item)
				if err != nil {
					return err
				}
				results[i] = result
				return nil
			})
		}
		return nil
	}, opts...)

	if err != nil {
		return nil, err
	}
	return results, nil
}

// ForEach applies fn to each element concurrently. It is equivalent to
// Map but discards return values, suitable for side-effect-only operations.
func ForEach[T any](ctx context.Context, items []T, fn func(ctx context.Context, item T) error, opts ...Option) error {
	if len(items) == 0 {
		return nil
	}

	return Run(ctx, func(s *Scope) error {
		for i := range items {
			item := items[i]
			s.Go(func(ctx context.Context) error {
				return fn(ctx, item)
			})
		}
		return nil
	}, opts...)
}

// Filter returns elements for which fn returns true. The predicate
// is evaluated concurrently. Result order matches input order.
func Filter[T any](ctx context.Context, items []T, fn func(ctx context.Context, item T) (bool, error), opts ...Option) ([]T, error) {
	if len(items) == 0 {
		return nil, nil
	}

	keep := make([]bool, len(items))

	err := Run(ctx, func(s *Scope) error {
		for i, item := range items {
			i, item := i, item
			s.Go(func(ctx context.Context) error {
				ok, err := fn(ctx, item)
				if err != nil {
					return err
				}
				keep[i] = ok
				return nil
			})
		}
		return nil
	}, opts...)

	if err != nil {
		return nil, err
	}

	var filtered []T
	for i, item := range items {
		if keep[i] {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

// First runs fn concurrently on all items and returns the first
// successful (non-error) result. Once a result is found, remaining
// goroutines are cancelled. Returns the zero value and an error if
// all invocations fail.
func First[T, R any](ctx context.Context, items []T, fn func(ctx context.Context, item T) (R, error), opts ...Option) (R, error) {
	if len(items) == 0 {
		var zero R
		return zero, nil
	}

	type firstResult struct {
		val R
		ok  bool
	}

	var found atomic.Bool
	var res firstResult

	forcedOpts := append([]Option{WithErrorStrategy(CollectAll)}, opts...)

	err := Run(ctx, func(s *Scope) error {
		ctx := s.ctx

		innerCtx, innerCancel := context.WithCancelCause(ctx)
		defer innerCancel(nil)

		for _, item := range items {
			item := item
			s.Go(func(_ context.Context) error {
				if found.Load() {
					return nil
				}
				val, err := fn(innerCtx, item)
				if err != nil {
					return err
				}
				if found.CompareAndSwap(false, true) {
					res = firstResult{val: val, ok: true}
					innerCancel(nil)
				}
				return nil
			})
		}
		return nil
	}, forcedOpts...)

	if res.ok {
		return res.val, nil
	}

	var zero R
	if err != nil {
		return zero, err
	}
	return zero, nil
}

// Reduce concurrently processes all items and then reduces the results
// sequentially using the accumulator function. Processing of items is
// concurrent; the reduction step is sequential to maintain determinism.
func Reduce[T, R any](ctx context.Context, items []T, initial R, mapFn func(ctx context.Context, item T) (R, error), reduceFn func(a, b R) R, opts ...Option) (R, error) {
	if len(items) == 0 {
		return initial, nil
	}

	mapped, err := Map(ctx, items, mapFn, opts...)
	if err != nil {
		return initial, err
	}

	acc := initial
	for _, v := range mapped {
		acc = reduceFn(acc, v)
	}
	return acc, nil
}
