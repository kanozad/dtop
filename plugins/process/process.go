package process

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kanozad/dtop/internal/plugin"
	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/internal/ui"
	"github.com/kanozad/dtop/pkg/collector"
	"github.com/kanozad/dtop/pkg/types"
)

type viewMode int

const (
	modeList viewMode = iota
	modeFilterEdit
	modeDetail
	modeSignalChooser
)

type signalChoice struct {
	name string
	sig  syscall.Signal
}

var processSignals = []signalChoice{
	{name: "SIGTERM", sig: syscall.SIGTERM},
	{name: "SIGKILL", sig: syscall.SIGKILL},
	{name: "SIGINT", sig: syscall.SIGINT},
	{name: "SIGHUP", sig: syscall.SIGHUP},
	{name: "SIGSTOP", sig: syscall.SIGSTOP},
	{name: "SIGCONT", sig: syscall.SIGCONT},
}

type Process struct {
	cfg Config
	mu  sync.Mutex

	// Previous state for CPU calculation
	prevProc map[int]procStat
	prevSys  systemStat

	// uidCache maps UID → username to avoid a lookup syscall per process per tick.
	uidCache map[int]string

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

	mode         viewMode
	filterInput  string
	filterCursor int
	signalIndex  int
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
		uidCache:      make(map[int]string),
		mode:          modeList,
	}
}

func (p *Process) ID() plugin.ID { return "process" }
func (p *Process) Name() string  { return "Processes" }
func (p *Process) SizeHint() ui.SizeHint {
	return ui.SizeHint{MinH: 5, PrefH: 20, MaxH: 0, Weight: 4}
}
func (p *Process) AllowedConfigKeys() []string {
	return []string{"tree_view", "sort_by", "filter", "max_display", "follow_pid", "use_smaps"}
}

func (p *Process) Init(_ context.Context, cfg map[string]any) error {
	p.cfg = parseConfig(p.cfg, cfg)
	p.followPID = p.cfg.FollowPID
	return nil
}

// Reconfigure applies updated config at runtime, mirroring Init. It re-derives
// followPID from the new config so a reload reflects the file's intent.
func (p *Process) Reconfigure(cfg map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = parseConfig(p.cfg, cfg)
	p.followPID = p.cfg.FollowPID
}

func (p *Process) Collect(context.Context) (collector.Data, error) {
	// Snapshot the inputs under the lock, then run the (potentially slow)
	// /proc scan unlocked so View/Update on the main loop don't block on it.
	// Collects are serialized per-plugin by the scheduler, so prevProc/prevSys/
	// uidCache have no other concurrent accessor.
	p.mu.Lock()
	cfg := p.cfg
	prevProc := p.prevProc
	prevSys := p.prevSys
	uidCache := p.uidCache
	p.mu.Unlock()

	stats, nextProc, nextSys, err := readProcessStats(prevProc, prevSys, cfg, uidCache)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.prevProc = nextProc
	p.prevSys = nextSys
	p.mu.Unlock()

	return stats, nil
}

func (p *Process) Shutdown(context.Context) error {
	return nil
}

