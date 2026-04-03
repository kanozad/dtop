package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kanozad/dtop/internal/config"
	"github.com/kanozad/dtop/internal/plugin"
	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/internal/ui"
	"github.com/kanozad/dtop/pkg/collector"
	"github.com/kanozad/dtop/pkg/types"
)

// Reserved plugin.ID keys used to surface non-plugin errors in pluginErrs.
// The "~" prefix ensures they cannot collide with any real plugin ID.
const (
	errKeyConfig plugin.ID = "~config"
	errKeyPreset plugin.ID = "~preset"
	errKeyTheme  plugin.ID = "~theme"
)

type tickMsg struct{}
type pluginCollectMsg struct {
	id   plugin.ID
	data collector.Data
	err  error
}

func (m *Model) refreshPluginSchedules(now time.Time) {
	if m.pluginIntervals == nil {
		m.pluginIntervals = map[plugin.ID]time.Duration{}
	}
	if m.pluginNextDue == nil {
		m.pluginNextDue = map[plugin.ID]time.Time{}
	}
	if m.pluginInFlight == nil {
		m.pluginInFlight = map[plugin.ID]bool{}
	}

	seen := make(map[plugin.ID]struct{}, len(m.plugins))
	for _, p := range m.plugins {
		id := p.ID()
		seen[id] = struct{}{}
		interval := m.intervalForPlugin(id)
		m.pluginIntervals[id] = interval
		if _, ok := m.pluginNextDue[id]; !ok || m.pluginNextDue[id].IsZero() {
			m.pluginNextDue[id] = now.Add(interval)
		}
	}

	for id := range m.pluginIntervals {
		if _, ok := seen[id]; !ok {
			delete(m.pluginIntervals, id)
			delete(m.pluginNextDue, id)
			delete(m.pluginInFlight, id)
		}
	}
}

func (m *Model) schedulerTickInterval() time.Duration {
	minInterval := m.defaultInterval()
	for _, interval := range m.pluginIntervals {
		if interval > 0 && interval < minInterval {
			minInterval = interval
		}
	}
	return minInterval
}

func (m *Model) defaultInterval() time.Duration {
	interval := m.cfg.UpdateInterval.Duration
	if interval <= 0 {
		return defaultUpdateInterval
	}
	return interval
}

func (m *Model) intervalForPlugin(id plugin.ID) time.Duration {
	interval := m.defaultInterval()
	if m.cfg.Plugins.Config == nil {
		return interval
	}
	cfgByID, ok := m.cfg.Plugins.Config[string(id)]
	if !ok || cfgByID == nil {
		return interval
	}
	raw, ok := cfgByID[plugin.GlobalPluginIntervalKey]
	if !ok {
		return interval
	}
	parsed, ok := parseIntervalValue(raw)
	if !ok || parsed <= 0 {
		return interval
	}
	return parsed
}

func parseIntervalValue(raw any) (time.Duration, bool) {
	switch v := raw.(type) {
	case string:
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, false
		}
		return d, true
	case config.Duration:
		return v.Duration, true
	case time.Duration:
		return v, true
	default:
		return 0, false
	}
}

func nextDue(after, now time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		interval = defaultUpdateInterval
	}
	if after.IsZero() {
		after = now
	}
	for !after.After(now) {
		after = after.Add(interval)
	}
	return after
}

func (m *Model) collectDuePlugins(now time.Time) []tea.Cmd {
	m.refreshPluginSchedules(now)
	cmds := make([]tea.Cmd, 0, len(m.plugins))
	for _, p := range m.plugins {
		id := p.ID()
		interval := m.pluginIntervals[id]
		due := m.pluginNextDue[id]
		if now.Before(due) {
			continue
		}
		if m.pluginInFlight[id] {
			// Explicit non-overlapping semantics: skip this slot while a collect
			// is already running and advance to the next scheduled slot.
			m.pluginNextDue[id] = nextDue(due, now, interval)
			continue
		}

		m.pluginInFlight[id] = true
		m.pluginNextDue[id] = nextDue(due, now, interval)

		plug := p
		cmds = append(cmds, func() tea.Msg {
			out, err := plug.Collect(m.ctx)
			return pluginCollectMsg{id: plug.ID(), data: out, err: err}
		})
	}
	return cmds
}

