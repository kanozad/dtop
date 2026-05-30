package ui

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

// SizeHint describes the vertical sizing preferences for a plugin.
type SizeHint struct {
	MinH   int // absolute minimum content rows; below this the plugin is hidden
	PrefH  int // preferred/ideal content rows (0 = no preference)
	MaxH   int // hard cap on content rows (0 = unlimited)
	Weight int // relative priority when distributing surplus (0 = never grows)
}

// DefaultSizeHint returns the fallback hint used for plugins that do not
// implement SizeHinter. It matches the legacy equal-split behaviour.
func DefaultSizeHint() SizeHint {
	return SizeHint{MinH: 3, PrefH: 0, MaxH: 0, Weight: 1}
}

// AllocateHeights distributes totalHeight across plugins using their SizeHints.
//
// Algorithm:
//  1. Give each plugin its MinH.  If the budget is too small, do a proportional
//     undersized split (same degraded behaviour as before).
//  2. Distribute surplus proportionally by Weight, capping at MaxH.
//  3. When a plugin hits its cap, redistribute its unused portion to uncapped
//     plugins until no surplus remains or every plugin is capped.
func AllocateHeights(hints []SizeHint, totalHeight int) []int {
	n := len(hints)
	if n == 0 || totalHeight <= 0 {
		return nil
	}

	// Normalise hints.
	for i := range hints {
		if hints[i].MinH < 1 {
			hints[i].MinH = 1
		}
		if hints[i].Weight < 0 {
			hints[i].Weight = 0
		}
	}

	// Sum minimums.
	minSum := 0
	for _, h := range hints {
		minSum += h.MinH
	}

	// Under-budget: proportional degraded allocation.
	if totalHeight < minSum {
		alloc := make([]int, n)
		remaining := totalHeight
		for i := 0; i < n; i++ {
			share := totalHeight * hints[i].MinH / minSum
			if share < 1 && remaining > 0 {
				share = 1
			}
			if share > remaining {
				share = remaining
			}
			alloc[i] = share
			remaining -= share
		}
		// Any remainder to the last item
		if remaining > 0 && n > 0 {
			alloc[n-1] += remaining
		}
		return alloc
	}

	// Start everyone at their minimum.
	alloc := make([]int, n)
	for i, h := range hints {
		alloc[i] = h.MinH
	}

	surplus := totalHeight - minSum
	capped := make([]bool, n)

	for surplus > 0 {
		totalWeight := 0
		for i, h := range hints {
			if !capped[i] && h.Weight > 0 {
				totalWeight += h.Weight
			}
		}
		if totalWeight == 0 {
			break
		}

		// Distribute surplus in one pass; track newly-capped plugins.
		newCap := false
		remaining := surplus
		given := 0
		for i, h := range hints {
			if capped[i] || h.Weight == 0 {
				continue
			}
			share := remaining * h.Weight / totalWeight
			if share < 1 {
				share = 1
			}
			if share > surplus-given {
				share = surplus - given
			}
			if share <= 0 {
				break
			}

			maxH := capFor(h)
			if maxH > 0 && alloc[i]+share > maxH {
				share = maxH - alloc[i]
				if share < 0 {
					share = 0
				}
				capped[i] = true
				newCap = true
			}

			alloc[i] += share
			given += share
		}
		surplus -= given
		if !newCap {
			break
		}
	}

	// Distribute any integer-rounding remainder one row at a time.
	for i := range hints {
		if surplus <= 0 {
			break
		}
		if capped[i] || hints[i].Weight == 0 {
			continue
		}
		maxH := capFor(hints[i])
		if maxH > 0 && alloc[i] >= maxH {
			continue
		}
		alloc[i]++
		surplus--
	}

	return alloc
}

// capFor returns the effective upper bound for a hint. Zero means unlimited.
func capFor(h SizeHint) int {
	if h.MaxH > 0 {
		return h.MaxH
	}
	return 0
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
