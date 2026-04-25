package conquer

import "context"

// Future represents a value that will be available asynchronously.
// It is created by Async and resolved by the goroutine managed by
// the parent Scope. Futures are safe for concurrent Await calls.
type Future[T any] struct {
	done chan struct{}
	val  T
	err  error
}

// Async launches fn in a goroutine within the given Scope and returns
// a Future that will hold the result. The goroutine's lifecycle is
// managed by the scope, so the future is guaranteed to resolve before
// the scope's Run returns.
//
//	err := conquer.Run(ctx, func(s *conquer.Scope) error {
//	    userF := conquer.Async(s, func(ctx context.Context) (*User, error) {
//	        return fetchUser(ctx, id)
//	    })
//	    ordersF := conquer.Async(s, func(ctx context.Context) ([]Order, error) {
//	        return fetchOrders(ctx, id)
//	    })
//
//	    user, err := userF.Await(ctx)
//	    if err != nil { return err }
//	    orders, err := ordersF.Await(ctx)
//	    if err != nil { return err }
//
//	    // use user and orders
//	    return nil
//	})
func Async[T any](s *Scope, fn func(ctx context.Context) (T, error)) *Future[T] {
	f := &Future[T]{
		done: make(chan struct{}),
	}

	s.Go(func(ctx context.Context) error {
		defer close(f.done)
		val, err := fn(ctx)
		f.val = val
		f.err = err
		return err
	})

	return f
}

// Await blocks until the future resolves or the context is cancelled.
// Returns the value and error from the async function, or the context
// error if the context is cancelled first.
func (f *Future[T]) Await(ctx context.Context) (T, error) {
	select {
	case <-f.done:
		return f.val, f.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// Done returns a channel that is closed when the future resolves.
// This is useful for select statements.
func (f *Future[T]) Done() <-chan struct{} {
	return f.done
}

// TryGet returns the result if the future has resolved, or false
// if it is still pending. Never blocks.
func (f *Future[T]) TryGet() (T, error, bool) {
	select {
	case <-f.done:
		return f.val, f.err, true
	default:
		var zero T
		return zero, nil, false
	}
}
