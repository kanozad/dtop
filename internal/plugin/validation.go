package plugin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ConfigValidator can be implemented by plugins to advertise allowed config keys.
type ConfigValidator interface {
	AllowedConfigKeys() []string
}

const (
	// GlobalPluginIntervalKey is a reserved plugin config key supported by the
	// app scheduler for per-plugin collection cadence overrides.
	GlobalPluginIntervalKey = "interval"

	// GlobalPluginColumnKey is a reserved plugin config key that pins a plugin
	// to a specific column (1-based) in grid and flow layouts.
	GlobalPluginColumnKey = "column"
)

// ValidateConfig checks for unknown keys in cfg.
func ValidateConfig(id ID, cfg map[string]any, allowedKeys ...string) error {
	if len(cfg) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, key := range allowedKeys {
		allowed[key] = struct{}{}
	}
	allowed[GlobalPluginIntervalKey] = struct{}{}
	allowed[GlobalPluginColumnKey] = struct{}{}
	unknown := make([]string, 0, len(cfg))
	for key := range cfg {
		if _, ok := allowed[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("plugin %q: unknown config keys: %s", id, strings.Join(unknown, ", "))
}

// ShutdownAll calls Shutdown on plugins and joins any errors.
func ShutdownAll(ctx context.Context, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if p == nil {
			continue
		}
		if err := p.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
