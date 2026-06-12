package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kanozad/dtop/internal/config"
	"github.com/kanozad/dtop/internal/plugin"
	"github.com/kanozad/dtop/internal/theme"
	"github.com/kanozad/dtop/internal/ui"
	"github.com/kanozad/dtop/pkg/collector"
)

type modelPlugin struct {
	id   plugin.ID
	name string
}

func (p *modelPlugin) ID() plugin.ID { return p.id }
func (p *modelPlugin) Name() string  { return p.name }
func (p *modelPlugin) Init(context.Context, map[string]any) error {
	return nil
}
func (p *modelPlugin) Collect(context.Context) (collector.Data, error) {
	return nil, nil
}
func (p *modelPlugin) Shutdown(context.Context) error { return nil }
func (p *modelPlugin) Update(tea.Msg) tea.Cmd         { return nil }
func (p *modelPlugin) View(collector.Data, int, int, theme.Theme) string {
	return "ok"
}

type capturingPlugin struct {
	modelPlugin
	capturing bool
	keys      []string
}

func (p *capturingPlugin) CapturingInput() bool { return p.capturing }
func (p *capturingPlugin) Update(msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		p.keys = append(p.keys, k.String())
	}
	return nil
}

type collectCountingPlugin struct {
	id           plugin.ID
	name         string
	collectCount int
}

func (p *collectCountingPlugin) ID() plugin.ID { return p.id }
func (p *collectCountingPlugin) Name() string  { return p.name }
func (p *collectCountingPlugin) Init(context.Context, map[string]any) error {
	return nil
}
func (p *collectCountingPlugin) Collect(context.Context) (collector.Data, error) {
	p.collectCount++
	return p.collectCount, nil
}
func (p *collectCountingPlugin) Shutdown(context.Context) error { return nil }
func (p *collectCountingPlugin) Update(tea.Msg) tea.Cmd         { return nil }
func (p *collectCountingPlugin) View(collector.Data, int, int, theme.Theme) string {
	return "ok"
}

func TestModelRenderPluginsVerticalChromeAccounting(t *testing.T) {
	t.Parallel()

	// 3 plugins, 24 rows. Each box has 2 lines of chrome.
	// Content budget = 24 - 2*3 = 18, split 3 ways = 6 each.
	// Total rendered per box = 6 (content) + 2 (chrome) = 8; 8*3 = 24. Fits exactly.
	plugins := []plugin.Plugin{
		&modelPlugin{id: "a", name: "A"},
		&modelPlugin{id: "b", name: "B"},
		&modelPlugin{id: "c", name: "C"},
	}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), plugins, "")
	m.width = 80
	m.height = 26 // 24 body + 2 header

	view := m.View()
	if strings.Contains(view, "hidden") {
		t.Fatalf("expected no hidden boxes, got:\n%s", view)
	}
}

func TestModelRenderPluginsHiddenWarning(t *testing.T) {
	t.Parallel()

	// 5 plugins in vertical mode, only 12 body rows.
	// Content budget = 12 - 2*5 = 2, minPluginHeight=3 per plugin.
	// 2 total content rows can't give 3 to any of the 5 plugins.
	cfg := config.Default()
	cfg.Layout.Mode = "vertical"
	plugins := []plugin.Plugin{
		&modelPlugin{id: "a", name: "A"},
		&modelPlugin{id: "b", name: "B"},
		&modelPlugin{id: "c", name: "C"},
		&modelPlugin{id: "d", name: "D"},
		&modelPlugin{id: "e", name: "E"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins, "")
	m.width = 80
	m.height = 14 // 12 body + 2 header

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Fatalf("expected hidden-box warning, got:\n%s", view)
	}
}

func TestModelViewNoPlugins(t *testing.T) {
	t.Parallel()

	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), nil, "")
	m.width = 80
	m.height = 10

	view := m.View()
	if !strings.Contains(view, "No plugins visible") {
		t.Fatalf("expected empty plugin placeholder, got:\n%s", view)
	}
}

func TestModelViewPluginErrors(t *testing.T) {
	t.Parallel()

	p := &modelPlugin{id: "clock", name: "Clock"}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), []plugin.Plugin{p}, "")
	m.width = 80
	m.height = 10
	m.pluginErrs = map[plugin.ID]error{"clock": errors.New("boom")}

	view := m.View()
	if !strings.Contains(view, "Clock: boom") {
		t.Fatalf("expected error line, got:\n%s", view)
	}
}

