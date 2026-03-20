package cpu

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/internal/ui"
	"mld.com/dtop/pkg/collector"
	"mld.com/dtop/pkg/types"
)

type Config struct {
	PerCore   bool
	ShowTemp  bool
	ShowFreq  bool
	ShowWatts bool
}

type CPU struct {
	cfg  Config
	mu   sync.Mutex
	prev map[string]cpuTimes

	// RAPL energy tracking for power calculation.
	prevRAPLEnergy uint64
	prevRAPLTime   time.Time

	lastWidth int
}

func New() *CPU {
	return &CPU{
		cfg: Config{
			PerCore:   true,
			ShowTemp:  false,
			ShowFreq:  true,
			ShowWatts: false,
		},
	}
}

func (c *CPU) ID() plugin.ID { return "cpu" }
func (c *CPU) Name() string  { return "CPU" }
func (c *CPU) SizeHint() ui.SizeHint {
	return ui.SizeHint{MinH: 3, PrefH: 12, MaxH: 0, Weight: 3}
}
func (c *CPU) AllowedConfigKeys() []string {
	return []string{"per_core", "show_temp", "show_freq", "show_watts"}
}

func (c *CPU) Init(_ context.Context, cfg map[string]any) error {
	c.cfg = parseConfig(c.cfg, cfg)
	return nil
}

func (c *CPU) Collect(context.Context) (collector.Data, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	opts := collectOpts{
		showTemp:       c.cfg.ShowTemp,
		showFreq:       c.cfg.ShowFreq,
		showWatts:      c.cfg.ShowWatts,
		prevRAPLEnergy: c.prevRAPLEnergy,
		prevRAPLTime:   c.prevRAPLTime,
	}
	stats, nextPrev, raplState, err := readCPUStats(c.prev, opts)
	if err != nil {
		return nil, err
	}
	c.prev = nextPrev
	c.prevRAPLEnergy = raplState.energy
	c.prevRAPLTime = raplState.timestamp

	return stats, nil
}

func (c *CPU) Shutdown(context.Context) error { return nil }

func (c *CPU) Update(tea.Msg) tea.Cmd { return nil }

func (c *CPU) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.CPUStats)
	if !ok {
		return th.RenderBox("CPU", th.Muted.Render("Collecting..."), width, height)
	}
	innerWidth := ui.ContentWidth(width)
	c.mu.Lock()
	c.lastWidth = innerWidth
	c.mu.Unlock()

	meterOpts := ui.MeterOpts{FillStyle: th.MeterFill, EmptyStyle: th.MeterEmpty, ASCII: !th.UTF8}
	lines := []string{
		c.renderMainMeter(&stats, innerWidth, meterOpts),
	}

	// Braille graph of total CPU history.
	if g := c.renderHistoryGraph(&stats, innerWidth, height, th); g != "" {
		lines = append(lines, g)
	}

	// Summary line: load averages + optional temp, freq, watts.
	lines = append(lines, c.renderSummaryLine(&stats, innerWidth, th))

	// Per-core mini meters.
	if c.cfg.PerCore && len(stats.PerCore) > 0 {
		coreLines := renderCoreBars(stats.PerCore, innerWidth, meterOpts)
		lines = append(lines, coreLines...)
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("CPU", body, width, height)
}

func (c *CPU) renderMainMeter(stats *types.CPUStats, width int, opts ui.MeterOpts) string {
	return ui.RenderMeter("CPU", stats.Total, width, opts)
}

func (c *CPU) renderHistoryGraph(stats *types.CPUStats, width, height int, th theme.Theme) string {
	coreBars := 0
	if c.cfg.PerCore {
		coreBars = coreBarCount(stats.PerCore)
	}
	graphHeight := graphRows(height, coreBars)
	if graphHeight > 0 && len(stats.TotalHistory) > 0 {
		return ui.RenderGraph(stats.TotalHistory, width, graphHeight, ui.GraphOpts{
			Min: 0, Max: 100, Style: th.GraphCPU, Fill: true, ASCII: !th.UTF8,
		})
	}
	return ""
}