func (p *Process) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			p.moveSelection(-3)
		case tea.MouseButtonWheelDown:
			p.moveSelection(3)
		}
		return nil
	case tea.KeyMsg:
		switch p.currentMode() {
		case modeFilterEdit:
			p.handleFilterEditKey(msg)
			return nil
		case modeDetail:
			p.handleDetailKey(msg)
			return nil
		case modeSignalChooser:
			p.handleSignalChooserKey(msg)
			return nil
		}
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
		case "f", "f3":
			p.startFilterEdit()
		case "F":
			p.toggleFollow()
		case "c":
			p.toggleCollapse()
		case "enter":
			p.openDetail()
		case "x":
			p.openSignalChooser(syscall.SIGTERM)
		case "X":
			p.openSignalChooser(syscall.SIGKILL)
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
	mode := p.mode
	filterInput := p.filterInput
	filterCursor := p.filterCursor
	signalIndex := p.signalIndex
	p.mu.Unlock()

	innerWidth := ui.ContentWidth(width)
	innerHeight := ui.ContentHeight(height)
	renderTree := cfg.TreeView && th.UTF8

	if statusMsg != "" && time.Since(statusAt) > statusTTL {
		p.clearStatus()
		statusMsg = ""
		statusErr = false
	}

	processes := stats.Processes
	if followPID > 0 {
		filtered := filterFollow(processes, followPID, renderTree)
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

	if renderTree {
		processes = applyTreeCollapse(processes, collapsed)
	}

	if cfg.MaxDisplay > 0 && len(processes) > cfg.MaxDisplay {
		processes = processes[:cfg.MaxDisplay]
	}

	selectedIndex := indexByPID(processes, selectedPID)
	if selectedIndex < 0 && len(processes) > 0 {
		selectedIndex = 0
		selectedPID = processes[0].PID
	}
	// Detail drill-in view.
	if mode == modeDetail {
		var body string
		if selectedIndex < 0 || len(processes) == 0 {
			body = th.Muted.Render("No process selected")
		} else {
			body = renderProcessDetail(processes[selectedIndex], innerWidth, th)
		}
		p.mu.Lock()
		p.selectedPID = selectedPID
		p.lastProcesses = append([]types.ProcessInfo(nil), processes...)
		p.treeCollapsed = pruneCollapsed(stats.Processes, collapsed)
		p.mu.Unlock()
		return th.RenderBox("Processes", body, width, height)
	}

	// Build header and mode prompts.
	header := buildHeader(stats, innerWidth, len(processes), followPID, cfg.UseSmaps)
	lines := []string{header}
	lines = append(lines, p.renderPrompts(mode, filterInput, filterCursor, signalIndex, innerWidth, th)...)

	if statusLine := p.renderStatusLine(statusMsg, statusErr, innerWidth, th); statusLine != "" {
		lines = append(lines, statusLine)
	}

	lines = append(lines, buildColumnHeader(stats.SortBy, innerWidth, th))
	hr := "-"
	if th.UTF8 {
		hr = "─"
	}
	lines = append(lines, strings.Repeat(hr, innerWidth))

	// Calculate available lines for processes.
	availableLines := innerHeight - len(lines)
	if availableLines < 0 {
		availableLines = 0
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

	lines = append(lines, p.renderProcesses(processes, scrollOffset, endIdx, selectedIndex, innerWidth, renderTree, th)...)

	// Add scroll indicator if needed
	if len(processes) > availableLines {
		scrollInfo := fmt.Sprintf(" (%d-%d of %d) ", scrollOffset+1, endIdx, len(processes))
		if len(lines) > 0 {
			lines[0] = ui.Truncate(lines[0]+th.Muted.Render(scrollInfo), innerWidth)
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

func (p *Process) renderPrompts(mode viewMode, filterInput string, filterCursor, signalIndex, width int, th theme.Theme) []string {
	var lines []string
	switch mode {
	case modeFilterEdit:
		lines = append(lines, th.Highlight.Render(renderFilterPrompt(filterInput, filterCursor, width, th)))
	case modeSignalChooser:
		lines = append(lines, th.Highlight.Render(renderSignalPrompt(signalIndex, width, th)))
	}
	return lines
}

func (p *Process) renderStatusLine(msg string, isErr bool, width int, th theme.Theme) string {
	if msg == "" {
		return ""
	}
	if isErr {
		return th.Error.Render(ui.Truncate("! "+msg, width))
	}
	return th.Muted.Render(ui.Truncate(msg, width))
}

func (p *Process) renderProcesses(processes []types.ProcessInfo, startIdx, endIdx, selectedIndex, width int, renderTree bool, th theme.Theme) []string {
	var lines []string
	for i := startIdx; i < endIdx; i++ {
		proc := processes[i]
		selected := i == selectedIndex
		line := formatProcess(proc, width, renderTree, false, th)
		if selected {
			lines = append(lines, th.Highlight.Render(ui.Truncate(line, width)))
		} else {
			lines = append(lines, th.Text.Render(line))
		}
	}
	return lines
}

// Process table column field widths, shared by buildColumnHeader and
// formatProcess so the header row and data rows always align. Each fixed column
// is followed by a single space separator in both renderers, so a column's
// total on-screen width is its field width + 1.
const (
	colWidthPID   = 6
	colWidthUser  = 10
	colWidthState = 2
	colWidthCPU   = 6 // "%5.1f%%" renders up to 6 cells
	colWidthMem   = 7
)

// buildColumnHeader renders the column header row with sort indicator.
func buildColumnHeader(sortBy types.ProcessSortField, width int, th theme.Theme) string {
	cols := []struct {
		name  string
		field types.ProcessSortField
		w     int
	}{
		{"PID", types.SortByPID, colWidthPID + 1},
		{"USER", types.SortByUser, colWidthUser + 1},
		{"S", types.ProcessSortField(-1), colWidthState + 1}, // state, not sortable by this name
		{"CPU%", types.SortByCPU, colWidthCPU + 1},
		{"MEM", types.SortByMemory, colWidthMem + 1},
		{"CMD", types.SortByName, 0}, // fills remaining
	}
	var parts []string
	for _, c := range cols {
		label := c.name
		if c.field == sortBy {
			if th.UTF8 {
				label += "▾"
			} else {
				label += "v"
			}
		}
		if c.w > 0 {
			parts = append(parts, fmt.Sprintf("%-*s", c.w, label))
		} else {
			parts = append(parts, label)
		}
	}
	line := "  " + strings.Join(parts, "")
	return th.Muted.Render(ui.Truncate(line, width))
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

	return ui.Truncate(header, width)
}

// formatProcess formats a single process for display.
func formatProcess(proc types.ProcessInfo, width int, treeView bool, selected bool, th theme.Theme) string {
	// Format: PID USER STATE CPU% MEM COMMAND
	// Adjust widths based on available space
	indicator := "  "
	if selected {
		indicator = "> "
	}
	// One space separator follows each of the 5 fixed columns.
	fixedWidth := len(indicator) + colWidthPID + colWidthUser + colWidthState + colWidthCPU + colWidthMem + 5

	cmdWidth := width - fixedWidth
	if treeView {
		prefix := proc.TreePrefix
		if !th.UTF8 {
			prefix = asciiTreePrefix(prefix)
		}
		cmdWidth -= len(prefix)
	}
	if treeView && proc.TreeCollapsed {
		if th.UTF8 {
			cmdWidth -= len("▸ ")
		} else {
			cmdWidth -= len("> ")
		}
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
		if !th.UTF8 {
			prefix = asciiTreePrefix(prefix)
		}
		if proc.TreeCollapsed {
			if th.UTF8 {
				prefix += "▸ "
			} else {
				prefix += "> "
			}
		}
		if prefix != "" {
			cmd = prefix + cmd
		}
	}

	line := fmt.Sprintf("%s%*d %-*s %*s %*.1f%% %*s %s",
		indicator,
		colWidthPID, proc.PID,
		colWidthUser, ui.Truncate(proc.User, colWidthUser),
		colWidthState, proc.State,
		colWidthCPU-1, proc.CPUPercent,
		colWidthMem, memStr,
		ui.Truncate(cmd, cmdWidth),
	)

	return ui.Truncate(line, width)
}

func asciiTreePrefix(s string) string {
	return strings.NewReplacer(
		"├", "|",
		"└", "`",
		"─", "-",
		"│", "|",
	).Replace(s)
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

const (
	statusTTL         = 5 * time.Second
	maxFilterInputLen = 120
)

func renderProcessDetail(proc types.ProcessInfo, width int, th theme.Theme) string {
	if width < 1 {
		width = 1
	}
	titleSep := " - "
	hr := "-"
	if th.UTF8 {
		titleSep = " — "
		hr = "─"
	}
	started := "n/a"
	if !proc.StartTime.IsZero() {
		started = proc.StartTime.Local().Format("2006-01-02 15:04:05")
	}
	lines := []string{
		th.Highlight.Render(ui.Truncate(fmt.Sprintf("PID %d%s%s", proc.PID, titleSep, proc.Command), width)),
		strings.Repeat(hr, width),
		th.Text.Render(ui.Truncate(fmt.Sprintf("User: %s   State: %s   Threads: %d   Nice: %d", proc.User, proc.State, proc.Threads, proc.Nice), width)),
		th.Text.Render(ui.Truncate(fmt.Sprintf("CPU: %.1f%%   Memory: %s", proc.CPUPercent, formatBytes(proc.MemBytes)), width)),
		th.Text.Render(ui.Truncate(fmt.Sprintf("PPID: %d   Started: %s", proc.PPID, started), width)),
	}
	if proc.FullCmd != "" {
		lines = append(lines, th.Text.Render(ui.Truncate("Cmd: "+proc.FullCmd, width)))
	} else {
		lines = append(lines, th.Text.Render(ui.Truncate("Cmd: "+proc.Command, width)))
	}
	lines = append(lines, th.Muted.Render(ui.Truncate("Esc/Enter: back to list", width)))
	return strings.Join(lines, "\n")
}

func renderFilterPrompt(input string, cursor int, width int, th theme.Theme) string {
	runes := []rune(input)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	cursorMark := "|"
	if th.UTF8 {
		cursorMark = "│"
	}
	withCursor := string(runes[:cursor]) + cursorMark + string(runes[cursor:])
	line := "Filter: " + withCursor + "  (Enter apply, Esc cancel)"
	return ui.Truncate(line, width)
}

func renderSignalPrompt(signalIndex int, width int, th theme.Theme) string {
	idx := clampSignalIndex(signalIndex)
	line := fmt.Sprintf("Signal: %s (up/down choose, Enter send, Esc cancel)", processSignals[idx].name)
	if th.UTF8 {
		line = fmt.Sprintf("Signal: %s (↑/↓ choose, Enter send, Esc cancel)", processSignals[idx].name)
	}
	return ui.Truncate(line, width)
}

func clampSignalIndex(idx int) int {
	if len(processSignals) == 0 {
		return 0
	}
	if idx < 0 {
		return len(processSignals) - 1
	}
	if idx >= len(processSignals) {
		return 0
	}
	return idx
}

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

func (p *Process) currentMode() viewMode {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mode
}

func (p *Process) startFilterEdit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = modeFilterEdit
	p.filterInput = p.cfg.FilterString
	p.filterCursor = len([]rune(p.filterInput))
}

func (p *Process) applyFilterEdit() {
	p.mu.Lock()
	filter := p.filterInput
	p.cfg.FilterString = filter
	p.mode = modeList
	p.scrollOffset = 0
	p.selectedPID = 0
	p.mu.Unlock()

	if filter == "" {
		p.setStatus("filter cleared", false)
	} else {
		p.setStatus(fmt.Sprintf("filter set: %s", filter), false)
	}
}

func (p *Process) cancelFilterEdit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = modeList
}

func (p *Process) handleFilterEditKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		p.cancelFilterEdit()
	case "enter":
		p.applyFilterEdit()
	case "left":
		p.moveFilterCursor(-1)
	case "right":
		p.moveFilterCursor(1)
	case "home", "ctrl+a":
		p.setFilterCursor(0)
	case "end", "ctrl+e":
		p.setFilterCursor(-1)
	case "backspace", "ctrl+h":
		p.deleteFilterBackward()
	case "delete":
		p.deleteFilterForward()
	default:
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			p.insertFilterRunes(msg.Runes)
		}
	}
}