func TestModelPerPluginIntervalOverride(t *testing.T) {
	cfg := config.Default()
	cfg.UpdateInterval = config.Duration{Duration: 2 * time.Second}
	cfg.Plugins.Config = map[string]map[string]any{
		"cpu": {plugin.GlobalPluginIntervalKey: "500ms"},
	}
	cpuPlugin := &collectCountingPlugin{id: "cpu", name: "CPU"}
	memPlugin := &collectCountingPlugin{id: "memory", name: "Memory"}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), []plugin.Plugin{cpuPlugin, memPlugin}, "")

	now := time.Unix(100, 0)
	m.refreshPluginSchedules(now)
	m.pluginNextDue[cpuPlugin.ID()] = now
	m.pluginNextDue[memPlugin.ID()] = now

	cmds := m.collectDuePlugins(now)
	if len(cmds) != 2 {
		t.Fatalf("expected 2 due collects, got %d", len(cmds))
	}
	if got := m.pluginIntervals[cpuPlugin.ID()]; got != 500*time.Millisecond {
		t.Fatalf("expected cpu interval 500ms, got %v", got)
	}
	if got := m.pluginIntervals[memPlugin.ID()]; got != 2*time.Second {
		t.Fatalf("expected memory interval default 2s, got %v", got)
	}
	if got := m.schedulerTickInterval(); got != 500*time.Millisecond {
		t.Fatalf("expected scheduler tick interval 500ms, got %v", got)
	}
}

func TestModelCollectDuePluginsSkipsWhenInFlight(t *testing.T) {
	cfg := config.Default()
	cfg.UpdateInterval = config.Duration{Duration: time.Second}
	p := &collectCountingPlugin{id: "cpu", name: "CPU"}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), []plugin.Plugin{p}, "")

	now := time.Unix(200, 0)
	m.refreshPluginSchedules(now)
	m.pluginNextDue[p.ID()] = now
	m.pluginInFlight[p.ID()] = true

	cmds := m.collectDuePlugins(now)
	if len(cmds) != 0 {
		t.Fatalf("expected no collect while in-flight, got %d", len(cmds))
	}
	if got := m.pluginNextDue[p.ID()]; !got.After(now) {
		t.Fatalf("expected next due to advance after skip, got %v", got)
	}
}

func TestModelCollectCompletionClearsInFlight(t *testing.T) {
	cfg := config.Default()
	cfg.UpdateInterval = config.Duration{Duration: time.Second}
	p := &collectCountingPlugin{id: "cpu", name: "CPU"}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), []plugin.Plugin{p}, "")

	now := time.Unix(300, 0)
	m.refreshPluginSchedules(now)
	m.pluginNextDue[p.ID()] = now

	cmds := m.collectDuePlugins(now)
	if len(cmds) != 1 {
		t.Fatalf("expected one collect command, got %d", len(cmds))
	}
	if !m.pluginInFlight[p.ID()] {
		t.Fatalf("expected plugin to be in-flight after dispatch")
	}

	msg := cmds[0]()
	nextModel, _ := m.Update(msg)
	updated, ok := nextModel.(Model)
	if !ok {
		t.Fatalf("unexpected model type %T", nextModel)
	}
	if updated.pluginInFlight[p.ID()] {
		t.Fatalf("expected plugin in-flight to be cleared on collect completion")
	}
}