func (c *CPU) renderSummaryLine(stats *types.CPUStats, width int, th theme.Theme) string {
	summary := fmt.Sprintf("Load: %.2f %.2f %.2f", stats.Load1, stats.Load5, stats.Load15)
	if c.cfg.ShowTemp && stats.TemperatureC != nil {
		if th.UTF8 {
			summary += fmt.Sprintf("  Temp: %.1f°C", *stats.TemperatureC)
		} else {
			summary += fmt.Sprintf("  Temp: %.1f C", *stats.TemperatureC)
		}
	}
	if c.cfg.ShowFreq && stats.FrequencyAvgMHz > 0 {
		summary += fmt.Sprintf("  Freq: %.0f MHz", stats.FrequencyAvgMHz)
	}
	if c.cfg.ShowWatts && stats.PowerWatts != nil {
		summary += fmt.Sprintf("  %.1fW", *stats.PowerWatts)
	}
	if stats.ContainerType != "" {
		if stats.EffectiveCPUs > 0 {
			summary += fmt.Sprintf("  [%s %d CPUs]", stats.ContainerType, stats.EffectiveCPUs)
		} else {
			summary += fmt.Sprintf("  [%s]", stats.ContainerType)
		}
	}
	return th.Muted.Render(ui.Truncate(summary, width))
}

// graphRows computes how many braille rows to allocate for the graph.
// h is the inner content height (rows) passed to View.
// coreBarRows is the number of per-core bar rows that will be rendered below the graph.
// Returns 0 if there is not enough room.
func graphRows(h, coreBarRows int) int {
	// Inner content: title(1) + meter(1) + summary(1) + coreBarRows + graphRows = 3 + coreBarRows + graphRows.
	inner := h - 3 - coreBarRows
	if inner < 1 {
		return 0
	}
	// Cap graph at 6 rows (24 dots of vertical resolution).
	if inner > 6 {
		inner = 6
	}
	return inner
}

// coreBarCount returns the number of rendered rows needed for per-core bars.
// Cores are displayed in a single column up to 4 cores, two columns above that.
func coreBarCount(cores []float64) int {
	n := len(cores)
	if n == 0 {
		return 0
	}
	if n <= 4 {
		return n
	}
	return (n + 1) / 2
}

// renderCoreBars renders per-core mini meters, using 2 columns if there are
// more than 4 cores.
func renderCoreBars(perCore []float64, width int, opts ui.MeterOpts) []string {
	if len(perCore) <= 4 {
		lines := make([]string, len(perCore))
		for i, v := range perCore {
			label := fmt.Sprintf("cpu%-2d", i)
			lines[i] = ui.RenderMiniMeter(label, v, width, opts)
		}
		return lines
	}
	// Two-column layout.
	colWidth := width / 2
	var lines []string
	for i := 0; i < len(perCore); i += 2 {
		left := ui.RenderMiniMeter(fmt.Sprintf("cpu%-2d", i), perCore[i], colWidth, opts)
		if i+1 < len(perCore) {
			right := ui.RenderMiniMeter(fmt.Sprintf("cpu%-2d", i+1), perCore[i+1], colWidth, opts)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
		} else {
			lines = append(lines, left)
		}
	}
	return lines
}

func (c *CPU) UpdateHistory(h *types.HistoryStore, data collector.Data, width int) collector.Data {
	stats, ok := data.(types.CPUStats)
	if !ok {
		return data
	}

	// Total CPU history
	h.Push("cpu.total", stats.Total, width)
	stats.TotalHistory = h.Get("cpu.total")

	// Per-core history
	if len(stats.PerCore) > 0 {
		stats.PerCoreHistory = make([][]float64, len(stats.PerCore))
		for i, v := range stats.PerCore {
			name := fmt.Sprintf("cpu.core.%d", i)
			h.Push(name, v, width)
			stats.PerCoreHistory[i] = h.Get(name)
		}
	}

	return stats
}

func (c *CPU) targetWidth() int {
	if c.lastWidth > 0 {
		return c.lastWidth
	}
	return 80
}
