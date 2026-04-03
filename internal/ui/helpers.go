package ui

import "github.com/mattn/go-runewidth"

// PushAndClamp appends value to hist, keeping at most width entries.
func PushAndClamp(hist []float64, value float64, width int) []float64 {
	if width <= 0 {
		return hist
	}
	hist = append(hist, value)
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
	return hist
}

// ResizeHistory trims or pads hist to exactly width entries.
// When padding, the last value in hist is repeated.
func ResizeHistory(hist []float64, width int) []float64 {
	if width <= 0 {
		return hist
	}
	// trim
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
	// pad with last value if growing
	if len(hist) < width {
		padVal := 0.0
		if len(hist) > 0 {
			padVal = hist[len(hist)-1]
		}
		for len(hist) < width {
			hist = append(hist, padVal)
		}
	}
	return hist
}

// ContentWidth returns the usable inner width after subtracting box border
// and padding (default 1 left + 1 right padding + 1 left + 1 right border = 4).
func ContentWidth(totalWidth int) int {
	w := totalWidth - 4
	if w < 1 {
		return 1
	}
	return w
}

// ContentHeight returns the usable body height given the inner content rows
// passed to View(). It subtracts 1 for the title row that RenderBox prepends.
func ContentHeight(height int) int {
	h := height - 1
	if h < 1 {
		return 1
	}
	return h
}

// Truncate shortens s to width using an ellipsis if needed.
func Truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return runewidth.Truncate(s, width, "...")
}
