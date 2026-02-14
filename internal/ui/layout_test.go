package ui

import "testing"

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

func assertInts(t *testing.T, label string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d want %d; got %v", label, len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: index %d: got %d want %d (full: %v)", label, i, got[i], want[i], got)
		}
	}
}

func sumInts(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func TestAllocateHeightsEqualWeight(t *testing.T) {
	t.Parallel()
	hints := []SizeHint{
		{MinH: 3, Weight: 1},
		{MinH: 3, Weight: 1},
		{MinH: 3, Weight: 1},
	}
	got := AllocateHeights(hints, 30)
	assertInts(t, "equal", got, []int{10, 10, 10})
}

func TestAllocateHeightsMixedWeight(t *testing.T) {
	t.Parallel()
	// Process (w4) + battery (w1, max3) + cpu (w3)
	hints := []SizeHint{
		{MinH: 5, MaxH: 0, Weight: 4}, // process
		{MinH: 2, MaxH: 3, Weight: 1}, // battery
		{MinH: 3, MaxH: 0, Weight: 3}, // cpu
	}
	total := 40
	got := AllocateHeights(hints, total)
	if sumInts(got) != total {
		t.Fatalf("sum=%d want %d; alloc=%v", sumInts(got), total, got)
	}
	// Battery should be capped at 3.
	if got[1] != 3 {
		t.Fatalf("battery got %d want 3 (capped)", got[1])
	}
	// Process should get more than CPU (weight 4 vs 3).
	if got[0] <= got[2] {
		t.Fatalf("process (%d) should exceed cpu (%d)", got[0], got[2])
	}
}

func TestAllocateHeightsAllCapped(t *testing.T) {
	t.Parallel()
	hints := []SizeHint{
		{MinH: 2, MaxH: 5, Weight: 1},
		{MinH: 2, MaxH: 5, Weight: 1},
	}
	got := AllocateHeights(hints, 100)
	assertInts(t, "all-capped", got, []int{5, 5})
}

func TestAllocateHeightsUndersized(t *testing.T) {
	t.Parallel()
	hints := []SizeHint{
		{MinH: 5, Weight: 1},
		{MinH: 5, Weight: 1},
	}
	got := AllocateHeights(hints, 6)
	if sumInts(got) != 6 {
		t.Fatalf("sum=%d want 6; alloc=%v", sumInts(got), got)
	}
}

func TestAllocateHeightsSinglePlugin(t *testing.T) {
	t.Parallel()
	hints := []SizeHint{{MinH: 3, MaxH: 0, Weight: 1}}
	got := AllocateHeights(hints, 50)
	assertInts(t, "single", got, []int{50})
}

func TestAllocateHeightsSinglePluginCapped(t *testing.T) {
	t.Parallel()
	hints := []SizeHint{{MinH: 3, MaxH: 10, Weight: 1}}
	got := AllocateHeights(hints, 50)
	assertInts(t, "single-capped", got, []int{10})
}

func TestAllocateHeightsZeroWeight(t *testing.T) {
	t.Parallel()
	// Clock (w0, max1) should only get its min, all surplus goes to cpu.
	hints := []SizeHint{
		{MinH: 1, MaxH: 1, Weight: 0}, // clock
		{MinH: 3, MaxH: 0, Weight: 3}, // cpu
	}
	got := AllocateHeights(hints, 30)
	assertInts(t, "zero-weight", got, []int{1, 29})
}

func TestAllocateHeightsEdgeCases(t *testing.T) {
	t.Parallel()
	if got := AllocateHeights(nil, 10); got != nil {
		t.Fatalf("nil hints: expected nil, got %v", got)
	}
	if got := AllocateHeights([]SizeHint{{MinH: 3, Weight: 1}}, 0); got != nil {
		t.Fatalf("zero total: expected nil, got %v", got)
	}
}

func TestAllocateHeightsBatteryScenario(t *testing.T) {
	t.Parallel()
	// Realistic scenario: cpu + memory + network + process + battery
	hints := []SizeHint{
		{MinH: 3, PrefH: 12, MaxH: 0, Weight: 3}, // cpu
		{MinH: 3, PrefH: 8, MaxH: 0, Weight: 2},  // memory
		{MinH: 3, PrefH: 8, MaxH: 0, Weight: 2},  // network
		{MinH: 5, PrefH: 20, MaxH: 0, Weight: 4}, // process
		{MinH: 2, PrefH: 3, MaxH: 3, Weight: 1},  // battery
	}
	total := 60
	got := AllocateHeights(hints, total)
	if sumInts(got) != total {
		t.Fatalf("sum=%d want %d; alloc=%v", sumInts(got), total, got)
	}
	// Battery capped at 3.
	if got[4] != 3 {
		t.Fatalf("battery got %d want 3", got[4])
	}
	// Process (weight 4) should be largest.
	for i := 0; i < 4; i++ {
		if i == 3 {
			continue
		}
		if got[3] <= got[i] {
			t.Fatalf("process (%d) should exceed plugin %d (%d)", got[3], i, got[i])
		}
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
