package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kanozad/dtop/internal/plugin"
	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/internal/ui"
	"github.com/kanozad/dtop/pkg/collector"
	"github.com/kanozad/dtop/pkg/types"
)

type Config struct {
	ShowSwap     bool
	ShowDisks    bool
	ShowIOStat   bool
	Base10Sizes  bool
	ZFSARCCached bool
	DisksFilter  []string
}

type Memory struct {
	cfg  Config
	mu   sync.Mutex
	prev map[string]diskIOCounters

	lastWidth int
}

func New() *Memory {
	return &Memory{
		cfg: Config{
			ShowSwap:     true,
			ShowDisks:    true,
			ShowIOStat:   true,
			Base10Sizes:  false,
			ZFSARCCached: false,
		},
		prev: make(map[string]diskIOCounters),
	}
}

func (m *Memory) ID() plugin.ID { return "memory" }
func (m *Memory) Name() string  { return "Memory" }
func (m *Memory) SizeHint() ui.SizeHint {
	return ui.SizeHint{MinH: 3, PrefH: 8, MaxH: 0, Weight: 2}
}
func (m *Memory) AllowedConfigKeys() []string {
	return []string{"show_swap", "show_disks", "show_io_stat", "base_10_sizes", "zfs_arc_cached", "disks_filter"}
}

func (m *Memory) Init(_ context.Context, cfg map[string]any) error {
	m.cfg = parseConfig(m.cfg, cfg)
	return nil
}

func (m *Memory) Collect(context.Context) (collector.Data, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, nextPrev, err := readMemoryStats(m.prev, m.cfg)
	if err != nil {
		return nil, err
	}
	m.prev = nextPrev

	return stats, nil
}

func (m *Memory) Shutdown(context.Context) error { return nil }

func (m *Memory) Update(tea.Msg) tea.Cmd { return nil }

func (m *Memory) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.MemoryStats)
	if !ok {
		return th.RenderBox("Memory", th.Muted.Render("Collecting..."), width, height)
	}
	innerWidth := ui.ContentWidth(width)
	m.mu.Lock()
	m.lastWidth = innerWidth
	m.mu.Unlock()

	meterOpts := ui.MeterOpts{FillStyle: th.MeterFill, EmptyStyle: th.MeterEmpty, ASCII: !th.UTF8}
	lines := []string{}

	// RAM meter
	lines = append(lines, m.renderRAMBar(&stats, innerWidth, meterOpts))

	// Memory history graph
	if g := m.renderHistoryGraph(&stats, innerWidth, height, th); g != "" {
		lines = append(lines, g)
	}

	// Cached + ZFS ARC summary
	if s := m.renderExtraSummary(&stats, innerWidth, th); s != "" {
		lines = append(lines, s)
	}

	// Swap meter
	if m.cfg.ShowSwap && stats.SwapTotal > 0 {
		lines = append(lines, m.renderSwapBar(&stats, innerWidth, meterOpts))
	}

	// Disk stats with capacity bars
	if m.cfg.ShowDisks && len(stats.Disks) > 0 {
		lines = append(lines, m.renderDiskStats(&stats, innerWidth, th, meterOpts)...)
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("Memory", body, width, height)
}

func (m *Memory) renderRAMBar(stats *types.MemoryStats, width int, opts ui.MeterOpts) string {
	ramUsedPct := 0.0
	if stats.RAMTotal > 0 {
		ramUsedPct = float64(stats.RAMUsed) * 100.0 / float64(stats.RAMTotal)
	}
	ramLabel := fmt.Sprintf("RAM %s/%s",
		formatBytes(stats.RAMUsed, m.cfg.Base10Sizes),
		formatBytes(stats.RAMTotal, m.cfg.Base10Sizes))
	return ui.RenderMeter(ramLabel, ramUsedPct, width, opts)
}

func (m *Memory) renderSwapBar(stats *types.MemoryStats, width int, opts ui.MeterOpts) string {
	swapUsedPct := float64(stats.SwapUsed) * 100.0 / float64(stats.SwapTotal)
	swapLabel := fmt.Sprintf("Swap %s/%s",
		formatBytes(stats.SwapUsed, m.cfg.Base10Sizes),
		formatBytes(stats.SwapTotal, m.cfg.Base10Sizes))
	return ui.RenderMeter(swapLabel, swapUsedPct, width, opts)
}

