package conquer

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestGo_Basic(t *testing.T) {
	var done atomic.Bool
	err := Go(ctx(), func(ctx context.Context) error {
		done.Store(true)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done.Load() {
		t.Fatal("function not executed")
	}
}

func TestGo_PanicRecovery(t *testing.T) {
	err := Go(ctx(), func(ctx context.Context) error {
		panic("go panic")
	})
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got: %T", err)
	}
}

func TestGo2_Basic(t *testing.T) {
	var a, b atomic.Bool
	err := Go2(ctx(),
		func(ctx context.Context) error { a.Store(true); return nil },
		func(ctx context.Context) error { b.Store(true); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.Load() || !b.Load() {
		t.Fatal("not all functions executed")
	}
}

func TestGo3_Basic(t *testing.T) {
	var count atomic.Int64
	err := Go3(ctx(),
		func(ctx context.Context) error { count.Add(1); return nil },
		func(ctx context.Context) error { count.Add(1); return nil },
		func(ctx context.Context) error { count.Add(1); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3, got %d", count.Load())
	}
}

func TestGoN_Basic(t *testing.T) {
	var count atomic.Int64
	fns := make([]func(ctx context.Context) error, 10)
	for i := range fns {
		fns[i] = func(ctx context.Context) error { count.Add(1); return nil }
	}

	err := GoN(ctx(), fns...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 10 {
		t.Fatalf("expected 10, got %d", count.Load())
	}
}

func TestRace_FirstWins(t *testing.T) {
	result, err := Race(ctx(),
		func(ctx context.Context) (string, error) {
			time.Sleep(100 * time.Millisecond)
			return "slow", nil
		},
		func(ctx context.Context) (string, error) {
			return "fast", nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fast" {
		t.Fatalf("expected 'fast', got '%s'", result)
	}
}

func TestRace_AllFail(t *testing.T) {
	_, err := Race(ctx(),
		func(ctx context.Context) (int, error) { return 0, fmt.Errorf("fail1") },
		func(ctx context.Context) (int, error) { return 0, fmt.Errorf("fail2") },
	)
	if err == nil {
		t.Fatal("expected error when all racers fail")
	}
}

func TestRace_Empty(t *testing.T) {
	result, err := Race[int](ctx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Fatalf("expected zero, got %d", result)
	}
}

func TestRace_Single(t *testing.T) {
	result, err := Race(ctx(),
		func(ctx context.Context) (string, error) { return "only", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "only" {
		t.Fatalf("expected 'only', got '%s'", result)
	}
}

func TestRetry_SucceedsOnFirst(t *testing.T) {
	result, err := Retry(ctx(), 3, time.Millisecond,
		func(ctx context.Context) (string, error) {
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got '%s'", result)
	}
}

func TestRetry_SucceedsOnThird(t *testing.T) {
	var attempts atomic.Int64
	result, err := Retry(ctx(), 5, time.Millisecond,
		func(ctx context.Context) (int, error) {
			a := attempts.Add(1)
			if a < 3 {
				return 0, fmt.Errorf("attempt %d", a)
			}
			return 42, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRetry_AllFail(t *testing.T) {
	_, err := Retry(ctx(), 3, time.Millisecond,
		func(ctx context.Context) (int, error) {
			return 0, fmt.Errorf("always fail")
		})
	if err == nil {
		t.Fatal("expected error after all retries")
	}
}

func TestRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Retry(ctx, 10, time.Second,
		func(ctx context.Context) (int, error) {
			return 0, fmt.Errorf("fail")
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestRetryWithFn_CustomBackoff(t *testing.T) {
	var attempts atomic.Int64
	_, err := RetryWithFn(ctx(), 3,
		func(attempt int) time.Duration { return time.Millisecond },
		func(ctx context.Context) (int, error) {
			attempts.Add(1)
			return 0, fmt.Errorf("fail")
		})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}
