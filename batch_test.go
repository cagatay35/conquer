package conquer

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBatchProcessor_SizeFlush(t *testing.T) {
	var mu sync.Mutex
	var batches [][]int

	bp := NewBatchProcessor(ctx(), func(ctx context.Context, batch []int) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]int, len(batch))
		copy(cp, batch)
		batches = append(batches, cp)
		return nil
	}, BatchSize(3))

	for i := 1; i <= 7; i++ {
		if err := bp.Add(i); err != nil {
			t.Fatalf("Add(%d) error: %v", i, err)
		}
	}

	if err := bp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(batches) != 3 {
		t.Fatalf("expected 3 batches (3+3+1), got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("batch 0 size = %d, want 3", len(batches[0]))
	}
	if len(batches[1]) != 3 {
		t.Errorf("batch 1 size = %d, want 3", len(batches[1]))
	}
	if len(batches[2]) != 1 {
		t.Errorf("batch 2 size = %d, want 1", len(batches[2]))
	}
}

func TestBatchProcessor_IntervalFlush(t *testing.T) {
	var mu sync.Mutex
	var batches [][]string

	bp := NewBatchProcessor(ctx(), func(ctx context.Context, batch []string) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]string, len(batch))
		copy(cp, batch)
		batches = append(batches, cp)
		return nil
	}, BatchInterval(50*time.Millisecond))

	_ = bp.Add("a")
	_ = bp.Add("b")

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	flushed := len(batches) > 0
	mu.Unlock()

	if !flushed {
		t.Error("expected interval-based flush to have triggered")
	}

	if err := bp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestBatchProcessor_ManualFlush(t *testing.T) {
	var batches [][]int

	bp := NewBatchProcessor(ctx(), func(ctx context.Context, batch []int) error {
		cp := make([]int, len(batch))
		copy(cp, batch)
		batches = append(batches, cp)
		return nil
	})

	_ = bp.Add(1)
	_ = bp.Add(2)

	if err := bp.Flush(ctx()); err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("expected 1 batch after manual flush, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("batch size = %d, want 2", len(batches[0]))
	}

	if err := bp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestBatchProcessor_CloseFlushesRemaining(t *testing.T) {
	var batches [][]int

	bp := NewBatchProcessor(ctx(), func(ctx context.Context, batch []int) error {
		cp := make([]int, len(batch))
		copy(cp, batch)
		batches = append(batches, cp)
		return nil
	}, BatchSize(100))

	_ = bp.Add(1)
	_ = bp.Add(2)

	if err := bp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("expected Close to flush remaining, got %d batches", len(batches))
	}
}

func TestBatchProcessor_ConcurrentAdd(t *testing.T) {
	var mu sync.Mutex
	totalItems := 0

	bp := NewBatchProcessor(ctx(), func(ctx context.Context, batch []int) error {
		mu.Lock()
		defer mu.Unlock()
		totalItems += len(batch)
		return nil
	}, BatchSize(10))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = bp.Add(base*10 + j)
			}
		}(i)
	}

	wg.Wait()

	if err := bp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if totalItems != 100 {
		t.Fatalf("expected 100 items flushed, got %d", totalItems)
	}
}

func TestBatchProcessor_EmptyClose(t *testing.T) {
	called := false

	bp := NewBatchProcessor(ctx(), func(ctx context.Context, batch []int) error {
		called = true
		return nil
	}, BatchSize(5))

	if err := bp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if called {
		t.Error("flush should not be called when no items were added")
	}
}
