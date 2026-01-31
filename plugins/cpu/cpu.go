package cpu

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
	PerCore  bool
	ShowTemp bool
}

type CPU struct {
	cfg  Config
	mu   sync.Mutex
	prev map[string]cpuTimes

	// histories sized to current viewport width (inner content width)
	totalHistory   []float64
	perCoreHistory [][]float64
	lastWidth      int
}

func New() *CPU {
	return &CPU{
		cfg: Config{
			PerCore:  true,
			ShowTemp: false,
		},
	}
}

func (c *CPU) ID() plugin.ID { return "cpu" }
func (c *CPU) Name() string  { return "CPU" }
func (c *CPU) AllowedConfigKeys() []string {
	return []string{"per_core", "show_temp"}
}

func (c *CPU) Init(_ context.Context, cfg map[string]any) error {
	c.cfg = parseConfig(c.cfg, cfg)
	return nil
}

func (c *CPU) Collect(context.Context) (collector.Data, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, nextPrev, err := readCPUStats(c.prev, c.cfg.ShowTemp)
	if err != nil {
		return nil, err
	}
	c.prev = nextPrev

	targetWidth := c.targetWidth()
	c.appendHistory(&stats, targetWidth)

	return stats, nil
}

func (c *CPU) Shutdown(context.Context) error { return nil }

func (c *CPU) Update(tea.Msg) tea.Cmd { return nil }

func (c *CPU) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.CPUStats)
	if !ok {
		return th.RenderBox("CPU", th.Muted.Render("Collecting..."), width, height)
	}
	innerWidth := contentWidth(width)
	c.mu.Lock()
	c.lastWidth = innerWidth
	c.reflowHistory(&stats, innerWidth)
	c.mu.Unlock()

	lines := []string{
		th.Text.Render(truncate(fmt.Sprintf("Total: %.1f%%", stats.Total), innerWidth)),
		th.Text.Render(truncate(fmt.Sprintf("Load: %.2f %.2f %.2f", stats.Load1, stats.Load5, stats.Load15), innerWidth)),
	}
	if c.cfg.ShowTemp && stats.TemperatureC != nil {
		lines = append(lines, th.Text.Render(truncate(fmt.Sprintf("Temp: %.1f°C", *stats.TemperatureC), innerWidth)))
	}
	if c.cfg.PerCore && len(stats.PerCore) > 0 {
		for i, v := range stats.PerCore {
			lines = append(lines, th.Text.Render(truncate(fmt.Sprintf("cpu%d: %5.1f%%", i, v), innerWidth)))
		}
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("CPU", body, width, height)
}

func (c *CPU) appendHistory(stats *types.CPUStats, width int) {
	// total history
	c.totalHistory = pushAndClamp(c.totalHistory, stats.Total, width)
	stats.TotalHistory = c.totalHistory

	// per-core history
	if len(stats.PerCoreHistory) == 0 && len(stats.PerCore) > 0 {
		stats.PerCoreHistory = make([][]float64, len(stats.PerCore))
	}
	// Ensure capacity and alignment with core count
	if len(c.perCoreHistory) != len(stats.PerCore) {
		c.perCoreHistory = make([][]float64, len(stats.PerCore))
	}
	for i, v := range stats.PerCore {
		c.perCoreHistory[i] = pushAndClamp(c.perCoreHistory[i], v, width)
		stats.PerCoreHistory[i] = c.perCoreHistory[i]
	}
}

func (c *CPU) reflowHistory(stats *types.CPUStats, width int) {
	if width <= 0 {
		return
	}
	c.totalHistory = resizeHistory(c.totalHistory, width)
	stats.TotalHistory = c.totalHistory

	if len(stats.PerCore) > 0 && len(c.perCoreHistory) != len(stats.PerCore) {
		// align core histories on core-count changes
		c.perCoreHistory = make([][]float64, len(stats.PerCore))
	}
	stats.PerCoreHistory = make([][]float64, len(c.perCoreHistory))
	for i := range c.perCoreHistory {
		c.perCoreHistory[i] = resizeHistory(c.perCoreHistory[i], width)
		stats.PerCoreHistory[i] = c.perCoreHistory[i]
	}
}

func (c *CPU) targetWidth() int {
	if c.lastWidth > 0 {
		return c.lastWidth
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
