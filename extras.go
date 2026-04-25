package conquer

import (
	"context"
	"sync"
	"time"
)

// Tee splits a single input channel into N output channels.
// Every item from input is sent to all output channels.
// All output channels are closed when the input channel closes.
//
//	ch1, ch2 := conquer.Tee2(ctx, source)
func Tee2[T any](ctx context.Context, input <-chan T) (<-chan T, <-chan T) {
	out1 := make(chan T)
	out2 := make(chan T)

	go func() {
		defer close(out1)
		defer close(out2)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-input:
				if !ok {
					return
				}
				select {
				case out1 <- v:
				case <-ctx.Done():
					return
				}
				select {
				case out2 <- v:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out1, out2
}

// TeeN splits a single input channel into n output channels.
func TeeN[T any](ctx context.Context, input <-chan T, n int) []<-chan T {
	outputs := make([]chan T, n)
	readOnly := make([]<-chan T, n)
	for i := range outputs {
		outputs[i] = make(chan T)
		readOnly[i] = outputs[i]
	}

	go func() {
		defer func() {
			for _, ch := range outputs {
				close(ch)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-input:
				if !ok {
					return
				}
				for _, ch := range outputs {
					select {
					case ch <- v:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return readOnly
}

// Throttle wraps fn so that it executes at most once per interval.
// Calls during the cooldown period block until the next window opens.
// Useful for rate-limited API calls.
//
//	throttled := conquer.Throttle(100*time.Millisecond, callAPI)
//	result, err := throttled(ctx, request)
func Throttle[In, Out any](interval time.Duration, fn func(ctx context.Context, in In) (Out, error)) func(ctx context.Context, in In) (Out, error) {
	var mu sync.Mutex
	var lastCall time.Time

	return func(ctx context.Context, in In) (Out, error) {
		mu.Lock()
		elapsed := time.Since(lastCall)
		if elapsed < interval {
			wait := interval - elapsed
			mu.Unlock()
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				var zero Out
				return zero, ctx.Err()
			}
			mu.Lock()
		}
		lastCall = time.Now()
		mu.Unlock()

		return fn(ctx, in)
	}
}

// Debounce returns a function that delays execution of fn until after
// the specified duration has elapsed since the last call. If called
// again before the duration elapses, the timer resets. Only the last
// call's input is used.
func Debounce[In, Out any](delay time.Duration, fn func(ctx context.Context, in In) (Out, error)) func(ctx context.Context, in In) <-chan Result[Out] {
	var mu sync.Mutex
	var timer *time.Timer

	return func(ctx context.Context, in In) <-chan Result[Out] {
		ch := make(chan Result[Out], 1)
		mu.Lock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, func() {
			val, err := fn(ctx, in)
			ch <- Result[Out]{Value: val, Err: err}
			close(ch)
		})
		mu.Unlock()
		return ch
	}
}

// Chunk splits a slice into chunks of the given size.
// Useful for batch processing:
//
//	for _, chunk := range conquer.Chunk(items, 100) {
//	    pool.Go(func(ctx context.Context) error { return processBatch(ctx, chunk) })
//	}
func Chunk[T any](items []T, size int) [][]T {
	if size <= 0 {
		panic("conquer: Chunk size must be positive")
	}
	if len(items) == 0 {
		return nil
	}

	chunks := make([][]T, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// MapChunked splits items into chunks and processes each chunk concurrently.
// Within each chunk, items are processed sequentially. This is useful for
// batch APIs or when you want to limit the granularity of concurrency.
//
//	results, err := conquer.MapChunked(ctx, userIDs, 50,
//	    func(ctx context.Context, ids []int64) ([]*User, error) {
//	        return userService.BatchGet(ctx, ids)
//	    })
func MapChunked[T, R any](ctx context.Context, items []T, chunkSize int, fn func(ctx context.Context, chunk []T) ([]R, error), opts ...Option) ([]R, error) {
	chunks := Chunk(items, chunkSize)
	if len(chunks) == 0 {
		return nil, nil
	}

	chunkResults, err := Map(ctx, chunks, fn, opts...)
	if err != nil {
		return nil, err
	}

	total := 0
	for _, cr := range chunkResults {
		total += len(cr)
	}

	results := make([]R, 0, total)
	for _, cr := range chunkResults {
		results = append(results, cr...)
	}
	return results, nil
}

// Collect reads all values from a channel into a slice.
// Blocks until the channel is closed or context is cancelled.
func Collect[T any](ctx context.Context, ch <-chan T) []T {
	var items []T
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return items
			}
			items = append(items, v)
		case <-ctx.Done():
			return items
		}
	}
}

// SendAll sends all items from a slice to a channel. The channel is NOT
// closed after sending. Respects context cancellation.
func SendAll[T any](ctx context.Context, ch chan<- T, items []T) error {
	for _, item := range items {
		select {
		case ch <- item:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
