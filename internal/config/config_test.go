package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestLoadDefaultWhenMissing(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("expected default config, got %+v", got)
	}
}

func TestLoadLegacyFallback(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	legacy := filepath.Join(dir, "dtop", "dtop.toml")
	writeFile(t, legacy, `
update_interval = "3s"

[theme]
name = "legacy"

[plugins]
enabled = ["clock", "cpu"]
`)

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme.Name != "legacy" {
		t.Fatalf("theme name: got %q", got.Theme.Name)
	}
	if got.UpdateInterval.Duration != 3*time.Second {
		t.Fatalf("update interval: got %v", got.UpdateInterval.Duration)
	}
	if len(got.Plugins.Enabled) != 2 {
		t.Fatalf("enabled plugins: got %v", got.Plugins.Enabled)
	}
	if got.Plugins.Config == nil {
		t.Fatalf("expected non-nil plugin config map")
	}
}

func TestLoadDefaultsForInvalidInterval(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "dtop", "dtop.conf")
	writeFile(t, path, `
update_interval = "0s"

[theme]
name = "default"
`)

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.UpdateInterval.Duration != Default().UpdateInterval.Duration {
		t.Fatalf("expected default interval, got %v", got.UpdateInterval.Duration)
	}
}

func TestLoadPluginIntervalValid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "dtop", "dtop.conf")
	writeFile(t, path, `
update_interval = "2s"

[plugins]
enabled = ["clock", "cpu"]

[plugins.config.cpu]
interval = "750ms"
per_core = true
`)

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	raw := got.Plugins.Config["cpu"]["interval"]
	v, ok := raw.(string)
	if !ok {
		t.Fatalf("expected normalized plugin interval string, got %#v", raw)
	}
	if v != "750ms" {
		t.Fatalf("expected normalized interval 750ms, got %q", v)
	}
}

func TestLoadPluginIntervalInvalid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "dtop", "dtop.conf")
	writeFile(t, path, `
update_interval = "2s"

[plugins]
enabled = ["clock", "cpu"]

[plugins.config.cpu]
interval = "nonsense"
`)

	_, err := Load("")
	if err == nil {
		t.Fatalf("expected invalid plugin interval error")
	}
	if !strings.Contains(err.Error(), "plugins.config.cpu.interval") {
		t.Fatalf("expected interval path in error, got %v", err)
	}
}

func TestResolvePathPrefersPrimaryThenLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	primary := filepath.Join(dir, "dtop", "dtop.conf")
	legacy := filepath.Join(dir, "dtop", "dtop.toml")

	// none exist -> primary
	got, err := ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != primary {
		t.Fatalf("expected primary path, got %s", got)
	}

	writeFile(t, legacy, "update_interval = \"2s\"\n")
	got, err = ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != legacy {
		t.Fatalf("expected legacy path fallback, got %s", got)
	}

	writeFile(t, primary, "update_interval = \"2s\"\n")
	got, err = ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != primary {
		t.Fatalf("expected primary path precedence, got %s", got)
	}
}

func TestLoadPresetsValid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "dtop", "dtop.conf")
	writeFile(t, path, `
update_interval = "2s"
live_reload = true

[presets.1]
layout_mode = "flow"
layout_columns = 3
update_interval = "1s"
visible_boxes = ["cpu", "memory"]
`)

	got, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.LiveReload {
		t.Fatalf("expected live_reload=true")
	}
	preset, ok := got.Presets["1"]
	if !ok {
		t.Fatalf("expected preset slot 1")
	}
	if preset.LayoutMode != "flow" || preset.LayoutColumns != 3 {
		t.Fatalf("unexpected preset layout: %+v", preset)
	}
	if preset.UpdateInterval.Duration != time.Second {
		t.Fatalf("unexpected preset interval: %v", preset.UpdateInterval.Duration)
	}
}

func TestLoadPresetInvalidSlot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "dtop", "dtop.conf")
	writeFile(t, path, `
[presets.bad]
layout_mode = "grid"
`)

	_, err := Load("")
	if err == nil {
		t.Fatalf("expected invalid preset slot error")
	}
	if !strings.Contains(err.Error(), "invalid slot") {
		t.Fatalf("expected invalid slot message, got %v", err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "dtop", "dtop.conf")

	cfg := Default()
	cfg.LiveReload = true
	cfg.Layout.Mode = "flow"
	cfg.Layout.Columns = 3
	cfg.Presets = map[string]PresetConfig{
		"7": {
			LayoutMode:     "vertical",
			UpdateInterval: Duration{Duration: time.Second},
			VisibleBoxes:   []string{"cpu", "process"},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.LiveReload {
		t.Fatalf("expected live_reload true after roundtrip")
	}
	if got.Layout.Mode != "flow" || got.Layout.Columns != 3 {
		t.Fatalf("unexpected layout after roundtrip: %+v", got.Layout)
	}
	if _, ok := got.Presets["7"]; !ok {
		t.Fatalf("expected preset slot 7")
	}
}

func TestExportPreset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "dtop", "dtop.conf")

	preset := PresetConfig{
		LayoutMode:     "flow",
		LayoutColumns:  3,
		UpdateInterval: Duration{Duration: 1500 * time.Millisecond},
		VisibleBoxes:   []string{"cpu", "memory"},
	}
	exportPath, err := ExportPreset(cfgPath, "3", preset)
	if err != nil {
		t.Fatalf("ExportPreset: %v", err)
	}
	expected := filepath.Join(dir, "dtop", "presets", "preset-3.toml")
	if exportPath != expected {
		t.Fatalf("expected export path %q, got %q", expected, exportPath)
	}
	b, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}
	if !strings.Contains(string(b), "[presets.3]") {
		t.Fatalf("expected exported preset table in TOML")
	}
	got, err := Load(exportPath)
	if err != nil {
		t.Fatalf("Load export file: %v", err)
	}
	if gotPreset, ok := got.Presets["3"]; !ok {
		t.Fatalf("expected exported preset slot 3")
	} else {
		if gotPreset.LayoutMode != "flow" || gotPreset.LayoutColumns != 3 {
			t.Fatalf("unexpected exported preset layout: %+v", gotPreset)
		}
		if gotPreset.UpdateInterval.Duration != 1500*time.Millisecond {
			t.Fatalf("unexpected exported preset interval: %v", gotPreset.UpdateInterval.Duration)
		}
	}
}

func TestImportPreset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgPath := filepath.Join(dir, "dtop", "dtop.conf")

	wanted := PresetConfig{
		LayoutMode:     "vertical",
		LayoutColumns:  1,
		UpdateInterval: Duration{Duration: 750 * time.Millisecond},
		VisibleBoxes:   []string{"cpu"},
	}
	if _, err := ExportPreset(cfgPath, "9", wanted); err != nil {
		t.Fatalf("ExportPreset: %v", err)
	}
	got, err := ImportPreset(cfgPath, "9")
	if err != nil {
		t.Fatalf("ImportPreset: %v", err)
	}
	if got.LayoutMode != wanted.LayoutMode || got.LayoutColumns != wanted.LayoutColumns {
		t.Fatalf("unexpected imported preset layout: %+v", got)
	}
	if got.UpdateInterval.Duration != wanted.UpdateInterval.Duration {
		t.Fatalf("unexpected imported preset interval: %v", got.UpdateInterval.Duration)
	}
	if len(got.VisibleBoxes) != 1 || got.VisibleBoxes[0] != "cpu" {
		t.Fatalf("unexpected imported visible boxes: %+v", got.VisibleBoxes)
	}
}
