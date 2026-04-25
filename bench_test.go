package conquer

import (
	"context"
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"
)

func BenchmarkScopeGo_1(b *testing.B) {
	benchmarkScopeGo(b, 1)
}

func BenchmarkScopeGo_10(b *testing.B) {
	benchmarkScopeGo(b, 10)
}

func BenchmarkScopeGo_100(b *testing.B) {
	benchmarkScopeGo(b, 100)
}

func BenchmarkScopeGo_1000(b *testing.B) {
	benchmarkScopeGo(b, 1000)
}

func benchmarkScopeGo(b *testing.B, n int) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = Run(ctx, func(s *Scope) error {
			for j := 0; j < n; j++ {
				s.Go(func(ctx context.Context) error {
					return nil
				})
			}
			return nil
		})
	}
}

func BenchmarkErrgroup_1(b *testing.B) {
	benchmarkErrgroup(b, 1)
}

func BenchmarkErrgroup_10(b *testing.B) {
	benchmarkErrgroup(b, 10)
}

func BenchmarkErrgroup_100(b *testing.B) {
	benchmarkErrgroup(b, 100)
}

func BenchmarkErrgroup_1000(b *testing.B) {
	benchmarkErrgroup(b, 1000)
}

func benchmarkErrgroup(b *testing.B, n int) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		g, _ := errgroup.WithContext(ctx)
		for j := 0; j < n; j++ {
			g.Go(func() error {
				return nil
			})
		}
		_ = g.Wait()
	}
}

func BenchmarkPool_Bounded(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pool := NewPool(ctx, WithMaxGoroutines(10))
		for j := 0; j < 100; j++ {
			pool.Go(func(ctx context.Context) error {
				return nil
			})
		}
		_ = pool.Wait()
	}
}

func BenchmarkMap_100(b *testing.B) {
	ctx := context.Background()
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = Map(ctx, items, func(ctx context.Context, item int) (int, error) {
			return item * 2, nil
		})
	}
}

func BenchmarkMap_100_Bounded(b *testing.B) {
	ctx := context.Background()
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = Map(ctx, items, func(ctx context.Context, item int) (int, error) {
			return item * 2, nil
		}, WithMaxGoroutines(10))
	}
}

func BenchmarkFuture(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = Run(ctx, func(s *Scope) error {
			f := Async(s, func(ctx context.Context) (int, error) {
				return 42, nil
			})
			_, _ = f.Await(ctx)
			return nil
		})
	}
}

func BenchmarkRawGoroutine_1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1000)
		for j := 0; j < 1000; j++ {
			go func() {
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkGo_Single(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Go(ctx, func(ctx context.Context) error { return nil })
	}
}

func BenchmarkGo2(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Go2(ctx,
			func(ctx context.Context) error { return nil },
			func(ctx context.Context) error { return nil },
		)
	}
}

func BenchmarkRace_3(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Race(ctx,
			func(ctx context.Context) (int, error) { return 1, nil },
			func(ctx context.Context) (int, error) { return 2, nil },
			func(ctx context.Context) (int, error) { return 3, nil },
		)
	}
}

func BenchmarkChunk(b *testing.B) {
	items := make([]int, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Chunk(items, 100)
	}
}
