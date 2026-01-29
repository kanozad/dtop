package config

import (
	"errors"
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

type Config struct {
	UpdateInterval Duration      `toml:"update_interval"`
	Theme          ThemeConfig   `toml:"theme"`
	Plugins        PluginsConfig `toml:"plugins"`
}

func Default() Config {
	return Config{
		UpdateInterval: Duration{Duration: 2 * time.Second},
		Theme: ThemeConfig{
			Name: "default",
		},
		Plugins: PluginsConfig{
			Enabled: []string{"clock"},
			Config:  map[string]map[string]any{},
		},
	}
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

	if cfg.UpdateInterval.Duration <= 0 {
		cfg.UpdateInterval = Default().UpdateInterval
	}

	return cfg, nil
}