func TestModelPresetLoadFromKeyWorkflow(t *testing.T) {
	cfg := config.Default()
	cfg.Presets = map[string]config.PresetConfig{
		"5": {
			LayoutMode:     "vertical",
			LayoutColumns:  1,
			UpdateInterval: config.Duration{Duration: 1500 * time.Millisecond},
			VisibleBoxes:   []string{"cpu"},
		},
	}
	plugins := []plugin.Plugin{
		&collectCountingPlugin{id: "cpu", name: "CPU"},
		&collectCountingPlugin{id: "memory", name: "Memory"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins, "")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	modelWithPrompt := next.(Model)
	if !modelWithPrompt.presetMode {
		t.Fatalf("expected preset mode prompt")
	}

	next, _ = modelWithPrompt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	updated := next.(Model)
	if updated.presetMode {
		t.Fatalf("expected preset mode to close after selection")
	}
	if updated.cfg.Layout.Mode != "vertical" || updated.cfg.Layout.Columns != 1 {
		t.Fatalf("unexpected layout after preset: %+v", updated.cfg.Layout)
	}
	if updated.cfg.UpdateInterval.Duration != 1500*time.Millisecond {
		t.Fatalf("unexpected interval after preset: %v", updated.cfg.UpdateInterval.Duration)
	}
	if updated.hiddenBoxes["cpu"] {
		t.Fatalf("expected cpu visible")
	}
	if !updated.hiddenBoxes["memory"] {
		t.Fatalf("expected memory hidden")
	}
}

func TestModelInputCaptureSuspendsGlobalShortcuts(t *testing.T) {
	t.Parallel()

	p := &capturingPlugin{modelPlugin: modelPlugin{id: "process", name: "Processes"}, capturing: true}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), []plugin.Plugin{p}, "")

	// "q" must not quit while the plugin is capturing input; the key goes to
	// the plugin instead.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, isQuit := msg.(tea.QuitMsg); isQuit {
				t.Fatalf("expected q not to quit while plugin captures input")
			}
		}
	}
	if len(p.keys) != 1 || p.keys[0] != "q" {
		t.Fatalf("expected key routed to plugin, got %v", p.keys)
	}

	// Global shortcuts like help must not fire either.
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if next.(Model).showHelp {
		t.Fatalf("expected help shortcut suspended while plugin captures input")
	}

	// Ctrl+C remains the escape hatch.
	_, cmd = next.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("expected ctrl+c to quit")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatalf("expected ctrl+c to produce tea.QuitMsg, got %T", cmd())
	}
}

func TestModelInputCaptureIgnoredWhenNotCapturing(t *testing.T) {
	t.Parallel()

	p := &capturingPlugin{modelPlugin: modelPlugin{id: "process", name: "Processes"}, capturing: false}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), []plugin.Plugin{p}, "")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("expected q to quit when no plugin captures input")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatalf("expected q to produce tea.QuitMsg, got %T", cmd())
	}
}

func TestModelInputCaptureIgnoredWhenPluginHidden(t *testing.T) {
	t.Parallel()

	p := &capturingPlugin{modelPlugin: modelPlugin{id: "process", name: "Processes"}, capturing: true}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), []plugin.Plugin{p}, "")
	m.hiddenBoxes["process"] = true

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("expected q to quit when capturing plugin is hidden")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatalf("expected q to produce tea.QuitMsg, got %T", cmd())
	}
}

func TestModelMaybeReloadConfigUpdatesInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dtop.conf")
	if err := os.WriteFile(path, []byte("update_interval = \"2s\"\nlive_reload = true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), nil, path)
	if m.cfg.UpdateInterval.Duration != 2*time.Second {
		t.Fatalf("expected initial interval 2s, got %v", m.cfg.UpdateInterval.Duration)
	}

	// Bump modtime by updating the file contents.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("update_interval = \"3s\"\nlive_reload = true\n"), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	m.maybeReloadConfig()
	if m.cfg.UpdateInterval.Duration != 3*time.Second {
		t.Fatalf("expected reloaded interval 3s, got %v", m.cfg.UpdateInterval.Duration)
	}
}

func TestModelPresetSaveFromKeyWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dtop.conf")
	if err := os.WriteFile(path, []byte("update_interval = \"2s\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Layout.Mode = "flow"
	cfg.Layout.Columns = 2
	cfg.UpdateInterval = config.Duration{Duration: 1500 * time.Millisecond}

	plugins := []plugin.Plugin{
		&collectCountingPlugin{id: "cpu", name: "CPU"},
		&collectCountingPlugin{id: "memory", name: "Memory"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins, path)
	m.hiddenBoxes["memory"] = true

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	modelWithPrompt := next.(Model)
	if !modelWithPrompt.presetSaveMode {
		t.Fatalf("expected preset save mode prompt")
	}
	next, _ = modelWithPrompt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	updated := next.(Model)
	if updated.presetSaveMode {
		t.Fatalf("expected preset save mode to close after selection")
	}
	preset, ok := updated.cfg.Presets["7"]
	if !ok {
		t.Fatalf("expected preset slot 7 saved")
	}
	if preset.LayoutMode != "flow" || preset.LayoutColumns != 2 {
		t.Fatalf("unexpected saved layout preset: %+v", preset)
	}
	if preset.UpdateInterval.Duration != 1500*time.Millisecond {
		t.Fatalf("unexpected saved interval: %v", preset.UpdateInterval.Duration)
	}
	if len(preset.VisibleBoxes) != 1 || preset.VisibleBoxes[0] != "cpu" {
		t.Fatalf("unexpected saved visible boxes: %+v", preset.VisibleBoxes)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(b), "[presets.7]") {
		t.Fatalf("expected persisted preset slot in config file")
	}
}

func TestModelPresetDeleteFromKeyWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dtop.conf")
	if err := os.WriteFile(path, []byte("update_interval = \"2s\"\n[presets.2]\nlayout_mode = \"grid\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), nil, path)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	modelWithPrompt := next.(Model)
	if !modelWithPrompt.presetDeleteMode {
		t.Fatalf("expected preset delete mode prompt")
	}
	next, _ = modelWithPrompt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	updated := next.(Model)
	if updated.presetDeleteMode {
		t.Fatalf("expected preset delete mode to close after selection")
	}
	if _, ok := updated.cfg.Presets["2"]; ok {
		t.Fatalf("expected preset slot 2 deleted from model config")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(b), "[presets.2]") {
		t.Fatalf("expected preset slot 2 removed from persisted config")
	}
}

func TestModelPresetExportFromKeyWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dtop.conf")
	if err := os.WriteFile(path, []byte("update_interval = \"2s\"\n[presets.8]\nlayout_mode = \"vertical\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), nil, path)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	modelWithPrompt := next.(Model)
	if !modelWithPrompt.presetExportMode {
		t.Fatalf("expected preset export mode prompt")
	}
	next, _ = modelWithPrompt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})
	updated := next.(Model)
	if updated.presetExportMode {
		t.Fatalf("expected preset export mode to close after selection")
	}
	exportPath := filepath.Join(dir, "presets", "preset-8.toml")
	b, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read exported preset: %v", err)
	}
	if !strings.Contains(string(b), "[presets.8]") {
		t.Fatalf("expected exported preset slot 8 in file")
	}
}

func TestModelPresetImportFromKeyWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dtop.conf")
	if err := os.WriteFile(path, []byte("update_interval = \"2s\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	importedPreset := config.PresetConfig{
		LayoutMode:     "vertical",
		LayoutColumns:  1,
		UpdateInterval: config.Duration{Duration: 750 * time.Millisecond},
		VisibleBoxes:   []string{"cpu"},
	}
	if _, err := config.ExportPreset(path, "6", importedPreset); err != nil {
		t.Fatalf("export preset: %v", err)
	}

	plugins := []plugin.Plugin{
		&collectCountingPlugin{id: "cpu", name: "CPU"},
		&collectCountingPlugin{id: "memory", name: "Memory"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins, path)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	modelWithPrompt := next.(Model)
	if !modelWithPrompt.presetImportMode {
		t.Fatalf("expected preset import mode prompt")
	}
	next, _ = modelWithPrompt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	updated := next.(Model)
	if updated.presetImportMode {
		t.Fatalf("expected preset import mode to close after selection")
	}
	preset, ok := updated.cfg.Presets["6"]
	if !ok {
		t.Fatalf("expected imported preset slot 6")
	}
	if preset.LayoutMode != "vertical" || preset.LayoutColumns != 1 {
		t.Fatalf("unexpected imported preset layout: %+v", preset)
	}
	if updated.cfg.UpdateInterval.Duration != 750*time.Millisecond {
		t.Fatalf("expected imported preset to be applied immediately, got %v", updated.cfg.UpdateInterval.Duration)
	}
	if updated.hiddenBoxes["cpu"] {
		t.Fatalf("expected cpu visible after imported preset apply")
	}
	if !updated.hiddenBoxes["memory"] {
		t.Fatalf("expected memory hidden after imported preset apply")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(b), "[presets.6]") {
		t.Fatalf("expected imported preset persisted to config")
	}
}

// Fix 3: per-plugin interval changes in the config file must take effect after
// applyReloadedConfig, without requiring a resize or restart.
func TestModelApplyReloadedConfigRefreshesIntervals(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.UpdateInterval = config.Duration{Duration: 2 * time.Second}
	p := &collectCountingPlugin{id: "cpu", name: "CPU"}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), []plugin.Plugin{p}, "")

	if got := m.pluginIntervals[p.ID()]; got != 2*time.Second {
		t.Fatalf("expected initial interval 2s, got %v", got)
	}

	nextCfg := config.Default()
	nextCfg.UpdateInterval = config.Duration{Duration: 2 * time.Second}
	nextCfg.Plugins.Config = map[string]map[string]any{
		"cpu": {plugin.GlobalPluginIntervalKey: "500ms"},
	}
	m.applyReloadedConfig(nextCfg)

	if got := m.pluginIntervals[p.ID()]; got != 500*time.Millisecond {
		t.Fatalf("expected refreshed interval 500ms after reload, got %v", got)
	}
}

// Fix 2: a plugin with column = 2 in its config must land in the second column
// (0-based index 1) when groupPluginsByColumn is called.
func TestModelGroupPluginsByColumnPinning(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Layout.Mode = "grid"
	cfg.Layout.Columns = 2
	cfg.Plugins.Config = map[string]map[string]any{
		"process": {plugin.GlobalPluginColumnKey: int64(2)},
	}
	plugins := []plugin.Plugin{
		&modelPlugin{id: "cpu", name: "CPU"},
		&modelPlugin{id: "memory", name: "Memory"},
		&modelPlugin{id: "process", name: "Process"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins, "")

	columns := m.groupPluginsByColumn(plugins, 2)
	if len(columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(columns))
	}

	var processInCol1 bool
	for _, p := range columns[1] {
		if p.ID() == "process" {
			processInCol1 = true
		}
	}
	if !processInCol1 {
		t.Fatalf("expected 'process' pinned to column index 1; col0=%v col1=%v",
			pluginIDs(columns[0]), pluginIDs(columns[1]))
	}
}

// Fix 2: without any pinning, groupPluginsByColumn must produce the same
// distribution as the original GridColumns-based approach.
func TestModelGroupPluginsByColumnNoPinningMatchesGridColumns(t *testing.T) {
	t.Parallel()

	plugins := []plugin.Plugin{
		&modelPlugin{id: "a"}, &modelPlugin{id: "b"}, &modelPlugin{id: "c"},
		&modelPlugin{id: "d"}, &modelPlugin{id: "e"},
	}
	m := NewModel(context.Background(), nil, config.Default(), theme.Default(), plugins, "")

	columns := m.groupPluginsByColumn(plugins, 2)
	sizes := ui.GridColumns(len(plugins), 2) // expected: [3, 2]

	for col, want := range sizes {
		if got := len(columns[col]); got != want {
			t.Errorf("col %d: got %d plugins, want %d", col, got, want)
		}
	}
	// Verify declaration order is preserved within each column.
	if columns[0][0].ID() != "a" || columns[0][1].ID() != "b" || columns[0][2].ID() != "c" {
		t.Errorf("unexpected col0 order: %v", pluginIDs(columns[0]))
	}
	if columns[1][0].ID() != "d" || columns[1][1].ID() != "e" {
		t.Errorf("unexpected col1 order: %v", pluginIDs(columns[1]))
	}
}

// Fix 1: in a 2-column grid layout, pluginContentWidth must return a width
// narrower than the full terminal width, not ui.ContentWidth(m.width).
func TestModelPluginContentWidthInGridLayout(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Layout.Mode = "grid"
	cfg.Layout.Columns = 2
	plugins := []plugin.Plugin{
		&modelPlugin{id: "a", name: "A"},
		&modelPlugin{id: "b", name: "B"},
	}
	m := NewModel(context.Background(), nil, cfg, theme.Default(), plugins, "")
	m.width = 80
	m.height = 30

	fullWidth := ui.ContentWidth(m.width) // 78 (80 - 2 border)
	wA := m.pluginContentWidth("a")
	wB := m.pluginContentWidth("b")

	if wA >= fullWidth {
		t.Errorf("pluginContentWidth(a) = %d, want < %d (full-width content)", wA, fullWidth)
	}
	if wB >= fullWidth {
		t.Errorf("pluginContentWidth(b) = %d, want < %d (full-width content)", wB, fullWidth)
	}
	if wA != wB {
		t.Errorf("both plugins are in equal-width columns: got %d and %d", wA, wB)
	}
}

func pluginIDs(plugins []plugin.Plugin) []plugin.ID {
	ids := make([]plugin.ID, len(plugins))
	for i, p := range plugins {
		ids[i] = p.ID()
	}
	return ids
}
