package conquer

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestMap_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	results, err := Map(ctx(), items, func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6, 8, 10}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("results[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestMap_PreservesOrder(t *testing.T) {
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	results, err := Map(ctx(), items, func(ctx context.Context, item int) (int, error) {
		return item, nil
	}, WithMaxGoroutines(5))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, v := range results {
		if v != i {
			t.Fatalf("order not preserved: results[%d] = %d", i, v)
		}
	}
}

func TestMap_Empty(t *testing.T) {
	results, err := Map(ctx(), []int{}, func(ctx context.Context, item int) (int, error) {
		return 0, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil for empty input, got: %v", results)
	}
}

func TestMap_Error(t *testing.T) {
	sentinel := fmt.Errorf("map error")
	items := []int{1, 2, 3}

	_, err := Map(ctx(), items, func(ctx context.Context, item int) (int, error) {
		if item == 2 {
			return 0, sentinel
		}
		return item, nil
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got: %v", err)
	}
}

func TestForEach_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results := make([]int, 5)

	err := ForEach(ctx(), items, func(ctx context.Context, item int) error {
		results[item-1] = item * 10
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, v := range results {
		if v != (i+1)*10 {
			t.Errorf("results[%d] = %d, want %d", i, v, (i+1)*10)
		}
	}
}

func TestForEach_Empty(t *testing.T) {
	err := ForEach(ctx(), []string{}, func(ctx context.Context, item string) error {
		t.Fatal("should not be called")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFilter_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	evens, err := Filter(ctx(), items, func(ctx context.Context, item int) (bool, error) {
		return item%2 == 0, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6, 8, 10}
	if len(evens) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(evens))
	}
	for i, v := range evens {
		if v != expected[i] {
			t.Errorf("evens[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestFilter_Empty(t *testing.T) {
	result, err := Filter(ctx(), []int{}, func(ctx context.Context, item int) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for empty, got: %v", result)
	}
}

func TestFilter_Error(t *testing.T) {
	sentinel := fmt.Errorf("filter error")

	_, err := Filter(ctx(), []int{1, 2, 3}, func(ctx context.Context, item int) (bool, error) {
		if item == 2 {
			return false, sentinel
		}
		return true, nil
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got: %v", err)
	}
}

func TestFirst_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	result, err := First(ctx(), items, func(ctx context.Context, item int) (string, error) {
		if item == 3 {
			return "found", nil
		}
		return "", fmt.Errorf("not this one")
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "found" {
		t.Fatalf("expected 'found', got: %s", result)
	}
}

func TestFirst_AllFail(t *testing.T) {
	items := []int{1, 2, 3}

	_, err := First(ctx(), items, func(ctx context.Context, item int) (string, error) {
		return "", fmt.Errorf("fail %d", item)
	})

	if err == nil {
		t.Fatal("expected error when all items fail")
	}
}

func TestFirst_Empty(t *testing.T) {
	result, err := First(ctx(), []int{}, func(ctx context.Context, item int) (string, error) {
		return "x", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected zero value, got: %s", result)
	}
}

func TestReduce_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	sum, err := Reduce(ctx(), items, 0,
		func(ctx context.Context, item int) (int, error) {
			return item, nil
		},
		func(a, b int) int {
			return a + b
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum != 15 {
		t.Fatalf("expected 15, got %d", sum)
	}
}

func TestReduce_Empty(t *testing.T) {
	result, err := Reduce(ctx(), []int{}, 42,
		func(ctx context.Context, item int) (int, error) {
			return item, nil
		},
		func(a, b int) int {
			return a + b
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("expected initial value 42, got: %d", result)
	}
}

func TestMap_WithMaxGoroutines(t *testing.T) {
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	results, err := Map(ctx(), items, func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	}, WithMaxGoroutines(3))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, v := range results {
		if v != i*2 {
			t.Errorf("results[%d] = %d, want %d", i, v, i*2)
		}
	}
}

func ctx() context.Context {
	return context.Background()
}
