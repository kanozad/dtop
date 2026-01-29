package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func writeThemeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, "dtop", "themes", name+".toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestFromNameInvalid(t *testing.T) {
	t.Parallel()

	_, err := FromName("Bad Name")
	if err == nil {
		t.Fatalf("expected error for invalid theme name")
	}
}

func TestFromNameUnknown(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := FromName("missing")
	if err == nil {
		t.Fatalf("expected error for unknown theme")
	}
}

func TestFromNameLoadsFile(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	writeThemeFile(t, dir, "custom", `
[header]
fg = "7"
bold = true

[box]
border = "double"
padding = 1
`)

	if _, err := FromName("custom"); err != nil {
		t.Fatalf("FromName: %v", err)
	}
}
