package cpu

import (
	"strings"
	"testing"

	"github.com/kanozad/dtop/internal/testutil"
	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/pkg/types"
)

func TestGraphRows(t *testing.T) {
	tests := []struct {
		h           int
		coreBarRows int
		want        int
	}{
		// MinH=3 (inner content): 3 - 3 - 0 = 0; title+meter+summary exactly fill the space.
		{3, 0, 0},
		// h=5, no cores: 5 - 3 - 0 = 2
		{5, 0, 2},
		// h=6, no cores: 6 - 3 - 0 = 3
		{6, 0, 3},
		// h=6, 4 core bar rows: 6 - 3 - 4 = -1 → 0
		{6, 4, 0},
		// PrefH=12, no core bars: 12 - 3 = 9, capped at 6
		{12, 0, 6},
		// PrefH=12, 4 core bar rows: 12 - 3 - 4 = 5
		{12, 4, 5},
		// h=8, 4 core bar rows: 8 - 3 - 4 = 1
		{8, 4, 1},
		// h=9, 4 core bar rows: 9 - 3 - 4 = 2
		{9, 4, 2},
		// h=10, 4 core bar rows: 10 - 3 - 4 = 3
		{10, 4, 3},
		// Zero height
		{0, 0, 0},
	}
	for _, tt := range tests {
		got := graphRows(tt.h, tt.coreBarRows)
		if got != tt.want {
			t.Errorf("graphRows(%d, %d) = %d, want %d", tt.h, tt.coreBarRows, got, tt.want)
		}
	}
}

func TestCoreBarCount(t *testing.T) {
	tests := []struct {
		cores []float64
		want  int
	}{
		{nil, 0},
		{[]float64{50}, 1},
		{[]float64{50, 60, 70, 80}, 4},
		// 5 cores → 2-column layout: (5+1)/2 = 3 rows
		{[]float64{50, 60, 70, 80, 90}, 3},
		// 6 cores → (6+1)/2 = 3 rows
		{[]float64{50, 60, 70, 80, 90, 100}, 3},
		// 8 cores → (8+1)/2 = 4 rows
		{[]float64{50, 60, 70, 80, 90, 100, 50, 60}, 4},
	}
	for _, tt := range tests {
		got := coreBarCount(tt.cores)
		if got != tt.want {
			t.Errorf("coreBarCount(%d cores) = %d, want %d", len(tt.cores), got, tt.want)
		}
	}
}

func TestCPUViewAtMinHeight(t *testing.T) {
	c := New()
	th := theme.Default()
	stats := types.CPUStats{
		Total:  42.5,
		Load1:  1.0,
		Load5:  0.8,
		Load15: 0.5,
	}
	// MinH=3 is the inner content minimum. Box renders 3 content rows (title+meter+summary).
	minH := c.SizeHint().MinH
	out := c.View(stats, 80, minH, th)
	if out == "" {
		t.Fatal("View returned empty output at MinH")
	}
	if !strings.Contains(out, "CPU") {
		t.Errorf("expected box title 'CPU' in output, got: %q", out)
	}
}

func TestCPUViewContentAtUsableHeight(t *testing.T) {
	c := New()
	c.cfg.PerCore = false
	th := theme.Default()
	stats := types.CPUStats{
		Total:  42.5,
		Load1:  1.0,
		Load5:  0.8,
		Load15: 0.5,
	}
	// At inner h=5: title(1)+meter(1)+graph(2)+summary(1)=5 content rows.
	out := c.View(stats, 80, 5, th)
	if !strings.Contains(out, "Load:") {
		t.Errorf("expected summary line 'Load:' at h=5, got: %q", out)
	}
}

func TestCPUViewGraphAppearsAtHeight4(t *testing.T) {
	c := New()
	c.cfg.PerCore = false
	th := theme.Default()

	history := make([]float64, 80)
	for i := range history {
		history[i] = float64(i % 100)
	}
	stats := types.CPUStats{
		Total:        50,
		Load1:        1.0,
		TotalHistory: history,
	}

	// At h=3: graphRows(3,0)=0 → no graph.
	// At h=4: graphRows(4,0)=1 → one graph row appears.
	outNoGraph := c.View(stats, 80, 3, th)
	outWithGraph := c.View(stats, 80, 4, th)

	if len(outWithGraph) <= len(outNoGraph) {
		t.Errorf("expected View at h=4 to produce more content than h=3 (graph row should appear)")
	}
}

func TestCPUViewPrefHeight(t *testing.T) {
	c := New()
	c.cfg.PerCore = false
	th := theme.Default()

	history := make([]float64, 80)
	for i := range history {
		history[i] = float64(i % 100)
	}
	stats := types.CPUStats{
		Total:        75,
		Load1:        2.0,
		TotalHistory: history,
	}

	prefH := c.SizeHint().PrefH
	out := c.View(stats, 80, prefH, th)
	if out == "" {
		t.Fatal("View returned empty output at PrefH")
	}
	if !strings.Contains(out, "Load:") {
		t.Errorf("expected summary line 'Load:' at PrefH, got: %q", out)
	}
}

// Golden-file tests: capture full rendered output (ANSI stripped) and compare
// against stored fixtures in testdata/. Re-generate with: go test -run 'Golden' -update

func TestCPUGoldenMinHeight(t *testing.T) {
	c := New()
	c.cfg.PerCore = false
	th := theme.Default()
	stats := types.CPUStats{
		Total:  42.5,
		Load1:  1.00,
		Load5:  0.75,
		Load15: 0.50,
	}
	out := c.View(stats, 80, c.SizeHint().MinH, th)
	testutil.CheckGolden(t, "cpu_min_height", out)
}

func TestCPUGoldenWithHistory(t *testing.T) {
	c := New()
	c.cfg.PerCore = false
	th := theme.Default()
	history := make([]float64, 60)
	for i := range history {
		history[i] = float64(i%50) * 2
	}
	stats := types.CPUStats{
		Total:        75.0,
		Load1:        2.00,
		Load5:        1.50,
		Load15:       1.00,
		TotalHistory: history,
	}
	out := c.View(stats, 80, 12, th)
	testutil.CheckGolden(t, "cpu_with_history", out)
}

func TestCPUGoldenWithPerCore(t *testing.T) {
	c := New()
	c.cfg.PerCore = true
	th := theme.Default()
	stats := types.CPUStats{
		Total:   60.0,
		Load1:   1.50,
		Load5:   1.25,
		Load15:  1.00,
		PerCore: []float64{80.0, 40.0, 70.0, 30.0},
	}
	out := c.View(stats, 80, 12, th)
	testutil.CheckGolden(t, "cpu_with_per_core", out)
}
