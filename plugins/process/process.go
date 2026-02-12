package process

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

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
	scrollOffset  int
	pageSize      int
	selectedPID   int
	followPID     int
	treeCollapsed map[int]struct{}
	lastProcesses []types.ProcessInfo

	statusMsg string
	statusErr bool
	statusAt  time.Time
}

func New() *Process {
	return &Process{
		cfg: Config{
			TreeView:   false,
			SortBy:     "cpu",
			MaxDisplay: 20,
		},
		prevProc:      make(map[int]procStat),
		treeCollapsed: make(map[int]struct{}),
	}
}

func (p *Process) ID() plugin.ID { return "process" }
func (p *Process) Name() string  { return "Processes" }
func (p *Process) AllowedConfigKeys() []string {
	return []string{"tree_view", "sort_by", "filter", "max_display", "follow_pid", "use_smaps"}
}

func (p *Process) Init(_ context.Context, cfg map[string]any) error {
	p.cfg = parseConfig(p.cfg, cfg)
	p.followPID = p.cfg.FollowPID
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
			p.moveSelection(-1)
		case "down", "j":
			p.moveSelection(1)
		case "pgup":
			p.moveSelection(-p.pageStep())
		case "pgdown":
			p.moveSelection(p.pageStep())
		case "home", "g":
			p.moveSelectionToStart()
		case "end", "G":
			p.moveSelectionToEnd()
		case "f":
			p.toggleFollow()
		case "c":
			p.toggleCollapse()
		case "x":
			p.sendSelectedSignal(syscall.SIGTERM)
		case "X":
			p.sendSelectedSignal(syscall.SIGKILL)
		case "r":
			p.reniceSelected(1)
		case "R":
			p.reniceSelected(-1)
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
	followPID := p.followPID
	selectedPID := p.selectedPID
	cfg := p.cfg
	collapsed := copyCollapsed(p.treeCollapsed)
	statusMsg := p.statusMsg
	statusErr := p.statusErr
	statusAt := p.statusAt
	p.mu.Unlock()

	innerWidth := contentWidth(width)
	innerHeight := contentHeight(height)

	if statusMsg != "" && time.Since(statusAt) > statusTTL {
		p.clearStatus()
		statusMsg = ""
		statusErr = false
	}

	processes := stats.Processes
	if followPID > 0 {
		filtered := filterFollow(processes, followPID, cfg.TreeView)
		if len(filtered) == 0 {
			p.setStatus(fmt.Sprintf("follow pid %d not found", followPID), true)
			p.mu.Lock()
			p.followPID = 0
			p.mu.Unlock()
			followPID = 0
		} else {
			processes = filtered
		}
	}

	if cfg.TreeView {
		processes = applyTreeCollapse(processes, collapsed)
	}

	if cfg.MaxDisplay > 0 && len(processes) > cfg.MaxDisplay {
		processes = processes[:cfg.MaxDisplay]
	}

	// Build header
	header := buildHeader(stats, innerWidth, len(processes), followPID, cfg.UseSmaps)
	if statusMsg != "" {
		if statusErr {
			header += " | ! " + statusMsg
		} else {
			header += " | " + statusMsg
		}
		header = truncate(header, innerWidth)
	}

	// Build process list
	lines := []string{header}
	lines = append(lines, buildColumnHeader(stats.SortBy, innerWidth, th))
	lines = append(lines, strings.Repeat("─", innerWidth))

	// Calculate available lines for processes (account for the extra column header line)
	availableLines := innerHeight - len(lines)
	if availableLines < 0 {
		availableLines = 0
	}

	selectedIndex := indexByPID(processes, selectedPID)
	if selectedIndex < 0 && len(processes) > 0 {
		selectedIndex = 0
		selectedPID = processes[0].PID
	}

	// Apply scroll offset
	maxOffset := len(processes) - availableLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if selectedIndex >= 0 {
		if selectedIndex < scrollOffset {
			scrollOffset = selectedIndex
		} else if selectedIndex >= scrollOffset+availableLines && availableLines > 0 {
			scrollOffset = selectedIndex - availableLines + 1
		}
	}
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Render visible processes
	endIdx := scrollOffset + availableLines
	if endIdx > len(processes) {
		endIdx = len(processes)
	}

	for i := scrollOffset; i < endIdx; i++ {
		proc := processes[i]
		selected := i == selectedIndex
		line := formatProcess(proc, innerWidth, cfg.TreeView, false)
		if selected {
			lines = append(lines, th.Highlight.Render(truncate(line, innerWidth)))
		} else {
			lines = append(lines, th.Text.Render(line))
		}
	}

	// Add scroll indicator if needed
	if len(processes) > availableLines {
		scrollInfo := fmt.Sprintf(" (%d-%d of %d) ", scrollOffset+1, endIdx, len(processes))
		if len(lines) > 0 {
			lines[0] = lines[0] + th.Muted.Render(scrollInfo)
		}
	}

	body := strings.Join(lines, "\n")
	p.mu.Lock()
	p.scrollOffset = scrollOffset
	p.pageSize = availableLines
	p.selectedPID = selectedPID
	p.lastProcesses = append([]types.ProcessInfo(nil), processes...)
	p.treeCollapsed = pruneCollapsed(stats.Processes, collapsed)
	p.mu.Unlock()
	return th.RenderBox("Processes", body, width, height)
}

