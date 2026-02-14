package gpu

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/internal/ui"
	"mld.com/dtop/pkg/collector"
	"mld.com/dtop/pkg/types"
)

// GPU implements plugin.Plugin for GPU monitoring.
type GPU struct {
	cfg Config
}

func New() *GPU {
	return &GPU{
		cfg: Config{
			ShowEncoder: false,
			ShowPCIe:    false,
		},
	}
}

func (g *GPU) ID() plugin.ID { return "gpu" }
func (g *GPU) Name() string  { return "GPU" }
func (g *GPU) SizeHint() ui.SizeHint {
	return ui.SizeHint{MinH: 4, PrefH: 6, MaxH: 0, Weight: 1}
}
func (g *GPU) AllowedConfigKeys() []string {
	return []string{"show_encoder", "show_pcie"}
}

func (g *GPU) Init(_ context.Context, cfg map[string]any) error {
	g.cfg = parseConfig(g.cfg, cfg)
	return nil
}

func (g *GPU) Collect(context.Context) (collector.Data, error) {
	return readGPUStats()
}

func (g *GPU) Shutdown(context.Context) error {
	shutdownNVML()
	shutdownROCm()
	return nil
}

func (g *GPU) Update(tea.Msg) tea.Cmd { return nil }

func (g *GPU) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.GPUStats)
	if !ok {
		return th.RenderBox("GPU", th.Muted.Render("Collecting..."), width, height)
	}

	innerWidth := ui.ContentWidth(width)

	if stats.Error != "" && len(stats.GPUs) == 0 {
		body := th.Muted.Render(ui.Truncate("N/A: "+stats.Error, innerWidth))
		return th.RenderBox("GPU", body, width, height)
	}

	meterOpts := ui.MeterOpts{FillStyle: th.MeterFill, EmptyStyle: th.MeterEmpty, ASCII: !th.UTF8}
	var lines []string

	for _, gpu := range stats.GPUs {
		// GPU name + utilization meter
		label := ui.Truncate(gpu.Name, 20)
		lines = append(lines, ui.RenderMeter(label, gpu.UtilizationPct, innerWidth, meterOpts))

		// Memory meter
		if gpu.MemoryTotal > 0 {
			memLabel := fmt.Sprintf("VRAM %s/%s",
				formatBytes(gpu.MemoryUsed), formatBytes(gpu.MemoryTotal))
			lines = append(lines, ui.RenderMiniMeter(memLabel, gpu.MemoryPct, innerWidth, meterOpts))
		}

		// Info line: temp + power + clocks
		var info []string
		if stats.HasTemp && gpu.TemperatureC >= 0 {
			if th.UTF8 {
				info = append(info, fmt.Sprintf("%.0f°C", gpu.TemperatureC))
			} else {
				info = append(info, fmt.Sprintf("%.0f C", gpu.TemperatureC))
			}
		}
		if stats.HasPower && gpu.PowerWatts >= 0 {
			pw := fmt.Sprintf("%.1fW", gpu.PowerWatts)
			if gpu.PowerCapWatts > 0 {
				pw += fmt.Sprintf("/%.0fW", gpu.PowerCapWatts)
			}
			info = append(info, pw)
		}
		if gpu.ClockCoreMHz > 0 {
			info = append(info, fmt.Sprintf("%d MHz", gpu.ClockCoreMHz))
		}
		if g.cfg.ShowEncoder && stats.HasEncoder {
			if gpu.EncoderPct >= 0 {
				info = append(info, fmt.Sprintf("Enc: %.0f%%", gpu.EncoderPct))
			}
			if gpu.DecoderPct >= 0 {
				info = append(info, fmt.Sprintf("Dec: %.0f%%", gpu.DecoderPct))
			}
		}
		if g.cfg.ShowPCIe && stats.HasPCIe {
			if gpu.PCIeTxMBps >= 0 {
				info = append(info, fmt.Sprintf("TX: %.0f MB/s", gpu.PCIeTxMBps))
			}
			if gpu.PCIeRxMBps >= 0 {
				info = append(info, fmt.Sprintf("RX: %.0f MB/s", gpu.PCIeRxMBps))
			}
		}
		if len(info) > 0 {
			lines = append(lines, th.Muted.Render(ui.Truncate(strings.Join(info, "  "), innerWidth)))
		}
	}

	if stats.Error != "" {
		lines = append(lines, th.Error.Render(ui.Truncate(stats.Error, innerWidth)))
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("GPU", body, width, height)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}
