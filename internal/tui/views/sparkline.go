package views

// sparklineBlocks maps values 0–7 to Unicode block elements for sparkline rendering.
var sparklineBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline renders a sparkline from a slice of float64 values.
// width is the maximum number of characters; data is sampled to fit.
// Returns empty string if data is empty.
func RenderSparkline(data []float64, width int) string {
	n := len(data)
	if n == 0 || width <= 0 {
		return ""
	}

	// Use the last `width` data points (or all if fewer).
	start := 0
	if n > width {
		start = n - width
	}
	visible := data[start:]

	minVal, maxVal := visible[0], visible[0]
	for _, v := range visible {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	rng := maxVal - minVal
	// Always produce exactly `width` runes: pad with ▁ on the left if fewer data points.
	result := make([]rune, width)
	pad := width - len(visible)
	for i := 0; i < pad; i++ {
		result[i] = sparklineBlocks[0]
	}
	for i, v := range visible {
		if rng == 0 {
			result[pad+i] = sparklineBlocks[0]
		} else {
			idx := int((v - minVal) / rng * 7)
			if idx > 7 {
				idx = 7
			}
			result[pad+i] = sparklineBlocks[idx]
		}
	}
	return string(result)
}