// buildColumnHeader renders the column header row with sort indicator.
func buildColumnHeader(sortBy types.ProcessSortField, width int, th theme.Theme) string {
	cols := []struct {
		name  string
		field types.ProcessSortField
		w     int
	}{
		{"PID", types.SortByPID, 8},
		{"USER", types.SortByUser, 11},
		{"S", types.ProcessSortField(-1), 3}, // state, not sortable by this name
		{"CPU%", types.SortByCPU, 7},
		{"MEM", types.SortByMemory, 8},
		{"CMD", types.SortByName, 0}, // fills remaining
	}
	var parts []string
	for _, c := range cols {
		label := c.name
		if c.field == sortBy {
			label += "▾"
		}
		if c.w > 0 {
			parts = append(parts, fmt.Sprintf("%-*s", c.w, label))
		} else {
			parts = append(parts, label)
		}
	}
	line := "  " + strings.Join(parts, "")
	return th.Muted.Render(truncate(line, width))
}

// buildHeader builds the header line showing sort field and filter.
func buildHeader(stats types.ProcessStats, width int, displayCount int, followPID int, useSmaps bool) string {
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
		header += fmt.Sprintf(" | Filter: %s (%d)", stats.FilterStr, stats.FilteredCount)
	}
	if displayCount > 0 && displayCount != stats.FilteredCount {
		header += fmt.Sprintf(" | Showing: %d", displayCount)
	}
	if followPID > 0 {
		header += fmt.Sprintf(" | Follow: %d", followPID)
	}
	if useSmaps {
		header += " | Smaps"
	}
	header += fmt.Sprintf(" | Sort: %s", sortField)

	return truncate(header, width)
}

// formatProcess formats a single process for display.
func formatProcess(proc types.ProcessInfo, width int, treeView bool, selected bool) string {
	// Format: PID USER STATE CPU% MEM COMMAND
	// Adjust widths based on available space
	indicator := "  "
	if selected {
		indicator = "> "
	}
	pidWidth := 7
	userWidth := 10
	stateWidth := 3
	cpuWidth := 6
	memWidth := 8
	fixedWidth := pidWidth + userWidth + stateWidth + cpuWidth + memWidth + 5 + len(indicator) // spaces

	cmdWidth := width - fixedWidth
	if treeView {
		cmdWidth -= len(proc.TreePrefix)
	}
	if treeView && proc.TreeCollapsed {
		cmdWidth -= len("▸ ")
	}
	if cmdWidth < 10 {
		cmdWidth = 10
	}

	// Format memory in human-readable form
	memStr := formatBytes(proc.MemBytes)

	// Build command with tree prefix if enabled
	cmd := proc.Command
	if treeView {
		prefix := proc.TreePrefix
		if proc.TreeCollapsed {
			prefix += "▸ "
		}
		if prefix != "" {
			cmd = prefix + cmd
		}
	}

	line := fmt.Sprintf("%s%6d %-10s %2s %5.1f%% %7s %s",
		indicator,
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

const statusTTL = 5 * time.Second

func copyCollapsed(src map[int]struct{}) map[int]struct{} {
	if len(src) == 0 {
		return map[int]struct{}{}
	}
	dst := make(map[int]struct{}, len(src))
	for pid := range src {
		dst[pid] = struct{}{}
	}
	return dst
}

func (p *Process) setStatus(msg string, isErr bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statusMsg = msg
	p.statusErr = isErr
	p.statusAt = time.Now()
}

func (p *Process) clearStatus() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.statusMsg == "" {
		return
	}
	if time.Since(p.statusAt) > statusTTL {
		p.statusMsg = ""
		p.statusErr = false
		p.statusAt = time.Time{}
	}
}

func (p *Process) pageStep() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pageSize > 0 {
		return p.pageSize
	}
	return 10
}

