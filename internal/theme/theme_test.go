package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mld.com/dtop/internal/termcap"
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

func TestBoxChromeDefault(t *testing.T) {
	t.Parallel()

	th := Default()
	v, h := th.BoxChrome()
	// Default: RoundedBorder (1 top + 1 bottom) + Padding(0,1) = 2 vertical, 4 horizontal.
	if v != 2 {
		t.Fatalf("vertical chrome: got %d want 2", v)
	}
	if h != 4 {
		t.Fatalf("horizontal chrome: got %d want 4", h)
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

func TestThemeWithCapabilities(t *testing.T) {
	th := Default()
	if !th.UTF8 {
		t.Fatalf("expected default theme utf8=true")
	}
	th = th.WithCapabilities(termcap.Capabilities{UTF8: false})
	if th.UTF8 {
		t.Fatalf("expected theme utf8 flag to be updated")
	}
}

func TestRenderBoxHeightEnforcement(t *testing.T) {
	t.Parallel()

	th := Default()
	vChrome, _ := th.BoxChrome()

	// height is inner content rows (not outer box height).
	// Outer box height = height + vChrome.
	tests := []struct {
		name   string
		title  string
		body   string
		height int // inner content rows
	}{
		{
			name:   "single line body fits",
			body:   "line 1",
			height: 1,
		},
		{
			name:   "multi line body truncated",
			body:   "line 1\nline 2\nline 3",
			height: 1,
		},
		{
			name:   "title + body truncated",
			title:  "Title",
			body:   "line 1\nline 2",
			height: 1, // only the title fits; body is truncated
		},
		{
			name:   "zero height budget",
			body:   "something",
			height: 0, // no height constraint applied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.RenderBox(tt.title, tt.body, 20, tt.height)
			lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
			// Outer box = content rows + chrome. When height > 0, content is capped at height rows.
			if tt.height > 0 {
				want := tt.height + vChrome
				if len(lines) > want {
					t.Errorf("RenderBox() output %d lines, want <= %d", len(lines), want)
				}
			}
		})
	}
}