func (m *Memory) renderHistoryGraph(stats *types.MemoryStats, width, height int, th theme.Theme) string {
	graphHeight := memGraphRows(height)
	if graphHeight > 0 && len(stats.MemoryHistory) > 0 {
		return ui.RenderGraph(stats.MemoryHistory, width, graphHeight, ui.GraphOpts{
			Min: 0, Max: 100, Style: th.GraphMem, Fill: true, ASCII: !th.UTF8,
		})
	}
	return ""
}

func (m *Memory) renderExtraSummary(stats *types.MemoryStats, width int, th theme.Theme) string {
	var extras []string
	if stats.RAMCached > 0 {
		extras = append(extras, fmt.Sprintf("Cached: %s", formatBytes(stats.RAMCached, m.cfg.Base10Sizes)))
	}
	if m.cfg.ZFSARCCached && stats.ZFSARCSize != nil {
		extras = append(extras, fmt.Sprintf("ZFS ARC: %s", formatBytes(*stats.ZFSARCSize, m.cfg.Base10Sizes)))
	}
	if len(extras) > 0 {
		return th.Muted.Render(ui.Truncate(strings.Join(extras, "  "), width))
	}
	return ""
}

func (m *Memory) renderDiskStats(stats *types.MemoryStats, width int, th theme.Theme, opts ui.MeterOpts) []string {
	var lines []string
	hr := "-"
	if th.UTF8 {
		hr = "─"
	}
	lines = append(lines, strings.Repeat(hr, width))
	for _, disk := range stats.Disks {
		usedPct := 0.0
		if disk.Total > 0 {
			usedPct = float64(disk.Used) * 100.0 / float64(disk.Total)
		}
		diskLabel := fmt.Sprintf("%-12s %s/%s",
			ui.Truncate(disk.MountPoint, 12),
			formatBytes(disk.Used, m.cfg.Base10Sizes),
			formatBytes(disk.Total, m.cfg.Base10Sizes))
		lines = append(lines, ui.RenderMiniMeter(diskLabel, usedPct, width, opts))

		if m.cfg.ShowIOStat {
			ioLine := fmt.Sprintf("  R: %s/s  W: %s/s",
				formatBytes(uint64(disk.ReadBytesPerSec), m.cfg.Base10Sizes),
				formatBytes(uint64(disk.WriteBytesPerSec), m.cfg.Base10Sizes))
			lines = append(lines, th.Muted.Render(ui.Truncate(ioLine, width)))
		}
	}
	return lines
}

func memGraphRows(boxHeight int) int {
	inner := boxHeight - 8
	if inner < 1 {
		return 0
	}
	if inner > 4 {
		inner = 4
	}
	return inner
}

func (m *Memory) UpdateHistory(h *types.HistoryStore, data collector.Data, width int) collector.Data {
	stats, ok := data.(types.MemoryStats)
	if !ok {
		return data
	}

	// RAM % history
	ramPct := 0.0
	if stats.RAMTotal > 0 {
		ramPct = float64(stats.RAMUsed) * 100.0 / float64(stats.RAMTotal)
	}
	h.Push("mem.ram", ramPct, width)
	stats.MemoryHistory = h.Get("mem.ram")

	// Swap % history
	swapPct := 0.0
	if stats.SwapTotal > 0 {
		swapPct = float64(stats.SwapUsed) * 100.0 / float64(stats.SwapTotal)
	}
	h.Push("mem.swap", swapPct, width)
	stats.SwapHistory = h.Get("mem.swap")

	return stats
}

func formatBytes(bytes uint64, base10 bool) string {
	if base10 {
		// Base-10: KB, MB, GB
		if bytes < 1000 {
			return fmt.Sprintf("%d B", bytes)
		} else if bytes < 1000*1000 {
			return fmt.Sprintf("%.1f KB", float64(bytes)/1000.0)
		} else if bytes < 1000*1000*1000 {
			return fmt.Sprintf("%.1f MB", float64(bytes)/(1000.0*1000.0))
		} else {
			return fmt.Sprintf("%.2f GB", float64(bytes)/(1000.0*1000.0*1000.0))
		}
	} else {
		// Base-1024: KiB, MiB, GiB
		if bytes < 1024 {
			return fmt.Sprintf("%d B", bytes)
		} else if bytes < 1024*1024 {
			return fmt.Sprintf("%.1f KiB", float64(bytes)/1024.0)
		} else if bytes < 1024*1024*1024 {
			return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024.0*1024.0))
		} else {
			return fmt.Sprintf("%.2f GiB", float64(bytes)/(1024.0*1024.0*1024.0))
		}
	}
}
