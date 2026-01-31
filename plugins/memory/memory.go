package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/collector"
	"mld.com/dtop/pkg/types"
)

type Config struct {
	ShowSwap      bool
	ShowDisks     bool
	ShowIOStat    bool
	Base10Sizes   bool
	ZFSARCCached  bool
	DisksFilter   []string
}

type Memory struct {
	cfg  Config
	mu   sync.Mutex
	prev map[string]diskIOCounters

	// histories sized to current viewport width (inner content width)
	memoryHistory []float64
	swapHistory   []float64
	lastWidth     int
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

	targetWidth := m.targetWidth()
	m.appendHistory(&stats, targetWidth)

	return stats, nil
}

func (m *Memory) Shutdown(context.Context) error { return nil }

func (m *Memory) Update(tea.Msg) tea.Cmd { return nil }

func (m *Memory) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.MemoryStats)
	if !ok {
		return th.RenderBox("Memory", th.Muted.Render("Collecting..."), width, height)
	}
	innerWidth := contentWidth(width)
	m.mu.Lock()
	m.lastWidth = innerWidth
	m.reflowHistory(&stats, innerWidth)
	m.mu.Unlock()

	lines := []string{}

	// RAM stats
	ramUsedPct := 0.0
	if stats.RAMTotal > 0 {
		ramUsedPct = float64(stats.RAMUsed) * 100.0 / float64(stats.RAMTotal)
	}
	lines = append(lines, th.Text.Render(truncate(
		fmt.Sprintf("RAM: %s / %s (%.1f%%)",
			formatBytes(stats.RAMUsed, m.cfg.Base10Sizes),
			formatBytes(stats.RAMTotal, m.cfg.Base10Sizes),
			ramUsedPct),
		innerWidth)))

	// Cached
	if stats.RAMCached > 0 {
		lines = append(lines, th.Text.Render(truncate(
			fmt.Sprintf("Cached: %s", formatBytes(stats.RAMCached, m.cfg.Base10Sizes)),
			innerWidth)))
	}

	// ZFS ARC
	if m.cfg.ZFSARCCached && stats.ZFSARCSize != nil {
		lines = append(lines, th.Text.Render(truncate(
			fmt.Sprintf("ZFS ARC: %s", formatBytes(*stats.ZFSARCSize, m.cfg.Base10Sizes)),
			innerWidth)))
	}

	// Swap stats
	if m.cfg.ShowSwap && stats.SwapTotal > 0 {
		swapUsedPct := 0.0
		if stats.SwapTotal > 0 {
			swapUsedPct = float64(stats.SwapUsed) * 100.0 / float64(stats.SwapTotal)
		}
		lines = append(lines, th.Text.Render(truncate(
			fmt.Sprintf("Swap: %s / %s (%.1f%%)",
				formatBytes(stats.SwapUsed, m.cfg.Base10Sizes),
				formatBytes(stats.SwapTotal, m.cfg.Base10Sizes),
				swapUsedPct),
			innerWidth)))
	}

	// Disk stats
	if m.cfg.ShowDisks && len(stats.Disks) > 0 {
		lines = append(lines, "") // blank line separator
		for _, disk := range stats.Disks {
			usedPct := 0.0
			if disk.Total > 0 {
				usedPct = float64(disk.Used) * 100.0 / float64(disk.Total)
			}
			diskLine := fmt.Sprintf("%s: %s / %s (%.1f%%)",
				truncate(disk.MountPoint, 15),
				formatBytes(disk.Used, m.cfg.Base10Sizes),
				formatBytes(disk.Total, m.cfg.Base10Sizes),
				usedPct)
			lines = append(lines, th.Text.Render(truncate(diskLine, innerWidth)))

			// I/O stats
			if m.cfg.ShowIOStat {
				ioLine := fmt.Sprintf("  R: %s/s  W: %s/s",
					formatBytes(uint64(disk.ReadBytesPerSec), m.cfg.Base10Sizes),
					formatBytes(uint64(disk.WriteBytesPerSec), m.cfg.Base10Sizes))
				lines = append(lines, th.Muted.Render(truncate(ioLine, innerWidth)))
			}
		}
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("Memory", body, width, height)
}

func (m *Memory) appendHistory(stats *types.MemoryStats, width int) {
	// memory % history
	memPct := 0.0
	if stats.RAMTotal > 0 {
		memPct = float64(stats.RAMUsed) * 100.0 / float64(stats.RAMTotal)
	}
	m.memoryHistory = pushAndClamp(m.memoryHistory, memPct, width)
	stats.MemoryHistory = m.memoryHistory

	// swap % history
	swapPct := 0.0
	if stats.SwapTotal > 0 {
		swapPct = float64(stats.SwapUsed) * 100.0 / float64(stats.SwapTotal)
	}
	m.swapHistory = pushAndClamp(m.swapHistory, swapPct, width)
	stats.SwapHistory = m.swapHistory
}

func (m *Memory) reflowHistory(stats *types.MemoryStats, width int) {
	if width <= 0 {
		return
	}
	m.memoryHistory = resizeHistory(m.memoryHistory, width)
	stats.MemoryHistory = m.memoryHistory

	m.swapHistory = resizeHistory(m.swapHistory, width)
	stats.SwapHistory = m.swapHistory
}

func (m *Memory) targetWidth() int {
	if m.lastWidth > 0 {
		return m.lastWidth
	}
	return 80
}

func pushAndClamp(hist []float64, value float64, width int) []float64 {
	if width <= 0 {
		return hist
	}
	hist = append(hist, value)
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
	return hist
}

func resizeHistory(hist []float64, width int) []float64 {
	if width <= 0 {
		return hist
	}
	// trim
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
	// pad with last value if growing
	if len(hist) < width {
		padVal := 0.0
		if len(hist) > 0 {
			padVal = hist[len(hist)-1]
		}
		for len(hist) < width {
			hist = append(hist, padVal)
		}
	}
	return hist
}

func contentWidth(totalWidth int) int {
	// Account for box padding (default 1 left/right) and border.
	w := totalWidth - 4
	if w < 1 {
		return 1
	}
	return w
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return runewidth.Truncate(s, width, "…")
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
