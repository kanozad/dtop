package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderDialogDefaultWidth(t *testing.T) {
	t.Parallel()

	out := RenderDialog(DialogOpts{
		Title: "Test",
		Body:  "hello",
	})
	if !strings.Contains(out, "Test") {
		t.Fatal("expected title in output")
	}
	if !strings.Contains(out, "hello") {
		t.Fatal("expected body in output")
	}
}

func TestRenderDialogWithActions(t *testing.T) {
	t.Parallel()

	out := RenderDialog(DialogOpts{
		Title: "Confirm",
		Body:  "Are you sure?",
		Actions: []DialogAction{
			{Label: "OK", Selected: true},
			{Label: "Cancel", Selected: false},
		},
		Width:       40,
		ActionStyle: lipgloss.NewStyle(),
		ActiveStyle: lipgloss.NewStyle().Bold(true),
	})
	if !strings.Contains(out, "OK") {
		t.Fatal("expected OK action in output")
	}
	if !strings.Contains(out, "Cancel") {
		t.Fatal("expected Cancel action in output")
	}
}

func TestRenderDialogEmptyBody(t *testing.T) {
	t.Parallel()

	out := RenderDialog(DialogOpts{
		Title: "Empty",
		Width: 30,
	})
	if !strings.Contains(out, "Empty") {
		t.Fatal("expected title in output")
	}
}

func TestPlaceOverlay(t *testing.T) {
	t.Parallel()

	overlay := "box"
	out := PlaceOverlay(overlay, 80, 24)
	if !strings.Contains(out, "box") {
		t.Fatal("expected overlay content in output")
	}
	// Should be padded to fill 80 columns.
	lines := strings.Split(out, "\n")
	if len(lines) < 24 {
		t.Fatalf("expected at least 24 lines, got %d", len(lines))
	}
}