func fileModTime(path string) time.Time {
	if strings.TrimSpace(path) == "" {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (m *Model) maybeReloadConfig() {
	if !m.cfg.LiveReload || strings.TrimSpace(m.configPath) == "" {
		return
	}
	modTime := fileModTime(m.configPath)
	if modTime.IsZero() {
		return
	}
	if !m.configMTime.IsZero() && !modTime.After(m.configMTime) {
		return
	}

	nextCfg, err := config.Load(m.configPath)
	if err != nil {
		m.pluginErrs[errKeyConfig] = fmt.Errorf("reload config: %w", err)
		m.configMTime = modTime
		return
	}

	m.applyReloadedConfig(nextCfg)
	m.configMTime = modTime
	delete(m.pluginErrs, errKeyConfig)
}

func (m *Model) applyReloadedConfig(nextCfg config.Config) {
	m.cfg.UpdateInterval = nextCfg.UpdateInterval
	m.cfg.Layout = nextCfg.Layout
	m.cfg.LiveReload = nextCfg.LiveReload
	m.cfg.Presets = nextCfg.Presets
	m.cfg.Plugins.Config = nextCfg.Plugins.Config
	m.refreshPluginSchedules(m.now())

	utf8 := m.theme.UTF8
	if nextCfg.Theme.Name == "" || nextCfg.Theme.Name == m.cfg.Theme.Name {
		return
	}
	nextTheme, err := theme.FromName(nextCfg.Theme.Name)
	if err != nil {
		m.pluginErrs[errKeyConfig] = fmt.Errorf("reload theme %q: %w", nextCfg.Theme.Name, err)
		return
	}
	nextTheme.UTF8 = utf8
	nextTheme.ColorLevel = m.theme.ColorLevel
	m.theme = nextTheme
	m.cfg.Theme = nextCfg.Theme
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

	hiddenBoxes      map[plugin.ID]bool
	showHelp         bool
	startTime        time.Time
	now              func() time.Time
	configPath       string
	configMTime      time.Time
	presetMode       bool
	presetSaveMode   bool
	presetDeleteMode bool
	presetExportMode bool
	presetImportMode bool

	showThemePicker bool
	themeMenu       ui.MenuState

	showOptions bool
	optionsMenu ui.MenuState

	pluginIntervals map[plugin.ID]time.Duration
	pluginNextDue   map[plugin.ID]time.Time
	pluginInFlight  map[plugin.ID]bool

	history *types.HistoryStore
}

const (
	defaultUpdateInterval = 2 * time.Second
)

func NewModel(ctx context.Context, cancel context.CancelFunc, cfg config.Config, th theme.Theme, plugins []plugin.Plugin, configPath string) Model {
	m := Model{
		ctx:         ctx,
		cancel:      cancel,
		cfg:         cfg,
		theme:       th,
		plugins:     plugins,
		data:        map[plugin.ID]collector.Data{},
		pluginErrs:  map[plugin.ID]error{},
		hiddenBoxes: map[plugin.ID]bool{},
		startTime:   time.Now(),
		now:         time.Now,
		configPath:  configPath,
		configMTime: fileModTime(configPath),

		pluginIntervals: map[plugin.ID]time.Duration{},
		pluginNextDue:   map[plugin.ID]time.Time{},
		pluginInFlight:  map[plugin.ID]bool{},

		history: types.NewHistoryStore(),
	}
	m.refreshPluginSchedules(m.now())
	return m
}

func (m Model) Init() tea.Cmd {
	return m.scheduleTick()
}

func (m Model) scheduleTick() tea.Cmd {
	now := m.now()
	m.refreshPluginSchedules(now)
	interval := m.schedulerTickInterval()
	return tea.Tick(interval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.history != nil {
			m.history.Resize(ui.ContentWidth(m.width))
		}
		return m, m.updatePlugins(msg)
	case tea.MouseMsg:
		// Delegate mouse events to plugins (e.g. scroll in process list).
		return m, m.updatePlugins(msg)
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.showThemePicker {
			switch msg.String() {
			case "up", "k":
				m.themeMenu.MoveSelection(-1, 10)
			case "down", "j":
				m.themeMenu.MoveSelection(1, 10)
			case "enter":
				m.applySelectedTheme()
				m.showThemePicker = false
			case "esc":
				m.showThemePicker = false
			}
			return m, nil
		}
		if m.showOptions {
			switch msg.String() {
			case "up", "k":
				m.optionsMenu.MoveSelection(-1, 10)
			case "down", "j":
				m.optionsMenu.MoveSelection(1, 10)
			case "enter":
				m.applySelectedOption()
			case "esc":
				m.showOptions = false
			}
			return m, nil
		}
		if m.presetMode {
			switch msg.String() {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.presetMode = false
				m.applyPreset(msg.String())
				return m, nil
			case "esc":
				m.presetMode = false
				return m, nil
			default:
				m.presetMode = false
			}
		}
		if m.presetSaveMode {
			switch msg.String() {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.presetSaveMode = false
				m.savePreset(msg.String())
				return m, nil
			case "esc":
				m.presetSaveMode = false
				return m, nil
			default:
				m.presetSaveMode = false
			}
		}
		if m.presetDeleteMode {
			switch msg.String() {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.presetDeleteMode = false
				m.deletePreset(msg.String())
				return m, nil
			case "esc":
				m.presetDeleteMode = false
				return m, nil
			default:
				m.presetDeleteMode = false
			}
		}
		if m.presetExportMode {
			switch msg.String() {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.presetExportMode = false
				m.exportPreset(msg.String())
				return m, nil
			case "esc":
				m.presetExportMode = false
				return m, nil
			default:
				m.presetExportMode = false
			}
		}
		if m.presetImportMode {
			switch msg.String() {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				m.presetImportMode = false
				m.importPreset(msg.String())
				return m, nil
			case "esc":
				m.presetImportMode = false
				return m, nil
			default:
				m.presetImportMode = false
			}
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
		case "p":
			m.presetSaveMode = false
			m.presetDeleteMode = false
			m.presetExportMode = false
			m.presetImportMode = false
			m.presetMode = true
			return m, nil
		case "P":
			m.presetMode = false
			m.presetDeleteMode = false
			m.presetExportMode = false
			m.presetImportMode = false
			m.presetSaveMode = true
			return m, nil
		case "D":
			m.presetMode = false
			m.presetSaveMode = false
			m.presetExportMode = false
			m.presetImportMode = false
			m.presetDeleteMode = true
			return m, nil
		case "E":
			m.presetMode = false
			m.presetSaveMode = false
			m.presetDeleteMode = false
			m.presetImportMode = false
			m.presetExportMode = true
			return m, nil
		case "I":
			m.presetMode = false
			m.presetSaveMode = false
			m.presetDeleteMode = false
			m.presetExportMode = false
			m.presetImportMode = true
			return m, nil
		case "t":
			m.openThemePicker()
			return m, nil
		case "o":
			m.openOptions()
			return m, nil
		}
		return m, m.updatePlugins(msg)
	case tickMsg:
		m.maybeReloadConfig()
		cmds := m.collectDuePlugins(m.now())
		cmds = append(cmds, m.scheduleTick())
		return m, tea.Batch(cmds...)
	case pluginCollectMsg:
		if m.pluginInFlight == nil {
			m.pluginInFlight = map[plugin.ID]bool{}
		}
		m.pluginInFlight[msg.id] = false
		if msg.err != nil {
			m.pluginErrs[msg.id] = msg.err
			return m, nil
		}
		delete(m.pluginErrs, msg.id)
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
	headerText := fmt.Sprintf("up %s  interval %s", uptime, m.cfg.UpdateInterval.Truncate(time.Millisecond))
	if m.cfg.LiveReload {
		headerText += "  reload:on"
	}
	if m.presetMode {
		headerText += "  preset:0-9/esc"
	}
	if m.presetSaveMode {
		headerText += "  save:0-9/esc"
	}
	if m.presetDeleteMode {
		headerText += "  del:0-9/esc"
	}
	if m.presetExportMode {
		headerText += "  export:0-9/esc"
	}
	if m.presetImportMode {
		headerText += "  import:0-9/esc"
	}
	headerText += "  [?]help [q]quit"
	headerRight := m.theme.Muted.Render(headerText)
	headerLine := lipgloss.NewStyle().Width(w).Render(headerLeft + "  " + headerRight)

	if m.showHelp {
		return headerLine + "\n" + m.renderHelp(w)
	}

	if m.showThemePicker {
		menuBody := ui.RenderMenu(m.themeMenu, ui.MenuOpts{
			Width:       36,
			MaxVisible:  10,
			NormalStyle: m.theme.Text,
			ActiveStyle: m.theme.Highlight,
		})
		overlay := ui.RenderDialog(ui.DialogOpts{
			Title:      "Theme",
			Body:       menuBody,
			Width:      40,
			TitleStyle: m.theme.BoxTitle,
			BodyStyle:  lipgloss.NewStyle(),
			BorderFg:   lipgloss.Color("8"),
		})
		return headerLine + "\n" + ui.PlaceOverlay(overlay, w, m.height-2)
	}

	if m.showOptions {
		menuBody := ui.RenderMenu(m.optionsMenu, ui.MenuOpts{
			Width:       46,
			MaxVisible:  10,
			NormalStyle: m.theme.Text,
			ActiveStyle: m.theme.Highlight,
		})
		overlay := ui.RenderDialog(ui.DialogOpts{
			Title:      "Options",
			Body:       menuBody,
			Width:      50,
			TitleStyle: m.theme.BoxTitle,
			BodyStyle:  lipgloss.NewStyle(),
			BorderFg:   lipgloss.Color("8"),
		})
		return headerLine + "\n" + ui.PlaceOverlay(overlay, w, m.height-2)
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

func (m *Model) applyPreset(slot string) {
	preset, ok := m.cfg.Presets[slot]
	if !ok {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("preset %s not configured", slot)
		return
	}

	if preset.LayoutMode != "" {
		m.cfg.Layout.Mode = preset.LayoutMode
	}
	if preset.LayoutColumns > 0 {
		m.cfg.Layout.Columns = preset.LayoutColumns
	}
	if preset.UpdateInterval.Duration > 0 {
		m.cfg.UpdateInterval = preset.UpdateInterval
	}
	if len(preset.VisibleBoxes) > 0 {
		visible := make(map[plugin.ID]struct{}, len(preset.VisibleBoxes))
		for _, id := range preset.VisibleBoxes {
			visible[plugin.ID(id)] = struct{}{}
		}
		for _, p := range m.plugins {
			_, show := visible[p.ID()]
			m.hiddenBoxes[p.ID()] = !show
		}
	}
	delete(m.pluginErrs, errKeyPreset)
}

func (m *Model) importPreset(slot string) {
	if strings.TrimSpace(m.configPath) == "" {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("cannot import preset %s: config path is not set", slot)
		return
	}
	preset, err := config.ImportPreset(m.configPath, slot)
	if err != nil {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("import preset %s: %w", slot, err)
		return
	}
	if m.cfg.Presets == nil {
		m.cfg.Presets = map[string]config.PresetConfig{}
	}
	m.cfg.Presets[slot] = preset
	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("persist imported preset %s: %w", slot, err)
		return
	}
	m.applyPreset(slot)
	m.configMTime = fileModTime(m.configPath)
	delete(m.pluginErrs, errKeyPreset)
}

func (m *Model) savePreset(slot string) {
	if m.cfg.Presets == nil {
		m.cfg.Presets = map[string]config.PresetConfig{}
	}
	visibleBoxes := make([]string, 0, len(m.plugins))
	for _, p := range m.plugins {
		if !m.hiddenBoxes[p.ID()] {
			visibleBoxes = append(visibleBoxes, string(p.ID()))
		}
	}
	m.cfg.Presets[slot] = config.PresetConfig{
		LayoutMode:     m.cfg.Layout.Mode,
		LayoutColumns:  m.cfg.Layout.Columns,
		UpdateInterval: m.cfg.UpdateInterval,
		VisibleBoxes:   visibleBoxes,
	}
	if strings.TrimSpace(m.configPath) == "" {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("cannot save preset %s: config path is not set", slot)
		return
	}
	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("save preset %s: %w", slot, err)
		return
	}
	m.configMTime = fileModTime(m.configPath)
	delete(m.pluginErrs, errKeyPreset)
}

func (m *Model) deletePreset(slot string) {
	if _, ok := m.cfg.Presets[slot]; !ok {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("preset %s not configured", slot)
		return
	}
	delete(m.cfg.Presets, slot)
	if strings.TrimSpace(m.configPath) == "" {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("cannot delete preset %s: config path is not set", slot)
		return
	}
	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("delete preset %s: %w", slot, err)
		return
	}
	m.configMTime = fileModTime(m.configPath)
	delete(m.pluginErrs, errKeyPreset)
}

func (m *Model) exportPreset(slot string) {
	preset, ok := m.cfg.Presets[slot]
	if !ok {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("preset %s not configured", slot)
		return
	}
	if _, err := config.ExportPreset(m.configPath, slot, preset); err != nil {
		m.pluginErrs[errKeyPreset] = fmt.Errorf("export preset %s: %w", slot, err)
		return
	}
	delete(m.pluginErrs, errKeyPreset)
}

func (m *Model) openOptions() {
	items := []ui.MenuItem{
		{Label: fmt.Sprintf("Interval: %s", m.cfg.UpdateInterval.Duration), Value: "interval"},
		{Label: fmt.Sprintf("Layout:   %s", m.cfg.Layout.Mode), Value: "layout"},
		{Label: fmt.Sprintf("Columns:  %d", m.cfg.Layout.Columns), Value: "columns"},
		{Label: fmt.Sprintf("Reload:   %v", m.cfg.LiveReload), Value: "reload"},
		{Label: "Save Config", Value: "save"},
		{Label: "Close", Value: "close"},
	}
	m.optionsMenu = ui.NewMenuState(items)
	m.showOptions = true
}

func (m *Model) applySelectedOption() {
	item := m.optionsMenu.Items[m.optionsMenu.Selected]
	switch item.Value {
	case "interval":
		// Cycle: 0.5s -> 1s -> 2s -> 5s -> 10s -> 0.5s
		d := m.cfg.UpdateInterval.Duration
		switch d {
		case 500 * time.Millisecond:
			d = 1 * time.Second
		case 1 * time.Second:
			d = 2 * time.Second
		case 2 * time.Second:
			d = 5 * time.Second
		case 5 * time.Second:
			d = 10 * time.Second
		default:
			d = 500 * time.Millisecond
		}
		m.cfg.UpdateInterval.Duration = d
	case "layout":
		// Cycle: vertical -> grid -> flow -> vertical
		switch m.cfg.Layout.Mode {
		case "vertical":
			m.cfg.Layout.Mode = "grid"
		case "grid":
			m.cfg.Layout.Mode = "flow"
		default:
			m.cfg.Layout.Mode = "vertical"
		}
	case "columns":
		m.cfg.Layout.Columns++
		if m.cfg.Layout.Columns > 4 {
			m.cfg.Layout.Columns = 1
		}
	case "reload":
		m.cfg.LiveReload = !m.cfg.LiveReload
	case "save":
		if m.configPath != "" {
			_ = config.Save(m.configPath, m.cfg)
		}
		m.showOptions = false
		return
	case "close":
		m.showOptions = false
		return
	}
	// Refresh menu labels
	m.openOptions()
}

func (m *Model) openThemePicker() {
	items := []ui.MenuItem{{Label: "default", Value: "default"}}
	dir, err := os.UserConfigDir()
	if err == nil {
		entries, _ := os.ReadDir(filepath.Join(dir, "dtop", "themes"))
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
				name := strings.TrimSuffix(e.Name(), ".toml")
				items = append(items, ui.MenuItem{Label: name, Value: name})
			}
		}
	}
	m.themeMenu = ui.NewMenuState(items)
	// Pre-select current theme.
	for i, item := range items {
		if item.Value == m.cfg.Theme.Name {
			m.themeMenu.Selected = i
			break
		}
	}
	m.showThemePicker = true
}

