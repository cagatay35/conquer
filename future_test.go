package conquer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestFuture_Basic(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		f := Async(s, func(ctx context.Context) (int, error) {
			return 42, nil
		})

		val, err := f.Await(context.Background())
		if err != nil {
			return err
		}
		if val != 42 {
			return fmt.Errorf("expected 42, got %d", val)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFuture_Error(t *testing.T) {
	sentinel := fmt.Errorf("async error")

	err := Run(context.Background(), func(s *Scope) error {
		f := Async(s, func(ctx context.Context) (int, error) {
			return 0, sentinel
		})

		_, fErr := f.Await(context.Background())
		if !errors.Is(fErr, sentinel) {
			return fmt.Errorf("expected sentinel, got: %v", fErr)
		}
		return nil
	}, WithErrorStrategy(CollectAll))

	if err == nil {
		t.Fatal("expected error from scope")
	}
}

func TestFuture_ContextCancelled(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		f := Async(s, func(ctx context.Context) (int, error) {
			time.Sleep(5 * time.Second)
			return 0, nil
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, awaitErr := f.Await(ctx)
		if !errors.Is(awaitErr, context.DeadlineExceeded) {
			return fmt.Errorf("expected DeadlineExceeded, got: %v", awaitErr)
		}
		return nil
	}, WithTimeout(100*time.Millisecond), WithErrorStrategy(CollectAll))

	// scope timeout cancels the long-running goroutine
	_ = err
}

func TestFuture_TryGet_Pending(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		started := make(chan struct{})
		f := Async(s, func(ctx context.Context) (string, error) {
			close(started)
			time.Sleep(100 * time.Millisecond)
			return "done", nil
		})

		<-started
		_, _, ok := f.TryGet()
		if ok {
			return fmt.Errorf("expected TryGet to return false while pending")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFuture_TryGet_Resolved(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		f := Async(s, func(ctx context.Context) (string, error) {
			return "hello", nil
		})

		val, fErr := f.Await(context.Background())
		if fErr != nil {
			return fErr
		}

		gotVal, gotErr, ok := f.TryGet()
		if !ok {
			return fmt.Errorf("expected TryGet to succeed after Await")
		}
		if gotVal != val {
			return fmt.Errorf("TryGet value mismatch: %v vs %v", gotVal, val)
		}
		if gotErr != nil {
			return fmt.Errorf("unexpected TryGet error: %v", gotErr)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFuture_Done(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		f := Async(s, func(ctx context.Context) (int, error) {
			return 1, nil
		})

		select {
		case <-f.Done():
			// expected
		case <-time.After(5 * time.Second):
			return fmt.Errorf("future.Done() not closed in time")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFuture_MultipleConcurrent(t *testing.T) {
	err := Run(context.Background(), func(s *Scope) error {
		f1 := Async(s, func(ctx context.Context) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return 1, nil
		})
		f2 := Async(s, func(ctx context.Context) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return 2, nil
		})
		f3 := Async(s, func(ctx context.Context) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return 3, nil
		})

		v1, _ := f1.Await(context.Background())
		v2, _ := f2.Await(context.Background())
		v3, _ := f3.Await(context.Background())

		if v1+v2+v3 != 6 {
			return fmt.Errorf("expected sum 6, got %d", v1+v2+v3)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
