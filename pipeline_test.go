package conquer

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

func TestPipeline2_Basic(t *testing.T) {
	source := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		source <- i
	}
	close(source)

	out, errCh := Pipeline2(
		ctx(),
		source,
		NewStage("double", func(ctx context.Context, in int) (int, error) {
			return in * 2, nil
		}, 2),
		NewStage("toString", func(ctx context.Context, in int) (string, error) {
			return fmt.Sprintf("val:%d", in), nil
		}, 2),
	)

	var results []string
	for v := range out {
		results = append(results, v)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	sort.Strings(results)
	expected := []string{"val:10", "val:2", "val:4", "val:6", "val:8"}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("results[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

func TestPipeline3_Basic(t *testing.T) {
	source := make(chan int, 3)
	source <- 1
	source <- 2
	source <- 3
	close(source)

	out, errCh := Pipeline3(
		ctx(),
		source,
		NewStage("add10", func(ctx context.Context, in int) (int, error) {
			return in + 10, nil
		}, 1),
		NewStage("double", func(ctx context.Context, in int) (int, error) {
			return in * 2, nil
		}, 1),
		NewStage("toString", func(ctx context.Context, in int) (string, error) {
			return fmt.Sprintf("%d", in), nil
		}, 1),
	)

	var results []string
	for v := range out {
		results = append(results, v)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(results)
	expected := []string{"22", "24", "26"}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d: %v", len(expected), len(results), results)
	}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("results[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

func TestPipeline2_ErrorPropagation(t *testing.T) {
	source := make(chan int, 3)
	source <- 1
	source <- 2
	source <- 3
	close(source)

	out, errCh := Pipeline2(
		ctx(),
		source,
		NewStage("fail", func(ctx context.Context, in int) (int, error) {
			if in == 2 {
				return 0, fmt.Errorf("bad input: %d", in)
			}
			return in, nil
		}, 1),
		NewStage("pass", func(ctx context.Context, in int) (int, error) {
			return in, nil
		}, 1),
	)

	for range out {
	}

	err := <-errCh
	if err == nil {
		t.Fatal("expected error from pipeline")
	}
}

func TestPipeline2_EmptySource(t *testing.T) {
	source := make(chan int)
	close(source)

	out, errCh := Pipeline2(
		ctx(),
		source,
		NewStage("s1", func(ctx context.Context, in int) (int, error) {
			return in, nil
		}, 1),
		NewStage("s2", func(ctx context.Context, in int) (string, error) {
			return "", nil
		}, 1),
	)

	count := 0
	for range out {
		count++
	}

	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 results from empty source, got %d", count)
	}
}

func TestPipeline2_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	source := make(chan int)
	go func() {
		defer close(source)
		for i := 0; ; i++ {
			select {
			case source <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	out, errCh := Pipeline2(
		ctx,
		source,
		NewStage("s1", func(ctx context.Context, in int) (int, error) {
			return in, nil
		}, 2),
		NewStage("s2", func(ctx context.Context, in int) (int, error) {
			return in, nil
		}, 2),
	)

	count := 0
	for range out {
		count++
		if count >= 10 {
			cancel()
		}
	}

	<-errCh
}
