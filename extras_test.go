package conquer

import (
	"context"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func TestTee2_Basic(t *testing.T) {
	input := make(chan int, 3)
	input <- 1
	input <- 2
	input <- 3
	close(input)

	ch1, ch2 := Tee2(ctx(), input)

	var out1, out2 []int
	done := make(chan struct{})
	go func() {
		for v := range ch2 {
			out2 = append(out2, v)
		}
		close(done)
	}()
	for v := range ch1 {
		out1 = append(out1, v)
	}
	<-done

	if len(out1) != 3 || len(out2) != 3 {
		t.Fatalf("expected 3 items each, got %d and %d", len(out1), len(out2))
	}
	for i := 0; i < 3; i++ {
		if out1[i] != out2[i] {
			t.Errorf("mismatch at %d: %d vs %d", i, out1[i], out2[i])
		}
	}
}

func TestTeeN_Basic(t *testing.T) {
	input := make(chan int, 2)
	input <- 10
	input <- 20
	close(input)

	outputs := TeeN(ctx(), input, 3)
	if len(outputs) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(outputs))
	}

	results := make([][]int, 3)
	var done [3]chan struct{}
	for i := range outputs {
		done[i] = make(chan struct{})
		go func(idx int) {
			for v := range outputs[idx] {
				results[idx] = append(results[idx], v)
			}
			close(done[idx])
		}(i)
	}

	for _, d := range done {
		<-d
	}

	for i, r := range results {
		if len(r) != 2 {
			t.Errorf("channel %d: expected 2 items, got %d", i, len(r))
		}
	}
}

func TestThrottle_Basic(t *testing.T) {
	var count atomic.Int64
	fn := func(ctx context.Context, in int) (int, error) {
		count.Add(1)
		return in * 2, nil
	}

	throttled := Throttle(50*time.Millisecond, fn)

	start := time.Now()
	for i := 0; i < 3; i++ {
		result, err := throttled(ctx(), i)
		if err != nil {
			t.Fatalf("call %d error: %v", i, err)
		}
		if result != i*2 {
			t.Errorf("call %d: expected %d, got %d", i, i*2, result)
		}
	}
	elapsed := time.Since(start)

	if elapsed < 80*time.Millisecond {
		t.Errorf("throttle should have enforced delay, elapsed: %v", elapsed)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", count.Load())
	}
}

func TestChunk_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	chunks := Chunk(items, 3)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 3 || len(chunks[1]) != 3 || len(chunks[2]) != 1 {
		t.Errorf("wrong chunk sizes: %d, %d, %d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func TestChunk_Empty(t *testing.T) {
	chunks := Chunk([]int{}, 5)
	if chunks != nil {
		t.Fatalf("expected nil for empty, got: %v", chunks)
	}
}

func TestChunk_ExactSize(t *testing.T) {
	items := []int{1, 2, 3}
	chunks := Chunk(items, 3)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestMapChunked_Basic(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	results, err := MapChunked(ctx(), items, 3,
		func(ctx context.Context, chunk []int) ([]int, error) {
			out := make([]int, len(chunk))
			for i, v := range chunk {
				out[i] = v * 10
			}
			return out, nil
		})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Ints(results)
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for i, v := range results {
		if v != (i+1)*10 {
			t.Errorf("results[%d] = %d, want %d", i, v, (i+1)*10)
		}
	}
}

func TestCollect_Basic(t *testing.T) {
	ch := make(chan int, 5)
	ch <- 1
	ch <- 2
	ch <- 3
	close(ch)

	items := Collect(ctx(), ch)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestSendAll_Basic(t *testing.T) {
	ch := make(chan int, 5)
	items := []int{10, 20, 30}

	err := SendAll(ctx(), ch, items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	close(ch)
	var results []int
	for v := range ch {
		results = append(results, v)
	}
	if len(results) != 3 || results[0] != 10 {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestSendAll_ContextCancelled(t *testing.T) {
	ch := make(chan int) // unbuffered, will block
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := SendAll(ctx, ch, []int{1, 2, 3})
	if err != context.Canceled {
		t.Fatalf("expected Canceled, got: %v", err)
	}
}
