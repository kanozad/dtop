package gpu

import (
	"strings"
	"testing"

	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/pkg/types"
)

func makeGPU() types.GPUInfo {
	return types.GPUInfo{
		Index:          0,
		Name:           "Test GPU",
		UtilizationPct: 75.0,
		MemoryUsed:     4 * 1024 * 1024 * 1024,
		MemoryTotal:    8 * 1024 * 1024 * 1024,
		MemoryPct:      50.0,
		TemperatureC:   65.0,
		PowerWatts:     120.0,
		PowerCapWatts:  200.0,
		ClockCoreMHz:   1800,
	}
}

func TestGPUViewNoData(t *testing.T) {
	g := New()
	th := theme.Default()
	out := g.View(nil, 80, 6, th)
	if !strings.Contains(out, "GPU") {
		t.Errorf("expected box title 'GPU', got: %q", out)
	}
	if !strings.Contains(out, "Collecting...") {
		t.Errorf("expected 'Collecting...' placeholder, got: %q", out)
	}
}

func TestGPUViewError(t *testing.T) {
	g := New()
	th := theme.Default()
	stats := types.GPUStats{
		Error: "driver not loaded",
	}
	out := g.View(stats, 80, 6, th)
	if !strings.Contains(out, "GPU") {
		t.Errorf("expected box title 'GPU', got: %q", out)
	}
	if !strings.Contains(out, "N/A") {
		t.Errorf("expected 'N/A' in error output, got: %q", out)
	}
}

func TestGPUViewAtMinHeight(t *testing.T) {
	g := New()
	th := theme.Default()
	stats := types.GPUStats{
		GPUs:    []types.GPUInfo{makeGPU()},
		HasTemp: true,
	}
	minH := g.SizeHint().MinH
	out := g.View(stats, 80, minH, th)
	if out == "" {
		t.Fatal("View returned empty output at MinH")
	}
	if !strings.Contains(out, "GPU") {
		t.Errorf("expected box title 'GPU', got: %q", out)
	}
	// At MinH=4: title(1) + util_meter(1) + vram_meter(1) + info_line(1) = 4 content rows.
	if !strings.Contains(out, "Test GPU") {
		t.Errorf("expected GPU name in util meter at MinH, got: %q", out)
	}
}

func TestGPUViewFullContent(t *testing.T) {
	g := New()
	th := theme.Default()
	stats := types.GPUStats{
		GPUs:     []types.GPUInfo{makeGPU()},
		HasTemp:  true,
		HasPower: true,
	}
	// At PrefH=6: all content fits comfortably.
	out := g.View(stats, 80, 6, th)
	if !strings.Contains(out, "Test GPU") {
		t.Errorf("expected GPU name in output, got: %q", out)
	}
	if !strings.Contains(out, "VRAM") {
		t.Errorf("expected VRAM meter in output, got: %q", out)
	}
	if !strings.Contains(out, "°C") {
		t.Errorf("expected temperature in info line, got: %q", out)
	}
}

func TestGPUViewHeightCapped(t *testing.T) {
	g := New()
	th := theme.Default()
	stats := types.GPUStats{
		GPUs: []types.GPUInfo{makeGPU()},
	}
	maxH := g.SizeHint().MaxH // = 6
	// Even with generous height allocation the box should not exceed MaxH content rows.
	out := g.View(stats, 80, maxH, th)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	vChrome := 2
	want := maxH + vChrome
	if len(lines) > want {
		t.Errorf("View at MaxH produced %d lines, want <= %d", len(lines), want)
	}
}
