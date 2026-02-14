package network

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/internal/ui"
	"mld.com/dtop/pkg/collector"
	"mld.com/dtop/pkg/types"
)

const minAutoScale = 10 * 1024

type Network struct {
	cfg  Config
	mu   sync.Mutex
	prev map[string]netDevCounters

	lastWidth int
	peakRx    float64
	peakTx    float64
}

func New() *Network {
	return &Network{
		cfg: Config{
			ShowIPv6: true,
		},
		prev: make(map[string]netDevCounters),
	}
}

func (n *Network) ID() plugin.ID { return "network" }
func (n *Network) Name() string  { return "Network" }
func (n *Network) SizeHint() ui.SizeHint {
	return ui.SizeHint{MinH: 3, PrefH: 8, MaxH: 0, Weight: 2}
}
func (n *Network) AllowedConfigKeys() []string {
	return []string{"interface", "show_ipv6"}
}

func (n *Network) Init(_ context.Context, cfg map[string]any) error {
	n.cfg = parseConfig(n.cfg, cfg)
	return nil
}

func (n *Network) Collect(context.Context) (collector.Data, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	stats, nextPrev, err := readNetworkStats(n.prev, n.cfg)
	if err != nil {
		return nil, err
	}
	n.prev = nextPrev

	n.peakRx = maxFloat(n.peakRx, stats.RxBytesPerSec)
	n.peakTx = maxFloat(n.peakTx, stats.TxBytesPerSec)
	stats.PeakRxBytesPerSec = n.peakRx
	stats.PeakTxBytesPerSec = n.peakTx

	return stats, nil
}

func (n *Network) Shutdown(context.Context) error { return nil }

func (n *Network) Update(tea.Msg) tea.Cmd { return nil }

func (n *Network) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.NetworkStats)
	if !ok {
		return th.RenderBox("Network", th.Muted.Render("Collecting..."), width, height)
	}
	innerWidth := ui.ContentWidth(width)
	n.mu.Lock()
	n.lastWidth = innerWidth
	n.mu.Unlock()

	lines := []string{
		n.renderHeader(&stats, innerWidth, th),
	}

	graphH := netGraphRows(height)

	// Download section
	lines = append(lines, n.renderDownloadSection(&stats, innerWidth, graphH, th)...)

	// Upload section
	lines = append(lines, n.renderUploadSection(&stats, innerWidth, graphH, th)...)

	// Footer: IPs + totals
	lines = append(lines, n.renderFooter(&stats, innerWidth, th)...)

	body := strings.Join(lines, "\n")
	return th.RenderBox("Network", body, width, height)
}

func (n *Network) renderHeader(stats *types.NetworkStats, width int, th theme.Theme) string {
	link := "down"
	if stats.LinkUp {
		link = "up"
	}
	return th.Text.Render(ui.Truncate(fmt.Sprintf("%s (%s)", stats.Interface, link), width))
}

func (n *Network) renderDownloadSection(stats *types.NetworkStats, width, height int, th theme.Theme) []string {
	var lines []string
	rxScale := autoScale(stats.RxHistory, minAutoScale)
	downPrefix := "▼ Down"
	if !th.UTF8 {
		downPrefix = "Down"
	}

	header := th.Text.Render(ui.Truncate(fmt.Sprintf("%s: %s (peak %s, scale %s)", downPrefix,
		formatRate(stats.RxBytesPerSec), formatRate(stats.PeakRxBytesPerSec), formatRate(rxScale)), width))

	if height > 0 && len(stats.RxHistory) > 0 {
		lines = append(lines, header)
		g := ui.RenderGraph(stats.RxHistory, width, height, ui.GraphOpts{
			Min: 0, Max: rxScale, Style: th.GraphNet, Fill: true, ASCII: !th.UTF8,
		})
		lines = append(lines, g)
	} else {
		// Fallback header without scale if no graph
		lines = append(lines,
			th.Text.Render(ui.Truncate(fmt.Sprintf("%s: %s (peak %s)", downPrefix,
				formatRate(stats.RxBytesPerSec), formatRate(stats.PeakRxBytesPerSec)), width)),
		)
	}
	return lines
}

