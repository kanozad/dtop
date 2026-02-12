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

func TestSplitWidths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		total   int
		numCols int
		want    []int
	}{
		{"even split", 80, 2, []int{40, 40}},
		{"odd split", 81, 2, []int{41, 40}},
		{"three cols", 120, 3, []int{40, 40, 40}},
		{"three cols uneven", 100, 3, []int{34, 33, 33}},
		{"single col", 80, 1, []int{80}},
		{"zero total", 0, 2, nil},
		{"zero cols", 80, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitWidths(tt.total, tt.numCols)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("index %d: got %d want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGridColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		count   int
		numCols int
		want    []int
	}{
		{"5 into 2", 5, 2, []int{3, 2}},
		{"4 into 2", 4, 2, []int{2, 2}},
		{"3 into 2", 3, 2, []int{2, 1}},
		{"1 into 2", 1, 2, []int{1}},
		{"6 into 3", 6, 3, []int{2, 2, 2}},
		{"7 into 3", 7, 3, []int{3, 2, 2}},
		{"zero count", 0, 2, nil},
		{"zero cols", 5, 0, nil},
		{"more cols than items", 2, 5, []int{1, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GridColumns(tt.count, tt.numCols)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("index %d: got %d want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFlowColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		count     int
		height    int
		minHeight int
		want      int
	}{
		{"fits in one column", 3, 9, 3, 1},
		{"needs two columns", 5, 9, 3, 2},
		{"single row", 5, 2, 3, 5},
		{"zero count", 0, 10, 3, 0},
		{"zero height", 3, 0, 3, 0},
		{"min height clamp", 4, 4, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlowColumns(tt.count, tt.height, tt.minHeight)
			if got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}
