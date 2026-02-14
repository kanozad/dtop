package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// MeterOpts controls meter rendering.
type MeterOpts struct {
	FillStyle  lipgloss.Style
	EmptyStyle lipgloss.Style
	ASCII      bool
}

// RenderMeter draws a single-line percentage bar:
//
//	label [████████░░░░░░] 56.2%
//
// Width is the total available character width for the entire line.
func RenderMeter(label string, pct float64, width int, opts MeterOpts) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	pctStr := fmt.Sprintf("%5.1f%%", pct)

	// Calculate bar width: total - label - brackets - spaces - pctStr.
	// Format: "label [bar] pct%"
	labelW := runewidth.StringWidth(label)
	overhead := labelW + 1 + 1 + 1 + 1 + len(pctStr)
	// label + " " + "[" + "]" + " " + pctStr
	barWidth := width - overhead
	if barWidth < 3 {
		// Fall back to compact mode without bar.
		compact := fmt.Sprintf("%s %s", label, pctStr)
		return runewidth.Truncate(compact, width, "...")
	}

	filled := int(pct / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	fillGlyph, emptyGlyph := "█", "░"
	if opts.ASCII {
		fillGlyph, emptyGlyph = "=", "-"
	}
	fillStr := opts.FillStyle.Render(strings.Repeat(fillGlyph, filled))
	emptyStr := opts.EmptyStyle.Render(strings.Repeat(emptyGlyph, empty))

	return fmt.Sprintf("%s [%s%s] %s", label, fillStr, emptyStr, pctStr)
}

// RenderMiniMeter draws a compact meter without brackets, suitable for
// per-core CPU bars in tight layouts:
//
//	cpu0 ████░░ 45.2%
func RenderMiniMeter(label string, pct float64, width int, opts MeterOpts) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	pctStr := fmt.Sprintf("%5.1f%%", pct)
	labelW := runewidth.StringWidth(label)
	// Format: "label bar pct%"
	overhead := labelW + 1 + 1 + len(pctStr)
	barWidth := width - overhead
	if barWidth < 2 {
		compact := fmt.Sprintf("%s %s", label, pctStr)
		return runewidth.Truncate(compact, width, "...")
	}

	filled := int(pct / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	fillGlyph, emptyGlyph := "█", "░"
	if opts.ASCII {
		fillGlyph, emptyGlyph = "=", "-"
	}
	fillStr := opts.FillStyle.Render(strings.Repeat(fillGlyph, filled))
	emptyStr := opts.EmptyStyle.Render(strings.Repeat(emptyGlyph, empty))

	return fmt.Sprintf("%s %s%s %s", label, fillStr, emptyStr, pctStr)
}
