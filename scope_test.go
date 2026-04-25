package conquer

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestRun_BasicExecution(t *testing.T) {
	var executed atomic.Bool

	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			executed.Store(true)
			return nil
		})
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed.Load() {
		t.Fatal("goroutine was not executed")
	}
}

func TestRun_MultipleGoroutines(t *testing.T) {
	const n = 100
	var count atomic.Int64

	err := Run(context.Background(), func(s *Scope) error {
		for i := 0; i < n; i++ {
			s.Go(func(ctx context.Context) error {
				count.Add(1)
				return nil
			})
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != n {
		t.Fatalf("expected %d goroutines, got %d", n, count.Load())
	}
}

func TestRun_FnError(t *testing.T) {
	sentinel := fmt.Errorf("fn error")

	err := Run(context.Background(), func(s *Scope) error {
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
}

func TestRun_GoroutineError_FailFast(t *testing.T) {
	sentinel := fmt.Errorf("goroutine error")

	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			return sentinel
		})
		return nil
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
}

func TestRun_GoroutineError_CollectAll(t *testing.T) {
	e1 := fmt.Errorf("error 1")
	e2 := fmt.Errorf("error 2")

	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			return e1
		})
		s.Go(func(ctx context.Context) error {
			return e2
		})
		return nil
	}, WithErrorStrategy(CollectAll))

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, e1) {
		t.Errorf("expected e1 in errors, got: %v", err)
	}
	if !errors.Is(err, e2) {
		t.Errorf("expected e2 in errors, got: %v", err)
	}
}

func TestRun_PanicRecovery(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			panic("boom")
		})
		return nil
	})

	if err == nil {
		t.Fatal("expected error from panic")
	}

	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got: %T: %v", err, err)
	}
	if pe.Value != "boom" {
		t.Errorf("expected panic value 'boom', got: %v", pe.Value)
	}
	if len(pe.Stack) == 0 {
		t.Error("expected stack trace")
	}
}

func TestRun_PanicHandler(t *testing.T) {
	var captured *PanicError

	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			panic("captured panic")
		})
		return nil
	}, WithPanicHandler(func(pe *PanicError) {
		captured = pe
	}))

	if err == nil {
		t.Fatal("expected error")
	}
	if captured == nil {
		t.Fatal("panic handler was not called")
	}
	if captured.Value != "captured panic" {
		t.Errorf("wrong panic value: %v", captured.Value)
	}
}

func TestRun_FnPanicRecovery(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		panic("fn panic")
	})

	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError from fn panic, got: %T: %v", err, err)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var ctxErr atomic.Value

	err := Run(ctx, func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			cancel()
			<-ctx.Done()
			ctxErr.Store(ctx.Err())
			return ctx.Err()
		})
		return nil
	})

	if err == nil {
		t.Fatal("expected error from cancellation")
	}

	stored := ctxErr.Load()
	if stored == nil {
		t.Fatal("expected context error to be stored")
	}
}

func TestRun_Timeout(t *testing.T) {
	start := time.Now()

	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		})
		return nil
	}, WithTimeout(50*time.Millisecond))

	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestRun_MaxGoroutines(t *testing.T) {
	const maxG = 5
	const total = 50
	var peak atomic.Int64
	var current atomic.Int64

	err := Run(context.Background(), func(s *Scope) error {
		for i := 0; i < total; i++ {
			s.Go(func(ctx context.Context) error {
				cur := current.Add(1)
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(time.Millisecond)
				current.Add(-1)
				return nil
			})
		}
		return nil
	}, WithMaxGoroutines(maxG))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if peak.Load() > int64(maxG) {
		t.Fatalf("peak concurrency %d exceeded max %d", peak.Load(), maxG)
	}
}

func TestRun_FailFast_CancelsOthers(t *testing.T) {
	sentinel := fmt.Errorf("fail fast trigger")
	var cancelled atomic.Bool

	_ = Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			return sentinel
		})
		s.Go(func(ctx context.Context) error {
			<-ctx.Done()
			cancelled.Store(true)
			return nil
		})
		return nil
	})

	time.Sleep(10 * time.Millisecond)
	if !cancelled.Load() {
		t.Error("expected second goroutine to be cancelled via context")
	}
}

func TestRun_NilFn(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		s.Go(nil)
		return nil
	})

	if err != nil {
		t.Fatalf("nil fn should be a no-op, got error: %v", err)
	}
}

func TestScope_Child(t *testing.T) {
	var parentDone, childDone atomic.Bool

	err := Run(context.Background(), func(s *Scope) error {
		s.Child(func(cs *Scope) error {
			cs.Go(func(ctx context.Context) error {
				childDone.Store(true)
				return nil
			})
			return nil
		})
		s.Go(func(ctx context.Context) error {
			parentDone.Store(true)
			return nil
		})
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parentDone.Load() {
		t.Error("parent goroutine did not run")
	}
	if !childDone.Load() {
		t.Error("child goroutine did not run")
	}
}

func TestScope_Child_ParentCancelCascades(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var childCancelled atomic.Bool

	_ = Run(ctx, func(s *Scope) error {
		s.Child(func(cs *Scope) error {
			cs.Go(func(ctx context.Context) error {
				cancel()
				<-ctx.Done()
				childCancelled.Store(true)
				return ctx.Err()
			})
			return nil
		})
		return nil
	})

	if !childCancelled.Load() {
		t.Error("child goroutine should have been cancelled when parent was cancelled")
	}
}

func TestRun_NoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		_ = Run(context.Background(), func(s *Scope) error {
			for j := 0; j < 100; j++ {
				s.Go(func(ctx context.Context) error {
					return nil
				})
			}
			return nil
		})
	}

	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()

	leaked := after - before
	if leaked > 5 {
		t.Errorf("potential goroutine leak: before=%d after=%d leaked=%d", before, after, leaked)
	}
}

func TestRun_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const iterations = 100
	const goroutinesPerIteration = 1000

	for i := 0; i < iterations; i++ {
		var sum atomic.Int64

		err := Run(context.Background(), func(s *Scope) error {
			for j := 0; j < goroutinesPerIteration; j++ {
				s.Go(func(ctx context.Context) error {
					sum.Add(1)
					return nil
				})
			}
			return nil
		})

		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if sum.Load() != goroutinesPerIteration {
			t.Fatalf("iteration %d: expected %d, got %d", i, goroutinesPerIteration, sum.Load())
		}
	}
}

func TestRun_CombinedFnAndGoroutineErrors(t *testing.T) {
	fnErr := fmt.Errorf("fn error")
	goErr := fmt.Errorf("goroutine error")

	err := Run(context.Background(), func(s *Scope) error {
		s.Go(func(ctx context.Context) error {
			time.Sleep(5 * time.Millisecond)
			return goErr
		})
		return fnErr
	}, WithErrorStrategy(CollectAll))

	if err == nil {
		t.Fatal("expected errors")
	}
	if !errors.Is(err, fnErr) {
		t.Errorf("expected fn error, got: %v", err)
	}
}

func TestRun_WithName(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		return nil
	}, WithName("test-scope"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
