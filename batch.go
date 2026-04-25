package conquer

import (
	"context"
	"sync"
	"time"
)

// BatchOption configures a BatchProcessor.
type BatchOption func(*batchOptions)

type batchOptions struct {
	maxSize     int
	maxInterval time.Duration
	maxBytes    int
	sizer       func(any) int
}

// BatchSize sets the maximum number of items before a flush is triggered.
func BatchSize(n int) BatchOption {
	return func(o *batchOptions) {
		o.maxSize = n
	}
}

// BatchInterval sets the maximum time between flushes. If the interval
// elapses and the batch is non-empty, it is flushed even if BatchSize
// has not been reached.
func BatchInterval(d time.Duration) BatchOption {
	return func(o *batchOptions) {
		o.maxInterval = d
	}
}

// BatchProcessor collects items and flushes them in batches.
// Flushes are triggered by size threshold, time interval, or explicit
// Flush() calls. The flush function is called with a snapshot of the
// accumulated items.
//
// BatchProcessor is safe for concurrent Add() calls.
type BatchProcessor[T any] struct {
	mu      sync.Mutex
	items   []T
	flush   func(ctx context.Context, batch []T) error
	opts    batchOptions
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	errOnce sync.Once
	err     error
}

// NewBatchProcessor creates a BatchProcessor that calls flush when
// a batch is ready. At least one of BatchSize or BatchInterval should
// be specified; otherwise items are only flushed via explicit Flush()
// or Close() calls.
func NewBatchProcessor[T any](
	ctx context.Context,
	flush func(ctx context.Context, batch []T) error,
	opts ...BatchOption,
) *BatchProcessor[T] {
	var bo batchOptions
	for _, o := range opts {
		o(&bo)
	}

	bpCtx, cancel := context.WithCancel(ctx)

	bp := &BatchProcessor[T]{
		flush:  flush,
		opts:   bo,
		ctx:    bpCtx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	if bo.maxSize > 0 {
		bp.items = make([]T, 0, bo.maxSize)
	}

	if bo.maxInterval > 0 {
		go bp.timerLoop()
	} else {
		close(bp.done)
	}

	return bp
}

// Add appends an item to the current batch. If the batch reaches
// BatchSize, an automatic flush is triggered synchronously.
// Returns an error if the processor has been closed or if a
// flush fails.
func (bp *BatchProcessor[T]) Add(item T) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	select {
	case <-bp.ctx.Done():
		return bp.ctx.Err()
	default:
	}

	bp.items = append(bp.items, item)

	if bp.opts.maxSize > 0 && len(bp.items) >= bp.opts.maxSize {
		return bp.flushLocked()
	}

	return nil
}

// Flush forces a flush of the current batch.
func (bp *BatchProcessor[T]) Flush(ctx context.Context) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.flushLocked()
}

// Close flushes any remaining items and stops the timer loop.
// After Close, further Add calls return an error.
func (bp *BatchProcessor[T]) Close() error {
	bp.cancel()

	select {
	case <-bp.done:
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	if len(bp.items) > 0 {
		return bp.flushWithContext(context.Background())
	}
	return bp.err
}

func (bp *BatchProcessor[T]) flushLocked() error {
	return bp.flushWithContext(bp.ctx)
}

func (bp *BatchProcessor[T]) flushWithContext(ctx context.Context) error {
	if len(bp.items) == 0 {
		return nil
	}

	batch := make([]T, len(bp.items))
	copy(batch, bp.items)
	bp.items = bp.items[:0]

	if err := bp.flush(ctx, batch); err != nil {
		bp.errOnce.Do(func() { bp.err = err })
		return err
	}
	return nil
}

func (bp *BatchProcessor[T]) timerLoop() {
	defer close(bp.done)

	ticker := time.NewTicker(bp.opts.maxInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bp.ctx.Done():
			return
		case <-ticker.C:
			bp.mu.Lock()
			_ = bp.flushLocked()
			bp.mu.Unlock()
		}
	}
}