func (m *Model) applySelectedTheme() {
	selected := m.themeMenu.SelectedItem()
	if selected.Value == "" || selected.Value == m.cfg.Theme.Name {
		return
	}
	nextTheme, err := theme.FromName(selected.Value)
	if err != nil {
		m.pluginErrs[errKeyTheme] = fmt.Errorf("load theme %q: %w", selected.Value, err)
		return
	}
	nextTheme.UTF8 = m.theme.UTF8
	nextTheme.ColorLevel = m.theme.ColorLevel
	m.theme = nextTheme
	m.cfg.Theme.Name = selected.Value
	delete(m.pluginErrs, errKeyTheme)
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
		"  p then 0-9   Load preset slot",
		"  P then 0-9   Save current view to preset slot",
		"  D then 0-9   Delete preset slot",
		"  E then 0-9   Export preset slot to <config-dir>/presets/",
		"  I then 0-9   Import preset slot from <config-dir>/presets/",
		"  t            Open theme picker",
		"  o            Open options editor",
		"  + / =        Decrease update interval",
		"  -            Increase update interval",
		"",
		"Process list:",
		"  Up/Down/j/k  Navigate",
		"  PgUp/PgDn    Page scroll",
		"  Mouse wheel  Scroll ±3 rows",
		"  Home/End     Jump to top/bottom",
		"  f / F3       Edit process filter",
		"  F            Toggle follow mode",
		"  Enter        Process detail view",
		"  c            Collapse/expand tree node",
		"  x / X        Open signal chooser",
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

	var result string
	if len(visible) > 1 {
		switch m.cfg.Layout.Mode {
		case "flow":
			hints := collectHints(visible)
			minH := hints[0].MinH
			for _, h := range hints {
				if h.MinH < minH {
					minH = h.MinH
				}
			}
			cols := ui.FlowColumns(len(visible), height, minH+vChrome)
			if cols > 0 {
				result = m.renderPluginsGrid(visible, width, height, cols, vChrome)
			}
		case "grid":
			result = m.renderPluginsGrid(visible, width, height, m.cfg.Layout.Columns, vChrome)
		}
	}
	if result == "" {
		result = m.renderPluginsVertical(visible, width, height, vChrome)
	}

	// Global safety belt: ensure the final string never exceeds terminal height.
	lines := strings.Split(result, "\n")
	if len(lines) > height {
		result = strings.Join(lines[:height], "\n")
	}
	return result
}