func (p *Process) moveFilterCursor(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	runes := []rune(p.filterInput)
	next := p.filterCursor + delta
	if next < 0 {
		next = 0
	}
	if next > len(runes) {
		next = len(runes)
	}
	p.filterCursor = next
}

func (p *Process) setFilterCursor(pos int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	runes := []rune(p.filterInput)
	if pos < 0 || pos > len(runes) {
		pos = len(runes)
	}
	p.filterCursor = pos
}

func (p *Process) insertFilterRunes(r []rune) {
	p.mu.Lock()
	defer p.mu.Unlock()

	in := []rune(p.filterInput)
	if len(in) >= maxFilterInputLen {
		return
	}
	remaining := maxFilterInputLen - len(in)
	if len(r) > remaining {
		r = r[:remaining]
	}
	cursor := p.filterCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(in) {
		cursor = len(in)
	}
	out := append([]rune{}, in[:cursor]...)
	out = append(out, r...)
	out = append(out, in[cursor:]...)
	p.filterInput = string(out)
	p.filterCursor = cursor + len(r)
}

func (p *Process) deleteFilterBackward() {
	p.mu.Lock()
	defer p.mu.Unlock()

	in := []rune(p.filterInput)
	if len(in) == 0 || p.filterCursor <= 0 {
		return
	}
	cursor := p.filterCursor
	if cursor > len(in) {
		cursor = len(in)
	}
	out := append([]rune{}, in[:cursor-1]...)
	out = append(out, in[cursor:]...)
	p.filterInput = string(out)
	p.filterCursor = cursor - 1
}

