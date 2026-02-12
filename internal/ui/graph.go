package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GraphOpts controls graph rendering behavior.
type GraphOpts struct {
	// Min and Max define the data range. Data is clamped to [Min, Max].
	Min float64
	Max float64
	// Style is applied to the rendered braille characters.
	Style lipgloss.Style
	// Fill renders a filled area graph when true; otherwise a line graph.
	Fill bool
}

// RenderGraph draws a braille sparkline from a time-series data slice.
//
// Width is the number of character columns (each column encodes 2 data points
// horizontally via braille dots). Height is the number of text rows (each row
// encodes 4 vertical levels via braille dots).
//
// The data slice should already be sized to width (one data point per column).
// If len(data) < width, the graph is right-aligned with empty columns on the
// left. If len(data) > width, only the last width values are used.
func RenderGraph(data []float64, width, height int, opts GraphOpts) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if opts.Max <= opts.Min {
		opts.Max = opts.Min + 1
	}

	// Ensure we work with at most width data points (take trailing).
	d := data
	if len(d) > width {
		d = d[len(d)-width:]
	}

	// Total vertical dot resolution.
	vRes := height * 4

	// Quantize each value to a row in [0, vRes].
	levels := make([]int, width)
	for i := 0; i < width; i++ {
		if i < width-len(d) {
			levels[i] = 0
			continue
		}
		v := d[i-(width-len(d))]
		v = clampF(v, opts.Min, opts.Max)
		frac := (v - opts.Min) / (opts.Max - opts.Min)
		levels[i] = int(frac * float64(vRes))
		if levels[i] > vRes {
			levels[i] = vRes
		}
	}

	// Build the grid row by row (top row = highest values).
	rows := make([]string, height)
	for row := 0; row < height; row++ {
		var sb strings.Builder
		// The vertical range this text row covers in dot units.
		rowTop := (height - row) * 4        // exclusive top
		rowBottom := (height - row - 1) * 4 // inclusive bottom

		for col := 0; col < width; col++ {
			lvl := levels[col]
			var dots [4]bool // dots[0] = bottom of cell, dots[3] = top of cell
			for dot := 0; dot < 4; dot++ {
				dotLevel := rowBottom + dot + 1 // the level this dot represents (1-indexed within row)
				if opts.Fill {
					dots[dot] = lvl >= dotLevel
				} else {
					// Line mode: light up the dot closest to the level.
					dots[dot] = lvl >= dotLevel && lvl < dotLevel+1
					// Also fill bottom if level exceeds row top to keep continuity.
					if lvl >= rowTop && dot == 3 {
						dots[dot] = true
					}
				}
			}
			sb.WriteRune(brailleFromDots(dots))
		}
		rows[row] = opts.Style.Render(sb.String())
	}
	return strings.Join(rows, "\n")
}

// brailleFromDots converts a column of 4 vertical dots into a single braille
// character using the left column of the braille cell (dots 1,2,3,7 in Unicode
// numbering).
//
// Braille dot positions (Unicode numbering):
//
//	1 4
//	2 5
//	3 6
//	7 8
//
// We use only the left column (dots 1,2,3,7).
// dots[0] = bottom (dot 7), dots[1] = row 3, dots[2] = row 2, dots[3] = top (dot 1).
func brailleFromDots(dots [4]bool) rune {
	var offset rune
	if dots[3] { // top    -> dot 1
		offset |= 0x01
	}
	if dots[2] { // row 2  -> dot 2
		offset |= 0x02
	}
	if dots[1] { // row 3  -> dot 3
		offset |= 0x04
	}
	if dots[0] { // bottom -> dot 7
		offset |= 0x40
	}
	return 0x2800 + offset
}

func clampF(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
