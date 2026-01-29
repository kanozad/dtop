package ui

import "testing"

func TestSplitHeightsTooSmall(t *testing.T) {
	t.Parallel()

	got := SplitHeights(5, 3, 2)
	want := []int{2, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestSplitHeightsWithMinimum(t *testing.T) {
	t.Parallel()

	got := SplitHeights(10, 3, 2)
	want := []int{4, 3, 3}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestSplitHeightsEdgeCases(t *testing.T) {
	t.Parallel()

	if got := SplitHeights(0, 3, 1); got != nil {
		t.Fatalf("expected nil for zero total, got %v", got)
	}
	if got := SplitHeights(10, 0, 1); got != nil {
		t.Fatalf("expected nil for zero count, got %v", got)
	}
	if got := SplitHeights(3, 3, 0); len(got) != 3 {
		t.Fatalf("expected 3 entries, got %v", got)
	}
}