func (p *Process) deleteFilterForward() {
	p.mu.Lock()
	defer p.mu.Unlock()

	in := []rune(p.filterInput)
	if len(in) == 0 {
		return
	}
	cursor := p.filterCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(in) {
		return
	}
	out := append([]rune{}, in[:cursor]...)
	out = append(out, in[cursor+1:]...)
	p.filterInput = string(out)
	p.filterCursor = cursor
}

func (p *Process) openDetail() {
	if _, ok := p.selectedProcess(); !ok {
		p.setStatus("no process selected", true)
		return
	}
	p.mu.Lock()
	p.mode = modeDetail
	p.mu.Unlock()
}

func (p *Process) handleDetailKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc", "enter", "backspace":
		p.mu.Lock()
		p.mode = modeList
		p.mu.Unlock()
	}
}

func (p *Process) openSignalChooser(defaultSig syscall.Signal) {
	if _, ok := p.selectedProcess(); !ok {
		p.setStatus("no process selected", true)
		return
	}
	idx := 0
	for i := range processSignals {
		if processSignals[i].sig == defaultSig {
			idx = i
			break
		}
	}
	p.mu.Lock()
	p.signalIndex = idx
	p.mode = modeSignalChooser
	p.mu.Unlock()
}

func (p *Process) handleSignalChooserKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		p.mu.Lock()
		p.mode = modeList
		p.mu.Unlock()
		p.setStatus("signal canceled", false)
	case "up", "k":
		p.mu.Lock()
		p.signalIndex = clampSignalIndex(p.signalIndex - 1)
		p.mu.Unlock()
	case "down", "j":
		p.mu.Lock()
		p.signalIndex = clampSignalIndex(p.signalIndex + 1)
		p.mu.Unlock()
	case "enter":
		p.applySelectedSignal()
	}
}

func (p *Process) applySelectedSignal() {
	p.mu.Lock()
	idx := clampSignalIndex(p.signalIndex)
	p.mode = modeList
	p.mu.Unlock()
	p.sendSelectedSignal(processSignals[idx].sig)
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
