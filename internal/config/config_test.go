package config

import (
	"os"
	"path/filepath"
	"reflect"
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
