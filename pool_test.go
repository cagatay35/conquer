package conquer

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_BasicExecution(t *testing.T) {
	pool := NewPool(context.Background())
	var count atomic.Int64

	for i := 0; i < 10; i++ {
		pool.Go(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})
	}

	if err := pool.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count.Load() != 10 {
		t.Fatalf("expected 10, got %d", count.Load())
	}
}

func TestPool_ErrorPropagation(t *testing.T) {
	pool := NewPool(context.Background())
	sentinel := fmt.Errorf("pool error")

	pool.Go(func(ctx context.Context) error {
		return sentinel
	})

	err := pool.Wait()
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got: %v", err)
	}
}

func TestPool_MaxGoroutines(t *testing.T) {
	const maxG = 3
	const total = 30
	var peak, current atomic.Int64

	pool := NewPool(context.Background(), WithMaxGoroutines(maxG))

	for i := 0; i < total; i++ {
		pool.Go(func(ctx context.Context) error {
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

	if err := pool.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if peak.Load() > int64(maxG) {
		t.Fatalf("peak %d exceeded max %d", peak.Load(), maxG)
	}
}

func TestPool_PanicRecovery(t *testing.T) {
	pool := NewPool(context.Background())

	pool.Go(func(ctx context.Context) error {
		panic("pool panic")
	})

	err := pool.Wait()
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got: %T: %v", err, err)
	}
}

func TestResultPool_Basic(t *testing.T) {
	rp := NewResultPool[int](context.Background())

	for i := 0; i < 5; i++ {
		i := i
		rp.Go(func(ctx context.Context) (int, error) {
			return i * 2, nil
		})
	}

	results, err := rp.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	for i, v := range results {
		if v != i*2 {
			t.Errorf("result[%d] = %d, want %d", i, v, i*2)
		}
	}
}

func TestResultPool_WithError(t *testing.T) {
	rp := NewResultPool[string](context.Background(), WithErrorStrategy(CollectAll))

	rp.Go(func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	rp.Go(func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("fail")
	})

	_, err := rp.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewPool(ctx)

	pool.Go(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	cancel()
	err := pool.Wait()
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
