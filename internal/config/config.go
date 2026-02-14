package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Duration is a TOML-friendly time.Duration wrapper.
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		d.Duration = 0
		return nil
	}
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

type ThemeConfig struct {
	Name string `toml:"name"`
}

type PluginsConfig struct {
	Enabled []string                  `toml:"enabled"`
	Config  map[string]map[string]any `toml:"config"`
}

// PresetConfig describes a loadable UI preset profile.
type PresetConfig struct {
	LayoutMode     string   `toml:"layout_mode"`
	LayoutColumns  int      `toml:"layout_columns"`
	UpdateInterval Duration `toml:"update_interval"`
	VisibleBoxes   []string `toml:"visible_boxes"`
}

// LayoutConfig controls how plugin boxes are arranged on screen.
type LayoutConfig struct {
	// Mode is the layout mode: "vertical" (single column) or "grid" (two columns).
	Mode string `toml:"mode"`
	// Columns is the number of columns in grid mode (default 2).
	Columns int `toml:"columns"`
}

type Config struct {
	UpdateInterval Duration                `toml:"update_interval"`
	LiveReload     bool                    `toml:"live_reload"`
	Theme          ThemeConfig             `toml:"theme"`
	Layout         LayoutConfig            `toml:"layout"`
	Plugins        PluginsConfig           `toml:"plugins"`
	Presets        map[string]PresetConfig `toml:"presets"`
}

type presetExportDocument struct {
	Presets map[string]PresetConfig `toml:"presets"`
}

func Default() Config {
	return Config{
		UpdateInterval: Duration{Duration: 2 * time.Second},
		LiveReload:     false,
		Theme: ThemeConfig{
			Name: "default",
		},
		Layout: LayoutConfig{
			Mode:    "grid",
			Columns: 2,
		},
		Plugins: PluginsConfig{
			Enabled: []string{"clock", "cpu"},
			Config:  map[string]map[string]any{},
		},
		Presets: map[string]PresetConfig{},
	}
}

func validatePresetEntry(slot string, preset PresetConfig) error {
	if !isValidPresetSlot(slot) {
		return fmt.Errorf("presets.%s: invalid slot (use 0-9)", slot)
	}
	switch preset.LayoutMode {
	case "", "vertical", "grid", "flow":
	default:
		return fmt.Errorf("presets.%s.layout_mode: invalid mode %q", slot, preset.LayoutMode)
	}
	if preset.LayoutColumns < 0 {
		return fmt.Errorf("presets.%s.layout_columns: must be >= 0", slot)
	}
	if preset.UpdateInterval.Duration < 0 {
		return fmt.Errorf("presets.%s.update_interval: must be >= 0", slot)
	}
	return nil
}

func isValidPresetSlot(slot string) bool {
	return len(slot) == 1 && slot[0] >= '0' && slot[0] <= '9'
}

func defaultPaths() (string, string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "dtop", "dtop.conf"), filepath.Join(dir, "dtop", "dtop.toml"), nil
}

func DefaultPath() (string, error) {
	primary, _, err := defaultPaths()
	return primary, err
}

// ResolvePath resolves the config file path that should be used for reads and
// live-reload tracking. If path is empty, it prefers dtop.conf, then legacy
// dtop.toml if present, otherwise returns the default dtop.conf path.
func ResolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	primary, legacy, err := defaultPaths()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(primary); err == nil {
		return primary, nil
	}
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	}
	return primary, nil
}

// Save writes cfg as TOML to path. If path is empty, ResolvePath is used.
func Save(path string, cfg Config) error {
	resolved, err := ResolvePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return err
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(resolved, b, 0o644)
}

// PresetExportPath returns the standard file path used when exporting a preset
// slot into a standalone TOML document.
func PresetExportPath(path, slot string) (string, error) {
	if !isValidPresetSlot(slot) {
		return "", fmt.Errorf("invalid slot %q (use 0-9)", slot)
	}
	resolved, err := ResolvePath(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(resolved), "presets", fmt.Sprintf("preset-%s.toml", slot)), nil
}

