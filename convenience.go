package conquer

import (
	"context"
	"time"
)

// Go is a convenience function that runs a single goroutine within
// a managed scope. It is the simplest way to run a goroutine with
// panic recovery and guaranteed completion.
//
//	err := conquer.Go(ctx, func(ctx context.Context) error {
//	    return doWork(ctx)
//	})
func Go(ctx context.Context, fn func(ctx context.Context) error) error {
	return Run(ctx, func(s *Scope) error {
		s.Go(fn)
		return nil
	})
}

// Go2 runs two goroutines concurrently and waits for both to complete.
//
//	err := conquer.Go2(ctx, fetchUser, fetchOrders)
func Go2(ctx context.Context, fn1, fn2 func(ctx context.Context) error) error {
	return Run(ctx, func(s *Scope) error {
		s.Go(fn1)
		s.Go(fn2)
		return nil
	})
}

// Go3 runs three goroutines concurrently and waits for all to complete.
func Go3(ctx context.Context, fn1, fn2, fn3 func(ctx context.Context) error) error {
	return Run(ctx, func(s *Scope) error {
		s.Go(fn1)
		s.Go(fn2)
		s.Go(fn3)
		return nil
	})
}

// GoN runs N goroutines concurrently and waits for all to complete.
//
//	fns := make([]func(ctx context.Context) error, len(urls))
//	for i, url := range urls {
//	    url := url
//	    fns[i] = func(ctx context.Context) error { return fetch(ctx, url) }
//	}
//	err := conquer.GoN(ctx, fns...)
func GoN(ctx context.Context, fns ...func(ctx context.Context) error) error {
	return Run(ctx, func(s *Scope) error {
		for _, fn := range fns {
			s.Go(fn)
		}
		return nil
	})
}

// Race runs all functions concurrently and returns the result of the
// first one to complete successfully. Remaining goroutines are cancelled.
// If all functions fail, returns the collected errors.
//
//	result, err := conquer.Race(ctx,
//	    func(ctx context.Context) (string, error) { return fetchFromCDN1(ctx) },
//	    func(ctx context.Context) (string, error) { return fetchFromCDN2(ctx) },
//	    func(ctx context.Context) (string, error) { return fetchFromCDN3(ctx) },
//	)
func Race[T any](ctx context.Context, fns ...func(ctx context.Context) (T, error)) (T, error) {
	if len(fns) == 0 {
		var zero T
		return zero, nil
	}
	if len(fns) == 1 {
		return fns[0](ctx)
	}

	type raceResult struct {
		val T
		ok  bool
	}

	resultCh := make(chan raceResult, 1)
	errCh := make(chan error, len(fns))

	raceCtx, raceCancel := context.WithCancel(ctx)
	defer raceCancel()

	err := Run(raceCtx, func(s *Scope) error {
		for _, fn := range fns {
			fn := fn
			s.Go(func(ctx context.Context) error {
				val, err := fn(ctx)
				if err != nil {
					errCh <- err
					return nil
				}
				select {
				case resultCh <- raceResult{val: val, ok: true}:
					raceCancel()
				default:
				}
				return nil
			})
		}
		return nil
	}, WithErrorStrategy(CollectAll))

	select {
	case r := <-resultCh:
		if r.ok {
			return r.val, nil
		}
	default:
	}

	var zero T
	if err != nil {
		return zero, err
	}

	close(errCh)
	var errs []error
	for e := range errCh {
		errs = append(errs, e)
	}
	return zero, joinErrors(errs)
}

// Retry executes fn up to maxAttempts times with the given delay between
// attempts. Returns the first successful result or the last error.
// The delay is multiplied by 2 after each attempt (exponential backoff).
//
//	result, err := conquer.Retry(ctx, 3, 100*time.Millisecond,
//	    func(ctx context.Context) (*Response, error) {
//	        return httpClient.Get(ctx, url)
//	    })
func Retry[T any](ctx context.Context, maxAttempts int, initialDelay time.Duration, fn func(ctx context.Context) (T, error)) (T, error) {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt < maxAttempts; attempt++ {
		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		lastErr = err

		if attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			case <-time.After(delay):
				delay *= 2
			}
		}
	}

	var zero T
	return zero, lastErr
}

// RetryWithFn is like Retry but allows a custom backoff strategy.
//
//	result, err := conquer.RetryWithFn(ctx, 5,
//	    func(attempt int) time.Duration { return time.Duration(attempt) * time.Second },
//	    func(ctx context.Context) (int, error) { return callAPI(ctx) },
//	)
func RetryWithFn[T any](ctx context.Context, maxAttempts int, backoff func(attempt int) time.Duration, fn func(ctx context.Context) (T, error)) (T, error) {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		lastErr = err

		if attempt < maxAttempts-1 {
			d := backoff(attempt)
			if d > 0 {
				select {
				case <-ctx.Done():
					var zero T
					return zero, ctx.Err()
				case <-time.After(d):
				}
			}
		}
	}

	var zero T
	return zero, lastErr
}