func (n *Network) renderUploadSection(stats *types.NetworkStats, width, height int, th theme.Theme) []string {
	var lines []string
	txScale := autoScale(stats.TxHistory, minAutoScale)
	upPrefix := "▲ Up"
	if !th.UTF8 {
		upPrefix = "Up"
	}

	header := th.Text.Render(ui.Truncate(fmt.Sprintf("%s: %s (peak %s, scale %s)", upPrefix,
		formatRate(stats.TxBytesPerSec), formatRate(stats.PeakTxBytesPerSec), formatRate(txScale)), width))

	if height > 0 && len(stats.TxHistory) > 0 {
		lines = append(lines, header)
		g := ui.RenderGraph(stats.TxHistory, width, height, ui.GraphOpts{
			Min: 0, Max: txScale, Style: th.GraphNet, Fill: true, ASCII: !th.UTF8,
		})
		lines = append(lines, g)
	} else {
		// Fallback header without scale if no graph
		lines = append(lines,
			th.Text.Render(ui.Truncate(fmt.Sprintf("%s: %s (peak %s)", upPrefix,
				formatRate(stats.TxBytesPerSec), formatRate(stats.PeakTxBytesPerSec)), width)),
		)
	}
	return lines
}

func (n *Network) renderFooter(stats *types.NetworkStats, width int, th theme.Theme) []string {
	var lines []string
	var footer []string
	if len(stats.IPv4) > 0 {
		footer = append(footer, "IPv4: "+strings.Join(stats.IPv4, ", "))
	}
	if n.cfg.ShowIPv6 && len(stats.IPv6) > 0 {
		footer = append(footer, "IPv6: "+strings.Join(stats.IPv6, ", "))
	}
	totalDownPrefix := "▼"
	totalUpPrefix := "▲"
	if !th.UTF8 {
		totalDownPrefix = "D"
		totalUpPrefix = "U"
	}
	footer = append(footer, fmt.Sprintf("Total: %s%s %s%s",
		totalDownPrefix,
		formatBytes(float64(stats.RxBytes), false),
		totalUpPrefix,
		formatBytes(float64(stats.TxBytes), false)))

	for _, f := range footer {
		lines = append(lines, th.Muted.Render(ui.Truncate(f, width)))
	}
	return lines
}

func netGraphRows(boxHeight int) int {
	// Two graphs + labels + footer need space. Reserve ~10 lines overhead.
	inner := (boxHeight - 10) / 2
	if inner < 1 {
		return 0
	}
	if inner > 4 {
		inner = 4
	}
	return inner
}

func (n *Network) UpdateHistory(h *types.HistoryStore, data collector.Data, width int) collector.Data {
	stats, ok := data.(types.NetworkStats)
	if !ok {
		return data
	}

	h.Push("net.rx", stats.RxBytesPerSec, width)
	stats.RxHistory = h.Get("net.rx")

	h.Push("net.tx", stats.TxBytesPerSec, width)
	stats.TxHistory = h.Get("net.tx")

	return stats
}

func (n *Network) targetWidth() int {
	if n.lastWidth > 0 {
		return n.lastWidth
	}
	return 80
}

func autoScale(hist []float64, floor float64) float64 {
	maxVal := floor
	for _, v := range hist {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func formatRate(bytesPerSec float64) string {
	return fmt.Sprintf("%s/s", formatBytes(bytesPerSec, false))
}

func formatBytes(value float64, base10 bool) string {
	if value < 0 {
		value = 0
	}
	unit := 1024.0
	suffixes := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	if base10 {
		unit = 1000.0
		suffixes = []string{"B", "KB", "MB", "GB", "TB"}
	}
	idx := 0
	for value >= unit && idx < len(suffixes)-1 {
		value /= unit
		idx++
	}
	if idx == 0 || value >= 100 {
		return fmt.Sprintf("%.0f %s", value, suffixes[idx])
	}
	return fmt.Sprintf("%.1f %s", value, suffixes[idx])
}

func maxFloat(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}
