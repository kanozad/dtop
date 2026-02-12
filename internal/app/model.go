package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mld.com/dtop/internal/config"
	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/internal/ui"
	"mld.com/dtop/pkg/collector"
)

type tickMsg struct{}
type pluginCollectMsg struct {
	id   plugin.ID
	data collector.Data
	err  error
}

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg     config.Config
	theme   theme.Theme
	plugins []plugin.Plugin
	data    map[plugin.ID]collector.Data

	width      int
	height     int
	pluginErrs map[plugin.ID]error

	hiddenBoxes map[plugin.ID]bool
	showHelp    bool
	startTime   time.Time
}

const minPluginHeight = 3

func NewModel(ctx context.Context, cancel context.CancelFunc, cfg config.Config, th theme.Theme, plugins []plugin.Plugin) Model {
	return Model{
		ctx:         ctx,
		cancel:      cancel,
		cfg:         cfg,
		theme:       th,
		plugins:     plugins,
		data:        map[plugin.ID]collector.Data{},
		pluginErrs:  map[plugin.ID]error{},
		hiddenBoxes: map[plugin.ID]bool{},
		startTime:   time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.scheduleTick()
}

func (m Model) scheduleTick() tea.Cmd {
	interval := m.cfg.UpdateInterval.Duration
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return tea.Tick(interval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.updatePlugins(msg)
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		switch msg.String() {
		case "1":
			m.toggleBox("cpu")
			return m, nil
		case "2":
			m.toggleBox("memory")
			return m, nil
		case "3":
			m.toggleBox("network")
			return m, nil
		case "4":
			m.toggleBox("process")
			return m, nil
		case "+", "=":
			if m.cfg.UpdateInterval.Duration > 500*time.Millisecond {
				m.cfg.UpdateInterval.Duration -= 500 * time.Millisecond
			}
			return m, nil
		case "-":
			m.cfg.UpdateInterval.Duration += 500 * time.Millisecond
			return m, nil
		case "?", "h":
			m.showHelp = !m.showHelp
			return m, nil
		}
		return m, m.updatePlugins(msg)
	case tickMsg:
		cmds := make([]tea.Cmd, 0, len(m.plugins)+1)
		for _, p := range m.plugins {
			p := p
			cmds = append(cmds, func() tea.Msg {
				out, err := p.Collect(m.ctx)
				return pluginCollectMsg{id: p.ID(), data: out, err: err}
			})
		}
		cmds = append(cmds, m.scheduleTick())
		return m, tea.Batch(cmds...)
	case pluginCollectMsg:
		if msg.err != nil {
			if m.pluginErrs == nil {
				m.pluginErrs = map[plugin.ID]error{}
			}
			m.pluginErrs[msg.id] = msg.err
			return m, nil
		}
		if m.pluginErrs != nil {
			delete(m.pluginErrs, msg.id)
		}
		m.data[msg.id] = msg.data
		return m, m.updatePlugin(msg.id, msg)
	default:
		return m, m.updatePlugins(msg)
	}
}

func (m Model) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	uptime := time.Since(m.startTime).Truncate(time.Second)
	headerLeft := m.theme.Header.Render("DTOP")
	headerRight := m.theme.Muted.Render(fmt.Sprintf("up %s  interval %s  [?]help [q]quit",
		uptime, m.cfg.UpdateInterval.Duration.Truncate(time.Millisecond)))
	headerLine := lipgloss.NewStyle().Width(w).Render(headerLeft + "  " + headerRight)

	if m.showHelp {
		return headerLine + "\n" + m.renderHelp(w)
	}

	bodyHeight := m.height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	pluginsView := m.renderPlugins(w, bodyHeight)

	lines := []string{headerLine, pluginsView}
	if len(m.pluginErrs) > 0 {
		seen := map[plugin.ID]struct{}{}
		for _, p := range m.plugins {
			if err := m.pluginErrs[p.ID()]; err != nil {
				seen[p.ID()] = struct{}{}
				errLine := m.theme.Error.Render(fmt.Sprintf("%s: %v", p.Name(), err))
				lines = append(lines, errLine)
			}
		}
		for id, err := range m.pluginErrs {
			if _, ok := seen[id]; ok || err == nil {
				continue
			}
			errLine := m.theme.Error.Render(fmt.Sprintf("%s: %v", id, err))
			lines = append(lines, errLine)
		}
	}

	return strings.Join(lines, "\n")
}

func (m *Model) toggleBox(id plugin.ID) {
	m.hiddenBoxes[id] = !m.hiddenBoxes[id]
}

func (m Model) visiblePlugins() []plugin.Plugin {
	out := make([]plugin.Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		if !m.hiddenBoxes[p.ID()] {
			out = append(out, p)
		}
	}
	return out
}

