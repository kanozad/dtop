package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func plainStyle() lipgloss.Style {
	return lipgloss.NewStyle()
}

func TestRenderGraph_Empty(t *testing.T) {
	if out := RenderGraph(nil, 0, 0, GraphOpts{}); out != "" {
		t.Errorf("expected empty, got %q", out)
	}
	if out := RenderGraph(nil, 5, 0, GraphOpts{}); out != "" {
		t.Errorf("expected empty, got %q", out)
	}
	if out := RenderGraph(nil, 0, 3, GraphOpts{}); out != "" {
		t.Errorf("expected empty, got %q", out)
	}
}

func TestRenderGraph_AllZero(t *testing.T) {
	data := make([]float64, 5)
	out := RenderGraph(data, 5, 1, GraphOpts{Min: 0, Max: 100, Style: plainStyle(), Fill: true})
	// All zeros should produce empty braille characters (U+2800).
	for _, r := range out {
		if r != 0x2800 {
			t.Errorf("expected blank braille, got %U", r)
		}
	}
}

func TestRenderGraph_AllMax(t *testing.T) {
	data := []float64{100, 100, 100}
	out := RenderGraph(data, 3, 1, GraphOpts{Min: 0, Max: 100, Style: plainStyle(), Fill: true})
	// All at max with fill should produce fully filled braille (dots 1,2,3,7 = 0x47).
	expected := rune(0x2800 + 0x47)
	for _, r := range out {
		if r != expected {
			t.Errorf("expected %U, got %U", expected, r)
		}
	}
}

func TestRenderGraph_RowCount(t *testing.T) {
	data := []float64{50, 50, 50, 50}
	out := RenderGraph(data, 4, 3, GraphOpts{Min: 0, Max: 100, Style: plainStyle(), Fill: true})
	rows := strings.Split(out, "\n")
	if len(rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(rows))
	}
}

func TestRenderGraph_DataShorterThanWidth(t *testing.T) {
	data := []float64{100}
	out := RenderGraph(data, 3, 1, GraphOpts{Min: 0, Max: 100, Style: plainStyle(), Fill: true})
	runes := []rune(out)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes, got %d", len(runes))
	}
	// First two should be blank, last should be filled.
	if runes[0] != 0x2800 || runes[1] != 0x2800 {
		t.Errorf("expected blank leading runes, got %U %U", runes[0], runes[1])
	}
	if runes[2] == 0x2800 {
		t.Errorf("trailing rune should not be blank")
	}
}

func TestRenderGraph_DataLongerThanWidth(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	// Width 3 should take the last 3 values: 30, 40, 50.
	out := RenderGraph(data, 3, 1, GraphOpts{Min: 0, Max: 100, Style: plainStyle(), Fill: true})
	runes := []rune(out)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes, got %d", len(runes))
	}
}

func TestRenderGraph_LineMode(t *testing.T) {
	data := []float64{50, 50, 50}
	out := RenderGraph(data, 3, 2, GraphOpts{Min: 0, Max: 100, Style: plainStyle(), Fill: false})
	rows := strings.Split(out, "\n")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Line mode should have some dots but not be fully filled.
	allFull := rune(0x2800 + 0x47)
	for _, r := range rows[1] {
		if r == allFull {
			t.Errorf("line mode should not produce fully filled cells in bottom row for 50%%")
		}
	}
}

func TestBrailleFromDots(t *testing.T) {
	// No dots.
	if r := brailleFromDots([4]bool{}); r != 0x2800 {
		t.Errorf("expected U+2800, got %U", r)
	}
	// All dots.
	if r := brailleFromDots([4]bool{true, true, true, true}); r != 0x2800+0x47 {
		t.Errorf("expected U+2847, got %U", r)
	}
	// Only top dot (dot 1).
	if r := brailleFromDots([4]bool{false, false, false, true}); r != 0x2801 {
		t.Errorf("expected U+2801, got %U", r)
	}
	// Only bottom dot (dot 7).
	if r := brailleFromDots([4]bool{true, false, false, false}); r != 0x2840 {
		t.Errorf("expected U+2840, got %U", r)
	}
}

func TestClampF(t *testing.T) {
	if v := clampF(-5, 0, 100); v != 0 {
		t.Errorf("expected 0, got %f", v)
	}
	if v := clampF(150, 0, 100); v != 100 {
		t.Errorf("expected 100, got %f", v)
	}
	if v := clampF(50, 0, 100); v != 50 {
		t.Errorf("expected 50, got %f", v)
	}
}
