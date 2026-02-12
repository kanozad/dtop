package network

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

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

	rxHistory []float64
	txHistory []float64
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

	targetWidth := n.targetWidth()
	n.appendHistory(&stats, targetWidth)

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
	innerWidth := contentWidth(width)
	n.mu.Lock()
	n.lastWidth = innerWidth
	n.reflowHistory(&stats, innerWidth)
	n.mu.Unlock()

	link := "down"
	if stats.LinkUp {
		link = "up"
	}
	lines := []string{
		th.Text.Render(truncate(fmt.Sprintf("%s (%s)", stats.Interface, link), innerWidth)),
	}

	rxScale := autoScale(stats.RxHistory, minAutoScale)
	txScale := autoScale(stats.TxHistory, minAutoScale)

	// Download graph
	graphH := netGraphRows(height)
	if graphH > 0 && len(stats.RxHistory) > 0 {
		lines = append(lines,
			th.Text.Render(truncate(fmt.Sprintf("▼ Down: %s (peak %s, scale %s)",
				formatRate(stats.RxBytesPerSec), formatRate(stats.PeakRxBytesPerSec), formatRate(rxScale)), innerWidth)),
		)
		g := ui.RenderGraph(stats.RxHistory, innerWidth, graphH, ui.GraphOpts{
			Min: 0, Max: rxScale, Style: th.GraphNet, Fill: true,
		})
		lines = append(lines, g)
	} else {
		lines = append(lines,
			th.Text.Render(truncate(fmt.Sprintf("▼ Down: %s (peak %s)",
				formatRate(stats.RxBytesPerSec), formatRate(stats.PeakRxBytesPerSec)), innerWidth)),
		)
	}

	// Upload graph
	if graphH > 0 && len(stats.TxHistory) > 0 {
		lines = append(lines,
			th.Text.Render(truncate(fmt.Sprintf("▲ Up:   %s (peak %s, scale %s)",
				formatRate(stats.TxBytesPerSec), formatRate(stats.PeakTxBytesPerSec), formatRate(txScale)), innerWidth)),
		)
		g := ui.RenderGraph(stats.TxHistory, innerWidth, graphH, ui.GraphOpts{
			Min: 0, Max: txScale, Style: th.GraphNet, Fill: true,
		})
		lines = append(lines, g)
	} else {
		lines = append(lines,
			th.Text.Render(truncate(fmt.Sprintf("▲ Up:   %s (peak %s)",
				formatRate(stats.TxBytesPerSec), formatRate(stats.PeakTxBytesPerSec)), innerWidth)),
		)
	}

	// Footer: IPs + totals
	var footer []string
	if len(stats.IPv4) > 0 {
		footer = append(footer, "IPv4: "+strings.Join(stats.IPv4, ", "))
	}
	if n.cfg.ShowIPv6 && len(stats.IPv6) > 0 {
		footer = append(footer, "IPv6: "+strings.Join(stats.IPv6, ", "))
	}
	footer = append(footer, fmt.Sprintf("Total: ▼%s ▲%s",
		formatBytes(float64(stats.RxBytes), false),
		formatBytes(float64(stats.TxBytes), false)))
	for _, f := range footer {
		lines = append(lines, th.Muted.Render(truncate(f, innerWidth)))
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("Network", body, width, height)
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

func (n *Network) appendHistory(stats *types.NetworkStats, width int) {
	n.rxHistory = pushAndClamp(n.rxHistory, stats.RxBytesPerSec, width)
	n.txHistory = pushAndClamp(n.txHistory, stats.TxBytesPerSec, width)
	stats.RxHistory = n.rxHistory
	stats.TxHistory = n.txHistory
}

func (n *Network) reflowHistory(stats *types.NetworkStats, width int) {
	if width <= 0 {
		return
	}
	n.rxHistory = resizeHistory(n.rxHistory, width)
	n.txHistory = resizeHistory(n.txHistory, width)
	stats.RxHistory = n.rxHistory
	stats.TxHistory = n.txHistory
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
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
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

func contentWidth(totalWidth int) int {
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

func maxFloat(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}
