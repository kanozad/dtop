package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewMenuState(t *testing.T) {
	t.Parallel()

	items := []MenuItem{
		{Label: "alpha", Value: "a"},
		{Label: "beta", Value: "b"},
	}
	s := NewMenuState(items)
	if s.Selected != 0 {
		t.Fatalf("Selected=%d want 0", s.Selected)
	}
	if s.Offset != 0 {
		t.Fatalf("Offset=%d want 0", s.Offset)
	}
	if len(s.Items) != 2 {
		t.Fatalf("len(Items)=%d want 2", len(s.Items))
	}
}

func TestMoveSelectionClampsToEnd(t *testing.T) {
	t.Parallel()

	s := NewMenuState([]MenuItem{
		{Label: "a", Value: "1"},
		{Label: "b", Value: "2"},
		{Label: "c", Value: "3"},
	})

	s.MoveSelection(10, 0)
	if s.Selected != 2 {
		t.Fatalf("Selected=%d want 2 (clamped to end)", s.Selected)
	}
}

func TestMoveSelectionClampsToStart(t *testing.T) {
	t.Parallel()

	s := NewMenuState([]MenuItem{
		{Label: "a", Value: "1"},
		{Label: "b", Value: "2"},
	})

	s.MoveSelection(-5, 0)
	if s.Selected != 0 {
		t.Fatalf("Selected=%d want 0 (clamped to start)", s.Selected)
	}
}

func TestMoveSelectionAdjustsOffset(t *testing.T) {
	t.Parallel()

	items := make([]MenuItem, 20)
	for i := range items {
		items[i] = MenuItem{Label: "item", Value: "v"}
	}
	s := NewMenuState(items)

	// Move past the visible window of 5.
	s.MoveSelection(7, 5)
	if s.Selected != 7 {
		t.Fatalf("Selected=%d want 7", s.Selected)
	}
	if s.Offset != 3 {
		t.Fatalf("Offset=%d want 3", s.Offset)
	}

	// Move back up past the offset.
	s.MoveSelection(-6, 5)
	if s.Selected != 1 {
		t.Fatalf("Selected=%d want 1", s.Selected)
	}
	if s.Offset != 1 {
		t.Fatalf("Offset=%d want 1", s.Offset)
	}
}

func TestMoveSelectionEmptyItems(t *testing.T) {
	t.Parallel()

	s := NewMenuState(nil)
	s.MoveSelection(1, 5)
	if s.Selected != 0 {
		t.Fatalf("Selected=%d want 0 for empty menu", s.Selected)
	}
}

func TestSelectedItem(t *testing.T) {
	t.Parallel()

	s := NewMenuState([]MenuItem{
		{Label: "alpha", Value: "a"},
		{Label: "beta", Value: "b"},
	})
	s.Selected = 1
	got := s.SelectedItem()
	if got.Value != "b" {
		t.Fatalf("SelectedItem().Value=%q want %q", got.Value, "b")
	}
}

func TestSelectedItemOutOfRange(t *testing.T) {
	t.Parallel()

	s := NewMenuState([]MenuItem{{Label: "a", Value: "1"}})
	s.Selected = 5
	got := s.SelectedItem()
	if got.Value != "" {
		t.Fatalf("expected empty MenuItem, got %q", got.Value)
	}
}

func TestRenderMenuEmpty(t *testing.T) {
	t.Parallel()

	s := NewMenuState(nil)
	out := RenderMenu(s, MenuOpts{
		Width:       30,
		NormalStyle: lipgloss.NewStyle(),
		ActiveStyle: lipgloss.NewStyle(),
	})
	if !strings.Contains(out, "(empty)") {
		t.Fatalf("expected (empty), got %q", out)
	}
}

func TestRenderMenuHighlightsSelected(t *testing.T) {
	t.Parallel()

	s := NewMenuState([]MenuItem{
		{Label: "one", Value: "1"},
		{Label: "two", Value: "2"},
		{Label: "three", Value: "3"},
	})
	s.Selected = 1
	out := RenderMenu(s, MenuOpts{
		Width:        40,
		NormalStyle:  lipgloss.NewStyle(),
		ActiveStyle:  lipgloss.NewStyle(),
		CursorPrefix: ">> ",
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[1], ">> two") {
		t.Fatalf("expected selected item to have cursor prefix, got %q", lines[1])
	}
	if strings.Contains(lines[0], ">>") {
		t.Fatalf("non-selected item should not have cursor prefix, got %q", lines[0])
	}
}

func TestRenderMenuRespectsMaxVisible(t *testing.T) {
	t.Parallel()

	items := make([]MenuItem, 10)
	for i := range items {
		items[i] = MenuItem{Label: "item", Value: "v"}
	}
	s := NewMenuState(items)
	out := RenderMenu(s, MenuOpts{
		Width:       40,
		MaxVisible:  3,
		NormalStyle: lipgloss.NewStyle(),
		ActiveStyle: lipgloss.NewStyle(),
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 visible lines, got %d", len(lines))
	}
}