func (p *Process) moveSelection(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lastProcesses) == 0 {
		return
	}
	idx := indexByPID(p.lastProcesses, p.selectedPID)
	if idx < 0 {
		idx = 0
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(p.lastProcesses) {
		idx = len(p.lastProcesses) - 1
	}
	p.selectedPID = p.lastProcesses[idx].PID
}

func (p *Process) moveSelectionToStart() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lastProcesses) == 0 {
		return
	}
	p.selectedPID = p.lastProcesses[0].PID
}

func (p *Process) moveSelectionToEnd() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lastProcesses) == 0 {
		return
	}
	p.selectedPID = p.lastProcesses[len(p.lastProcesses)-1].PID
}

func (p *Process) selectedProcess() (types.ProcessInfo, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lastProcesses) == 0 {
		return types.ProcessInfo{}, false
	}
	idx := indexByPID(p.lastProcesses, p.selectedPID)
	if idx < 0 {
		idx = 0
		p.selectedPID = p.lastProcesses[0].PID
	}
	return p.lastProcesses[idx], true
}

func (p *Process) toggleFollow() {
	proc, ok := p.selectedProcess()
	if !ok {
		p.setStatus("no process selected", true)
		return
	}
	p.mu.Lock()
	following := p.followPID == proc.PID
	if following {
		p.followPID = 0
	} else {
		p.followPID = proc.PID
		p.scrollOffset = 0
	}
	p.mu.Unlock()
	if following {
		p.setStatus("follow disabled", false)
	} else {
		p.setStatus(fmt.Sprintf("following pid %d", proc.PID), false)
	}
}

func (p *Process) toggleCollapse() {
	p.mu.Lock()
	if !p.cfg.TreeView {
		p.mu.Unlock()
		p.setStatus("tree view disabled", true)
		return
	}
	if len(p.lastProcesses) == 0 {
		p.mu.Unlock()
		p.setStatus("no process selected", true)
		return
	}
	idx := indexByPID(p.lastProcesses, p.selectedPID)
	if idx < 0 {
		idx = 0
		p.selectedPID = p.lastProcesses[0].PID
	}
	if !hasChildren(p.lastProcesses, idx) {
		p.mu.Unlock()
		p.setStatus("no children to collapse", true)
		return
	}
	pid := p.lastProcesses[idx].PID
	_, collapsed := p.treeCollapsed[pid]
	if collapsed {
		delete(p.treeCollapsed, pid)
	} else {
		if p.treeCollapsed == nil {
			p.treeCollapsed = make(map[int]struct{})
		}
		p.treeCollapsed[pid] = struct{}{}
	}
	p.mu.Unlock()
	if collapsed {
		p.setStatus(fmt.Sprintf("expanded pid %d", pid), false)
	} else {
		p.setStatus(fmt.Sprintf("collapsed pid %d", pid), false)
	}
}

func (p *Process) sendSelectedSignal(sig syscall.Signal) {
	proc, ok := p.selectedProcess()
	if !ok {
		p.setStatus("no process selected", true)
		return
	}
	if err := sendSignal(proc.PID, sig); err != nil {
		p.setStatus(fmt.Sprintf("signal %s failed: %v", sig.String(), err), true)
		return
	}
	p.setStatus(fmt.Sprintf("sent %s to %d", sig.String(), proc.PID), false)
}

func (p *Process) reniceSelected(delta int) {
	proc, ok := p.selectedProcess()
	if !ok {
		p.setStatus("no process selected", true)
		return
	}
	newNice, err := reniceProcess(proc.PID, proc.Nice, delta)
	if err != nil {
		p.setStatus(fmt.Sprintf("renice failed: %v", err), true)
		return
	}
	p.setStatus(fmt.Sprintf("renice pid %d to %d", proc.PID, newNice), false)
}
