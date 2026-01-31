package process

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

type Process struct {
	cfg Config
	mu  sync.Mutex

	// Previous state for CPU calculation
	prevProc map[int]procStat
	prevSys  systemStat

	// Scrolling state
	scrollOffset int
}

func New() *Process {
	return &Process{
		cfg: Config{
			TreeView:   false,
			SortBy:     "cpu",
			MaxDisplay: 20,
		},
		prevProc: make(map[int]procStat),
	}
}

func (p *Process) ID() plugin.ID { return "process" }
func (p *Process) Name() string  { return "Processes" }
func (p *Process) AllowedConfigKeys() []string {
	return []string{"tree_view", "sort_by", "filter", "max_display"}
}

func (p *Process) Init(_ context.Context, cfg map[string]any) error {
	p.cfg = parseConfig(p.cfg, cfg)
	return nil
}

func (p *Process) Collect(context.Context) (collector.Data, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats, nextProc, nextSys, err := readProcessStats(p.prevProc, p.prevSys, p.cfg)
	if err != nil {
		return nil, err
	}

	p.prevProc = nextProc
	p.prevSys = nextSys

	return stats, nil
}

func (p *Process) Shutdown(context.Context) error {
	return nil
}

func (p *Process) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			p.mu.Lock()
			if p.scrollOffset > 0 {
				p.scrollOffset--
			}
			p.mu.Unlock()
		case "down", "j":
			p.mu.Lock()
			p.scrollOffset++
			p.mu.Unlock()
		case "pgup":
			p.mu.Lock()
			p.scrollOffset -= 10
			if p.scrollOffset < 0 {
				p.scrollOffset = 0
			}
			p.mu.Unlock()
		case "pgdown":
			p.mu.Lock()
			p.scrollOffset += 10
			p.mu.Unlock()
		case "home", "g":
			p.mu.Lock()
			p.scrollOffset = 0
			p.mu.Unlock()
		}
	}
	return nil
}

func (p *Process) View(data collector.Data, width, height int, th theme.Theme) string {
	stats, ok := data.(types.ProcessStats)
	if !ok {
		return th.RenderBox("Processes", th.Muted.Render("Collecting..."), width, height)
	}

	p.mu.Lock()
	scrollOffset := p.scrollOffset
	p.mu.Unlock()

	innerWidth := contentWidth(width)
	innerHeight := contentHeight(height)

	// Build header
	header := buildHeader(stats, innerWidth)

	// Build process list
	lines := []string{header}
	lines = append(lines, strings.Repeat("─", innerWidth))

	// Calculate available lines for processes
	availableLines := innerHeight - len(lines)
	if availableLines < 0 {
		availableLines = 0
	}

	// Apply scroll offset
	maxOffset := len(stats.Processes) - availableLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
		p.mu.Lock()
		p.scrollOffset = scrollOffset
		p.mu.Unlock()
	}

	// Render visible processes
	endIdx := scrollOffset + availableLines
	if endIdx > len(stats.Processes) {
		endIdx = len(stats.Processes)
	}

	for i := scrollOffset; i < endIdx; i++ {
		proc := stats.Processes[i]
		line := formatProcess(proc, innerWidth, p.cfg.TreeView)
		lines = append(lines, th.Text.Render(line))
	}

	// Add scroll indicator if needed
	if len(stats.Processes) > availableLines {
		scrollInfo := fmt.Sprintf(" (%d-%d of %d) ", scrollOffset+1, endIdx, len(stats.Processes))
		if len(lines) > 0 {
			lines[0] = lines[0] + th.Muted.Render(scrollInfo)
		}
	}

	body := strings.Join(lines, "\n")
	return th.RenderBox("Processes", body, width, height)
}

// buildHeader builds the header line showing sort field and filter.
func buildHeader(stats types.ProcessStats, width int) string {
	sortField := "PID"
	switch stats.SortBy {
	case types.SortByName:
		sortField = "NAME"
	case types.SortByCPU:
		sortField = "CPU"
	case types.SortByMemory:
		sortField = "MEM"
	case types.SortByThreads:
		sortField = "THR"
	case types.SortByUser:
		sortField = "USER"
	}

	header := fmt.Sprintf("Total: %d", stats.TotalCount)
	if stats.FilterStr != "" {
		header += fmt.Sprintf(" | Filter: %s (showing %d)", stats.FilterStr, stats.FilteredCount)
	}
	header += fmt.Sprintf(" | Sort: %s", sortField)

	return truncate(header, width)
}

// formatProcess formats a single process for display.
func formatProcess(proc types.ProcessInfo, width int, treeView bool) string {
	// Format: PID USER STATE CPU% MEM COMMAND
	// Adjust widths based on available space
	pidWidth := 7
	userWidth := 10
	stateWidth := 3
	cpuWidth := 6
	memWidth := 8
	fixedWidth := pidWidth + userWidth + stateWidth + cpuWidth + memWidth + 5 // spaces

	cmdWidth := width - fixedWidth
	if treeView {
		cmdWidth -= len(proc.TreePrefix)
	}
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	// Format memory in human-readable form
	memStr := formatBytes(proc.MemBytes)

	// Build command with tree prefix if enabled
	cmd := proc.Command
	if treeView && proc.TreePrefix != "" {
		cmd = proc.TreePrefix + cmd
	}

	line := fmt.Sprintf("%6d %-10s %2s %5.1f%% %7s %s",
		proc.PID,
		truncate(proc.User, userWidth),
		proc.State,
		proc.CPUPercent,
		memStr,
		truncate(cmd, cmdWidth),
	)

	return truncate(line, width)
}

// formatBytes formats bytes into human-readable form (KB, MB, GB).
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), units[exp])
}

func contentWidth(totalWidth int) int {
	// Account for box padding and border
	w := totalWidth - 4
	if w < 1 {
		return 1
	}
	return w
}

func contentHeight(totalHeight int) int {
	// Account for box title, borders, and padding
	h := totalHeight - 4
	if h < 1 {
		return 1
	}
	return h
}

func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return runewidth.Truncate(s, width, "…")
}