// pluginColumnPref returns the 0-based column index a plugin requests via the
// "column" config key (1-based in TOML). Returns -1 if no preference is set.
func (m Model) pluginColumnPref(id plugin.ID) int {
	if m.cfg.Plugins.Config == nil {
		return -1
	}
	cfgByID, ok := m.cfg.Plugins.Config[string(id)]
	if !ok || cfgByID == nil {
		return -1
	}
	raw, ok := cfgByID[plugin.GlobalPluginColumnKey]
	if !ok {
		return -1
	}
	var col int
	switch v := raw.(type) {
	case int64:
		col = int(v)
	case int:
		col = v
	case float64:
		col = int(v)
	default:
		return -1
	}
	return col - 1 // convert 1-based (config) to 0-based (internal)
}

// groupPluginsByColumn distributes plugins into numCols columns. Plugins with
// a column preference are placed first; remaining plugins are distributed
// evenly in declaration order, matching the existing left-heavy GridColumns
// allocation so that the no-pinning case is identical to the prior behaviour.
func (m Model) groupPluginsByColumn(plugins []plugin.Plugin, numCols int) [][]plugin.Plugin {
	columns := make([][]plugin.Plugin, numCols)
	var unpinned []plugin.Plugin
	for _, p := range plugins {
		col := m.pluginColumnPref(p.ID())
		if col >= 0 && col < numCols {
			columns[col] = append(columns[col], p)
		} else {
			unpinned = append(unpinned, p)
		}
	}
	if len(unpinned) == 0 {
		return columns
	}
	sizes := ui.GridColumns(len(unpinned), numCols)
	idx := 0
	for c, size := range sizes {
		columns[c] = append(columns[c], unpinned[idx:idx+size]...)
		idx += size
	}
	return columns
}

