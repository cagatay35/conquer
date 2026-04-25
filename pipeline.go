package conquer

import (
	"context"
	"sync"
)

// Stage defines a processing step in a pipeline. Each stage has a name
// (for observability), a transformation function, and a worker count
// controlling how many goroutines process items concurrently.
type Stage[In, Out any] struct {
	Name    string
	Fn      func(ctx context.Context, in In) (Out, error)
	Workers int
}

// NewStage creates a Stage with the given parameters. If workers <= 0,
// it defaults to 1.
func NewStage[In, Out any](name string, fn func(ctx context.Context, in In) (Out, error), workers int) Stage[In, Out] {
	if workers <= 0 {
		workers = 1
	}
	return Stage[In, Out]{Name: name, Fn: fn, Workers: workers}
}

// Pipeline2 connects a source channel through 2 processing stages,
// returning the output channel. All goroutines are managed internally;
// if any stage returns an error, remaining stages are cancelled via context.
//
// The output channel is closed when all stages complete or an error occurs.
// Errors are sent to the returned error channel (buffered, size 1).
func Pipeline2[A, B, C any](
	ctx context.Context,
	source <-chan A,
	s1 Stage[A, B],
	s2 Stage[B, C],
	opts ...Option,
) (<-chan C, <-chan error) {
	errCh := make(chan error, 1)
	ch1 := make(chan B, s1.Workers)
	out := make(chan C, s2.Workers)

	go func() {
		err := Run(ctx, func(s *Scope) error {
			pipeStage(s, source, ch1, s1)
			pipeStage(s, ch1, out, s2)
			return nil
		}, opts...)
		if err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	return out, errCh
}

// Pipeline3 connects a source through 3 processing stages.
func Pipeline3[A, B, C, D any](
	ctx context.Context,
	source <-chan A,
	s1 Stage[A, B],
	s2 Stage[B, C],
	s3 Stage[C, D],
	opts ...Option,
) (<-chan D, <-chan error) {
	errCh := make(chan error, 1)
	ch1 := make(chan B, s1.Workers)
	ch2 := make(chan C, s2.Workers)
	out := make(chan D, s3.Workers)

	go func() {
		err := Run(ctx, func(s *Scope) error {
			pipeStage(s, source, ch1, s1)
			pipeStage(s, ch1, ch2, s2)
			pipeStage(s, ch2, out, s3)
			return nil
		}, opts...)
		if err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	return out, errCh
}

// Pipeline4 connects a source through 4 processing stages.
func Pipeline4[A, B, C, D, E any](
	ctx context.Context,
	source <-chan A,
	s1 Stage[A, B],
	s2 Stage[B, C],
	s3 Stage[C, D],
	s4 Stage[D, E],
	opts ...Option,
) (<-chan E, <-chan error) {
	errCh := make(chan error, 1)
	ch1 := make(chan B, s1.Workers)
	ch2 := make(chan C, s2.Workers)
	ch3 := make(chan D, s3.Workers)
	out := make(chan E, s4.Workers)

	go func() {
		err := Run(ctx, func(s *Scope) error {
			pipeStage(s, source, ch1, s1)
			pipeStage(s, ch1, ch2, s2)
			pipeStage(s, ch2, ch3, s3)
			pipeStage(s, ch3, out, s4)
			return nil
		}, opts...)
		if err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	return out, errCh
}

// pipeStage launches workers for a single pipeline stage. Each worker reads
// from `in`, processes items via stage.Fn, and writes to `out`.
// A WaitGroup tracks the workers so `out` is closed exactly when all
// workers for this stage have finished.
func pipeStage[In, Out any](s *Scope, in <-chan In, out chan<- Out, stage Stage[In, Out]) {
	workers := stage.Workers
	if workers <= 0 {
		workers = 1
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		s.Go(func(ctx context.Context) error {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case item, ok := <-in:
					if !ok {
						return nil
					}
					result, err := stage.Fn(ctx, item)
					if err != nil {
						return err
					}
					select {
					case out <- result:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		})
	}

	s.Go(func(ctx context.Context) error {
		wg.Wait()
		close(out)
		return nil
	})
}
