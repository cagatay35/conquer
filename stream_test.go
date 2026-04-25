package conquer

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

func TestFanOut_Basic(t *testing.T) {
	input := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		input <- i
	}
	close(input)

	out := FanOut(ctx(), input, 3, func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	})

	values, err := Drain(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Ints(values)
	expected := []int{2, 4, 6, 8, 10}
	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(values))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestFanOut_WithErrors(t *testing.T) {
	input := make(chan int, 3)
	input <- 1
	input <- 2
	input <- 3
	close(input)

	out := FanOut(ctx(), input, 2, func(ctx context.Context, item int) (string, error) {
		if item == 2 {
			return "", fmt.Errorf("bad: %d", item)
		}
		return fmt.Sprintf("ok:%d", item), nil
	})

	values, err := Drain(out)
	if err == nil {
		t.Fatal("expected error")
	}

	if len(values) == 0 {
		t.Error("expected some successful values despite error")
	}
}

func TestFanIn_Basic(t *testing.T) {
	ch1 := make(chan int, 3)
	ch2 := make(chan int, 3)
	ch3 := make(chan int, 3)

	ch1 <- 1
	ch1 <- 2
	close(ch1)

	ch2 <- 3
	ch2 <- 4
	close(ch2)

	ch3 <- 5
	close(ch3)

	merged := FanIn(ctx(), ch1, ch2, ch3)

	var results []int
	for v := range merged {
		results = append(results, v)
	}

	sort.Ints(results)
	expected := []int{1, 2, 3, 4, 5}
	if len(results) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(results))
	}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("results[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestFanIn_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; ; i++ {
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	merged := FanIn(ctx, ch)

	count := 0
	for range merged {
		count++
		if count >= 5 {
			cancel()
		}
	}
}

func TestGenerate_Basic(t *testing.T) {
	i := 0
	ch := Generate(ctx(), func(ctx context.Context) (int, bool) {
		i++
		if i > 5 {
			return 0, false
		}
		return i, true
	})

	var results []int
	for v := range ch {
		results = append(results, v)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 items, got %d", len(results))
	}
	for i, v := range results {
		if v != i+1 {
			t.Errorf("results[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestGenerate_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ch := Generate(ctx, func(ctx context.Context) (int, bool) {
		return 1, true
	})

	count := 0
	for range ch {
		count++
	}

	if count == 0 {
		t.Error("expected at least some items before cancellation")
	}
}

func TestDrain_Basic(t *testing.T) {
	ch := make(chan Result[int], 4)
	ch <- Result[int]{Value: 1}
	ch <- Result[int]{Value: 2}
	ch <- Result[int]{Err: fmt.Errorf("err1")}
	ch <- Result[int]{Value: 3}
	close(ch)

	values, err := Drain(ch)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}
}

func TestMerge_IsAliasForFanIn(t *testing.T) {
	ch1 := make(chan int, 2)
	ch1 <- 10
	ch1 <- 20
	close(ch1)

	out := Merge(ctx(), ch1)
	var results []int
	for v := range out {
		results = append(results, v)
	}

	sort.Ints(results)
	if len(results) != 2 || results[0] != 10 || results[1] != 20 {
		t.Fatalf("unexpected results: %v", results)
	}
}