// activeColumnCount returns the effective number of columns for the current
// layout mode and terminal dimensions. Used both for rendering and for sizing
// history buffers correctly in multi-column layouts.
func (m Model) activeColumnCount(visible []plugin.Plugin) int {
	if len(visible) <= 1 {
		return 1
	}
	switch m.cfg.Layout.Mode {
	case "grid":
		cols := m.cfg.Layout.Columns
		if cols < 1 {
			cols = 1
		}
		if cols > len(visible) {
			cols = len(visible)
		}
		return cols
	case "flow":
		vChrome, _ := m.theme.BoxChrome()
		hints := collectHints(visible)
		minH := hints[0].MinH
		for _, h := range hints {
			if h.MinH < minH {
				minH = h.MinH
			}
		}
		if cols := ui.FlowColumns(len(visible), m.height, minH+vChrome); cols > 0 {
			return cols
		}
	}
	return 1
}

// pluginContentWidth returns the correct content width for a plugin's history
// buffer, accounting for multi-column layouts where each column is narrower
// than the full terminal width.
func (m Model) pluginContentWidth(id plugin.ID) int {
	visible := m.visiblePlugins()
	numCols := m.activeColumnCount(visible)
	if numCols <= 1 {
		return ui.ContentWidth(m.width)
	}
	colWidths := ui.SplitWidths(m.width, numCols)
	for col, colPlugins := range m.groupPluginsByColumn(visible, numCols) {
		for _, p := range colPlugins {
			if p.ID() == id {
				return ui.ContentWidth(colWidths[col])
			}
		}
	}
	return ui.ContentWidth(m.width)
}

