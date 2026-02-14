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

	"mld.com/dtop/internal/config"
	"mld.com/dtop/internal/plugin"
	"mld.com/dtop/internal/theme"
	"mld.com/dtop/pkg/collector"
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
