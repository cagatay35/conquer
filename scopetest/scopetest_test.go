package scopetest_test

import (
	"context"
	"testing"

	"github.com/cagatay35/conquer"
	"github.com/cagatay35/conquer/scopetest"
)

func TestCheckLeaks_NoLeak(t *testing.T) {
	defer scopetest.CheckLeaks(t)

	_ = conquer.Run(context.Background(), func(s *conquer.Scope) error {
		s.Go(func(ctx context.Context) error {
			return nil
		})
		return nil
	})
}

func TestAssertPanic(t *testing.T) {
	err := conquer.Run(context.Background(), func(s *conquer.Scope) error {
		s.Go(func(ctx context.Context) error {
			panic("test panic")
		})
		return nil
	})

	pe := scopetest.AssertPanic(t, err)
	if pe.Value != "test panic" {
		t.Errorf("unexpected panic value: %v", pe.Value)
	}
}

func TestAssertNoPanic(t *testing.T) {
	err := conquer.Run(context.Background(), func(s *conquer.Scope) error {
		return nil
	})
	scopetest.AssertNoPanic(t, err)
}