func sizeHintForPlugin(p plugin.Plugin) ui.SizeHint {
	if sh, ok := p.(plugin.SizeHinter); ok {
		return sh.SizeHint()
	}
	return ui.DefaultSizeHint()
}

func collectHints(plugins []plugin.Plugin) []ui.SizeHint {
	hints := make([]ui.SizeHint, len(plugins))
	for i, p := range plugins {
		hints[i] = sizeHintForPlugin(p)
	}
	return hints
}

func (m Model) renderPluginsVertical(plugins []plugin.Plugin, width, height, vChrome int) string {
	// Subtract border overhead so the rendered boxes (content + chrome) fit.
	contentBudget := height - vChrome*len(plugins)
	if contentBudget < 0 {
		contentBudget = 0
	}
	heights := ui.AllocateHeights(collectHints(plugins), contentBudget)
	if len(heights) == 0 {
		return m.theme.Muted.Render("(terminal too small to display plugins)")
	}

	var views []string
	hidden := 0
	for i, p := range plugins {
		h := heights[i]
		minH := sizeHintForPlugin(p).MinH
		if h < minH {
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
		warn := m.theme.Muted.Render(fmt.Sprintf("(%d box(es) hidden - terminal too small)", hidden))
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

	colWidths := ui.SplitWidths(width, numCols)
	columns := m.groupPluginsByColumn(plugins, numCols)

	// Render each column
	colViews := make([]string, numCols)
	hidden := 0
	for col := 0; col < numCols; col++ {
		colPlugins := columns[col]
		count := len(colPlugins)

		// Subtract border overhead per box in this column.
		contentBudget := height - vChrome*count
		if contentBudget < 0 {
			contentBudget = 0
		}
		heights := ui.AllocateHeights(collectHints(colPlugins), contentBudget)
		if len(heights) == 0 {
			hidden += count
			continue
		}
		views := make([]string, 0, count)
		for i, p := range colPlugins {
			h := heights[i]
			minH := sizeHintForPlugin(p).MinH
			if h < minH {
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
		warn := m.theme.Muted.Render(fmt.Sprintf("(%d box(es) hidden - terminal too small)", hidden))
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
			if collectMsg, ok := msg.(pluginCollectMsg); ok {
				if ha, ok := p.(plugin.HistoryAware); ok {
					m.data[id] = ha.UpdateHistory(m.history, collectMsg.data, m.pluginContentWidth(id))
				}
			}
			return p.Update(msg)
		}
	}
	return nil
}