func (m Model) renderHelp(width int) string {
	help := []string{
		"Keyboard Shortcuts",
		"",
		"  q / Ctrl+C   Quit",
		"  ?  / h       Toggle this help",
		"  1            Toggle CPU box",
		"  2            Toggle Memory box",
		"  3            Toggle Network box",
		"  4            Toggle Process box",
		"  + / =        Decrease update interval",
		"  -            Increase update interval",
		"",
		"Process list:",
		"  Up/Down/j/k  Navigate",
		"  PgUp/PgDn    Page scroll",
		"  Home/End     Jump to top/bottom",
		"  f            Toggle follow mode",
		"  c            Collapse/expand tree node",
		"  x            Send SIGTERM to selected",
		"  X            Send SIGKILL to selected",
		"  r / R        Renice +1 / -1",
	}
	var sb strings.Builder
	for _, line := range help {
		sb.WriteString(m.theme.Text.Render(line))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) renderPlugins(width, height int) string {
	visible := m.visiblePlugins()
	if len(visible) == 0 {
		return m.theme.RenderBox("", "No plugins visible (press 1-4 to toggle)", width, height)
	}
	if height <= 0 {
		return ""
	}

	vChrome, _ := m.theme.BoxChrome()

	if len(visible) > 1 {
		switch m.cfg.Layout.Mode {
		case "flow":
			cols := ui.FlowColumns(len(visible), height, minPluginHeight+vChrome)
			if cols > 0 {
				return m.renderPluginsGrid(visible, width, height, cols, vChrome)
			}
		case "grid":
			return m.renderPluginsGrid(visible, width, height, m.cfg.Layout.Columns, vChrome)
		}
	}
	return m.renderPluginsVertical(visible, width, height, vChrome)
}

func (m Model) renderPluginsVertical(plugins []plugin.Plugin, width, height, vChrome int) string {
	// Subtract border overhead so the rendered boxes (content + chrome) fit.
	contentBudget := height - vChrome*len(plugins)
	if contentBudget < 0 {
		contentBudget = 0
	}
	heights := ui.SplitHeights(contentBudget, len(plugins), minPluginHeight)
	if len(heights) == 0 {
		return m.theme.Muted.Render("(terminal too small to display plugins)")
	}

	var views []string
	hidden := 0
	for i, p := range plugins {
		h := heights[i]
		if h < minPluginHeight {
			hidden++
			continue
		}
		views = append(views, p.View(m.data[p.ID()], width, h, m.theme))
	}
	if len(views) == 0 {
		return m.theme.Muted.Render("(terminal too small to display plugins)")
	}
	result := lipgloss.JoinVertical(lipgloss.Top, views...)
	if hidden > 0 {
		warn := m.theme.Muted.Render(fmt.Sprintf("(%d box(es) hidden — terminal too small)", hidden))
		result = lipgloss.JoinVertical(lipgloss.Top, result, warn)
	}
	return result
}

func (m Model) renderPluginsGrid(plugins []plugin.Plugin, width, height int, numCols, vChrome int) string {
	if numCols < 1 {
		numCols = 1
	}
	if numCols > len(plugins) {
		numCols = len(plugins)
	}

	// Distribute plugins across columns
	colSizes := ui.GridColumns(len(plugins), numCols)
	colWidths := ui.SplitWidths(width, numCols)

	// Render each column
	colViews := make([]string, numCols)
	hidden := 0
	idx := 0
	for col := 0; col < numCols; col++ {
		count := colSizes[col]
		colPlugins := plugins[idx : idx+count]
		idx += count

		// Subtract border overhead per box in this column.
		contentBudget := height - vChrome*count
		if contentBudget < 0 {
			contentBudget = 0
		}
		heights := ui.SplitHeights(contentBudget, count, minPluginHeight)
		if len(heights) == 0 {
			hidden += count
			continue
		}
		views := make([]string, 0, count)
		for i, p := range colPlugins {
			h := heights[i]
			if h < minPluginHeight {
				hidden++
				continue
			}
			views = append(views, p.View(m.data[p.ID()], colWidths[col], h, m.theme))
		}
		if len(views) > 0 {
			colViews[col] = lipgloss.JoinVertical(lipgloss.Top, views...)
		}
	}

	result := lipgloss.JoinHorizontal(lipgloss.Top, colViews...)
	if hidden > 0 {
		warn := m.theme.Muted.Render(fmt.Sprintf("(%d box(es) hidden — terminal too small)", hidden))
		result = lipgloss.JoinVertical(lipgloss.Top, result, warn)
	}
	return result
}

func (m Model) updatePlugins(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.plugins))
	for _, p := range m.plugins {
		cmds = append(cmds, p.Update(msg))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) updatePlugin(id plugin.ID, msg tea.Msg) tea.Cmd {
	for _, p := range m.plugins {
		if p.ID() == id {
			return p.Update(msg)
		}
	}
	return nil
}
