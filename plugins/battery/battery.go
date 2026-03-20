package battery

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/internal/ui"
	"mld.com/dtop/pkg/collector"
	"mld.com/dtop/pkg/types"
)

// Battery implements plugin.Plugin for battery monitoring.
type Battery struct {
	cfg Config
}

func New() *Battery {
	return &Battery{}
}

func (b *Battery) ID() plugin.ID               { return "battery" }
func (b *Battery) Name() string                { return "Battery" }
func (b *Battery) AllowedConfigKeys() []string { return nil }
func (b *Battery) SizeHint() ui.SizeHint {
	return ui.SizeHint{MinH: 3, PrefH: 3, MaxH: 3, Weight: 2}
}

func (b *Battery) Init(_ context.Context, cfg map[string]any) error {
	b.cfg = parseConfig(b.cfg, cfg)
	return nil
}

func (b *Battery) Collect(context.Context) (collector.Data, error) {
	return readBatteryStats()
}

func (b *Battery) Shutdown(context.Context) error { return nil }

func (b *Battery) Update(tea.Msg) tea.Cmd { return nil }

func (b *Battery) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.BatteryStats)
	if !ok {
		return th.RenderBox("Battery", th.Muted.Render("Collecting..."), width, height)
	}

	innerWidth := ui.ContentWidth(width)

	if !stats.Present {
		body := th.Muted.Render(ui.Truncate("No battery detected", innerWidth))
		return th.RenderBox("Battery", body, width, height)
	}

	if stats.Error != "" {
		body := th.Error.Render(ui.Truncate(stats.Error, innerWidth))
		return th.RenderBox("Battery", body, width, height)
	}

	meterOpts := ui.MeterOpts{FillStyle: th.MeterFill, EmptyStyle: th.MeterEmpty, ASCII: !th.UTF8}
	var lines []string

	// Capacity meter bar.
	lines = append(lines, ui.RenderMeter("BAT", stats.Capacity, innerWidth, meterOpts))

	// Status line: status + power draw + time estimate.
	var info []string
	info = append(info, stats.Status)

	if stats.PowerNowWatts != nil {
		info = append(info, fmt.Sprintf("%.1fW", *stats.PowerNowWatts))
	}

	if stats.TimeToEmpty != nil {
		info = append(info, fmt.Sprintf("%s remaining", formatDuration(*stats.TimeToEmpty)))
	}
	if stats.TimeToFull != nil {
		info = append(info, fmt.Sprintf("%s to full", formatDuration(*stats.TimeToFull)))
	}

	lines = append(lines, th.Muted.Render(ui.Truncate(strings.Join(info, "  "), innerWidth)))

	// Energy detail line.
	if stats.EnergyNowWh != nil && stats.EnergyFullWh != nil {
		detail := fmt.Sprintf("%.1f / %.1f Wh", *stats.EnergyNowWh, *stats.EnergyFullWh)
		lines = append(lines, th.Muted.Render(ui.Truncate(detail, innerWidth)))
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("Battery", body, width, height)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
