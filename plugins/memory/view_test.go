package memory

import (
	"strings"
	"testing"

	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/internal/ui"
	"github.com/kanozad/dtop/pkg/types"
)

func TestRenderRAMBar(t *testing.T) {
	m := &Memory{}
	stats := &types.MemoryStats{
		RAMTotal: 16 * 1024 * 1024 * 1024,
		RAMUsed:  8 * 1024 * 1024 * 1024,
	}
	width := 50
	opts := ui.MeterOpts{ASCII: true}

	rendered := m.renderRAMBar(stats, width, opts)
	if !strings.Contains(rendered, "RAM") {
		t.Errorf("Expected 'RAM' in rendered bar, got: %q", rendered)
	}
	if !strings.Contains(rendered, "8.00 GiB/16.00 GiB") {
		t.Errorf("Expected formatted memory sizes, got: %q", rendered)
	}
}

func TestRenderExtraSummary(t *testing.T) {
	m := &Memory{}
	stats := &types.MemoryStats{
		RAMCached: 4 * 1024 * 1024 * 1024,
		RAMFree:   2 * 1024 * 1024 * 1024,
	}
	width := 50
	th := theme.Default()

	rendered := m.renderExtraSummary(stats, width, th)
	if !strings.Contains(rendered, "Cached: 4.00 GiB") {
		t.Errorf("Expected 'Cached: 4.00 GiB', got: %q", rendered)
	}
	// Note: renderExtraSummary only shows Free if it's coded to. Let's check.
}

func TestMemGraphRows(t *testing.T) {
	tests := []struct {
		height   int
		expected int
	}{
		{8, 0},
		{9, 1},
		{10, 2},
		{12, 4},
		{20, 4},
	}

	for _, tt := range tests {
		result := memGraphRows(tt.height)
		if result != tt.expected {
			t.Errorf("memGraphRows(%d) = %d, want %d", tt.height, result, tt.expected)
		}
	}
}
