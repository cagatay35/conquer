package conquer

import (
	"context"
	"sync"
)

// Result wraps a value with its associated error, used by channel-based
// operations where errors need to flow alongside data.
type Result[T any] struct {
	Value T
	Err   error
}

// FanOut distributes items from the input channel across n worker
// goroutines, each applying fn, and merges results into a single
// output channel. Results may arrive out of order.
//
// The output channel is closed when all workers complete.
// Panics in workers are recovered and returned as Result.Err.
func FanOut[T, R any](ctx context.Context, input <-chan T, workers int, fn func(ctx context.Context, item T) (R, error), opts ...Option) <-chan Result[R] {
	if workers <= 0 {
		workers = 1
	}

	out := make(chan Result[R], workers)

	go func() {
		_ = Run(ctx, func(s *Scope) error {
			for i := 0; i < workers; i++ {
				s.Go(func(ctx context.Context) error {
					for {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case item, ok := <-input:
							if !ok {
								return nil
							}
							val, err := fn(ctx, item)
							select {
							case out <- Result[R]{Value: val, Err: err}:
							case <-ctx.Done():
								return ctx.Err()
							}
						}
					}
				})
			}
			return nil
		}, opts...)
		close(out)
	}()

	return out
}

// FanIn merges multiple input channels into a single output channel.
// The output channel is closed when all input channels are closed.
// Order is non-deterministic.
func FanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T {
	out := make(chan T, len(channels))

	var wg sync.WaitGroup
	wg.Add(len(channels))

	for _, ch := range channels {
		ch := ch
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case v, ok := <-ch:
					if !ok {
						return
					}
					select {
					case out <- v:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// Merge is an alias for FanIn that merges multiple channels into one.
func Merge[T any](ctx context.Context, channels ...<-chan T) <-chan T {
	return FanIn(ctx, channels...)
}

// Generate creates a channel that emits values produced by fn.
// fn is called repeatedly until it returns false or the context
// is cancelled. The returned channel is closed when generation stops.
func Generate[T any](ctx context.Context, fn func(ctx context.Context) (T, bool)) <-chan T {
	out := make(chan T)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				val, ok := fn(ctx)
				if !ok {
					return
				}
				select {
				case out <- val:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// Drain reads all values from a channel of Results, collecting
// values and errors separately. Blocks until the channel is closed.
func Drain[T any](ch <-chan Result[T]) ([]T, error) {
	var values []T
	var errs []error

	for r := range ch {
		if r.Err != nil {
			errs = append(errs, r.Err)
		} else {
			values = append(values, r.Value)
		}
	}

	return values, joinErrors(errs)
}
