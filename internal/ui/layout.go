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