// ExportPreset writes a single preset slot to a standalone TOML file.
func ExportPreset(path, slot string, preset PresetConfig) (string, error) {
	exportPath, err := PresetExportPath(path, slot)
	if err != nil {
		return "", err
	}
	doc := presetExportDocument{
		Presets: map[string]PresetConfig{slot: preset},
	}
	b, err := toml.Marshal(doc)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(exportPath, b, 0o644); err != nil {
		return "", err
	}
	return exportPath, nil
}

// ImportPreset reads a preset slot from a standalone exported TOML file.
func ImportPreset(path, slot string) (PresetConfig, error) {
	importPath, err := PresetExportPath(path, slot)
	if err != nil {
		return PresetConfig{}, err
	}
	b, err := os.ReadFile(importPath)
	if err != nil {
		return PresetConfig{}, err
	}
	var doc presetExportDocument
	if err := toml.Unmarshal(b, &doc); err != nil {
		return PresetConfig{}, err
	}
	preset, ok := doc.Presets[slot]
	if !ok {
		return PresetConfig{}, fmt.Errorf("preset %s not found in %s", slot, importPath)
	}
	if err := validatePresetEntry(slot, preset); err != nil {
		return PresetConfig{}, err
	}
	return preset, nil
}

// Load reads a TOML config file.
// If path is empty, DefaultPath() is used.
// If the file does not exist, Default() is returned.
func Load(path string) (Config, error) {
	isDefault := false
	var legacyPath string
	if path == "" {
		primaryPath, legacy, err := defaultPaths()
		if err != nil {
			return Config{}, err
		}
		path = primaryPath
		legacyPath = legacy
		isDefault = true
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if isDefault && legacyPath != "" && legacyPath != path {
				legacyBytes, legacyErr := os.ReadFile(legacyPath)
				if legacyErr == nil {
					b = legacyBytes
				} else if errors.Is(legacyErr, os.ErrNotExist) {
					return Default(), nil
				} else {
					return Config{}, legacyErr
				}
			} else {
				return Default(), nil
			}
		} else {
			return Config{}, err
		}
	}

	cfg := Default()
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}

	if cfg.Plugins.Config == nil {
		cfg.Plugins.Config = map[string]map[string]any{}
	}
	if cfg.Presets == nil {
		cfg.Presets = map[string]PresetConfig{}
	}

	if cfg.UpdateInterval.Duration <= 0 {
		cfg.UpdateInterval = Default().UpdateInterval
	}

	for id, pluginCfg := range cfg.Plugins.Config {
		if len(pluginCfg) == 0 {
			continue
		}
		raw, ok := pluginCfg["interval"]
		if !ok {
			continue
		}
		d, err := parseDurationAny(raw)
		if err != nil || d <= 0 {
			if err == nil {
				err = fmt.Errorf("must be > 0")
			}
			return Config{}, fmt.Errorf("plugins.config.%s.interval: %w", id, err)
		}
		// Normalize to a canonical string form for downstream consumers.
		pluginCfg["interval"] = d.String()
	}

	// Normalize layout config.
	switch cfg.Layout.Mode {
	case "vertical", "grid", "flow":
		// valid
	case "":
		cfg.Layout.Mode = Default().Layout.Mode
	default:
		cfg.Layout.Mode = Default().Layout.Mode
	}
	if cfg.Layout.Columns < 1 {
		cfg.Layout.Columns = Default().Layout.Columns
	}
	for slot, preset := range cfg.Presets {
		if err := validatePresetEntry(slot, preset); err != nil {
			return Config{}, err
		}
		// Persist normalized values back into the map.
		cfg.Presets[slot] = preset
	}

	return cfg, nil
}

func parseDurationAny(v any) (time.Duration, error) {
	switch val := v.(type) {
	case string:
		return time.ParseDuration(val)
	case Duration:
		return val.Duration, nil
	case time.Duration:
		return val, nil
	default:
		return 0, fmt.Errorf("must be a duration string")
	}
}
