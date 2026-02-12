package ui

// SplitHeights splits total height across count rows without over-allocation.
// If total is too small to satisfy min for each row, it distributes evenly
// and may return zero-height rows.
func SplitHeights(total, count, min int) []int {
	if count <= 0 || total <= 0 {
		return nil
	}
	if min < 1 {
		min = 1
	}

	heights := make([]int, count)
	if total < count*min {
		base := total / count
		rem := total % count
		for i := 0; i < count; i++ {
			h := base
			if i < rem {
				h++
			}
			heights[i] = h
		}
		return heights
	}

	remaining := total - count*min
	base := remaining / count
	rem := remaining % count
	for i := 0; i < count; i++ {
		h := min + base
		if i < rem {
			h++
		}
		heights[i] = h
	}
	return heights
}

// SplitWidths divides total width into numCols columns.
// Returns the width for each column.
func SplitWidths(total, numCols int) []int {
	if numCols <= 0 || total <= 0 {
		return nil
	}
	widths := make([]int, numCols)
	base := total / numCols
	rem := total % numCols
	for i := 0; i < numCols; i++ {
		w := base
		if i < rem {
			w++
		}
		widths[i] = w
	}
	return widths
}

// GridColumns distributes count items across numCols columns as evenly as
// possible, filling left columns first. Returns a slice of column sizes.
// For example, GridColumns(5, 2) returns [3, 2].
func GridColumns(count, numCols int) []int {
	if numCols <= 0 || count <= 0 {
		return nil
	}
	if numCols > count {
		numCols = count
	}
	cols := make([]int, numCols)
	base := count / numCols
	rem := count % numCols
	for i := 0; i < numCols; i++ {
		cols[i] = base
		if i < rem {
			cols[i]++
		}
	}
	return cols
}

// FlowColumns computes the number of columns needed to fit count items into a
// height budget with a minimum per-item height. It favors more columns when
// height is constrained to avoid vertical overflow.
func FlowColumns(count, height, minHeight int) int {
	if count <= 0 || height <= 0 {
		return 0
	}
	if minHeight < 1 {
		minHeight = 1
	}
	maxRows := height / minHeight
	if maxRows < 1 {
		maxRows = 1
	}
	cols := count / maxRows
	if count%maxRows != 0 {
		cols++
	}
	if cols < 1 {
		cols = 1
	}
	if cols > count {
		cols = count
	}
	return cols
}
