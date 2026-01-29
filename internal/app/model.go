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
}

func NewModel(ctx context.Context, cancel context.CancelFunc, cfg config.Config, th theme.Theme, plugins []plugin.Plugin) Model {
	return Model{
		ctx:        ctx,
		cancel:     cancel,
		cfg:        cfg,
		theme:      th,
		plugins:    plugins,
		data:       map[plugin.ID]collector.Data{},
		pluginErrs: map[plugin.ID]error{},
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
		if msg.Type == tea.KeyCtrlC {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
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
	header := m.theme.Header.Render("DTOP")
	subtitle := m.theme.Muted.Render("Phase 1 scaffold")

	w := m.width
	if w <= 0 {
		w = 80
	}

	headerLine := lipgloss.NewStyle().Width(w).Render(header + "  " + subtitle)

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

func (m Model) renderPlugins(width, height int) string {
	if len(m.plugins) == 0 {
		return m.theme.RenderBox("", "No plugins enabled", width, height)
	}
	if height <= 0 {
		return ""
	}
	heights := ui.SplitHeights(height, len(m.plugins), 3)
	views := make([]string, 0, len(m.plugins))
	for i, p := range m.plugins {
		h := heights[i]
		if h <= 0 {
			continue
		}
		views = append(views, p.View(m.data[p.ID()], width, h, m.theme))
	}
	if len(views) == 0 {
		return ""
	}

	return lipgloss.JoinVertical(lipgloss.Top, views...)
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
