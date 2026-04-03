package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// vuChannelLabel returns a label for a VU channel.
func vuChannelLabel(channels, idx int) string {
	if channels == 1 {
		return "M"
	}
	labels := []string{"L", "R", "C", "LFE", "SL", "SR"}
	if idx < len(labels) {
		return labels[idx]
	}
	return fmt.Sprintf("%d", idx+1)
}

// spectrumBlocks maps normalized values to vertical block characters (bottom-up).
var spectrumBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// vuSepStyle is the cached style for the VU separator.
var vuSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// cachedColorStyles pre-builds styles for each spectrumColors entry.
var cachedColorStyles []lipgloss.Style

func init() {
	cachedColorStyles = make([]lipgloss.Style, len(spectrumColors))
	for i, c := range spectrumColors {
		cachedColorStyles[i] = lipgloss.NewStyle().Foreground(c)
	}
}

// spectrumColors provides gradient colors from green (low) to red (high).
var spectrumColors = []lipgloss.Color{
	lipgloss.Color("22"),  // dark green
	lipgloss.Color("34"),  // green
	lipgloss.Color("82"),  // light green
	lipgloss.Color("154"), // yellow-green
	lipgloss.Color("226"), // yellow
	lipgloss.Color("214"), // orange
	lipgloss.Color("202"), // dark orange
	lipgloss.Color("196"), // red
}

// RenderSpectrumWithVU renders spectrum bars on the left and vertical VU meters on the right.
// If only spectrum is provided, renders spectrum only. If only VU, renders VU only.
// height: number of rows for the visualization.
func RenderSpectrumWithVU(bands []float64, vuLevels []float64, height, maxWidth int) string {
	if len(bands) == 0 && len(vuLevels) == 0 {
		return ""
	}
	if height <= 0 {
		height = 5
	}

	// VU section width: each channel = 3 chars (bar + space + bar) + labels row
	// Format: "  ┃  █ █" for stereo = gap(2) + separator(1) + gap(2) + bars(ch*2-1)
	vuWidth := 0
	if len(vuLevels) > 0 {
		vuWidth = 5 + len(vuLevels)*2 // "  ┃  " + "X " per channel
	}

	specWidth := maxWidth - vuWidth
	if specWidth < 4 {
		specWidth = 4
	}
	// Clamp to actual band count so VU stays close to spectrum
	actualSpecWidth := len(bands) * 2 // each band = 1 char + 1 space
	if actualSpecWidth > 0 && actualSpecWidth < specWidth {
		specWidth = actualSpecWidth
	}

	// Build spectrum rows
	specRows := renderSpectrumRows(bands, height, specWidth)

	// Build VU rows (1 row shorter to leave room for labels on last row)
	vuHeight := height - 1
	if vuHeight < 1 {
		vuHeight = 1
	}
	vuRows := renderVURows(vuLevels, vuHeight)

	// Build VU label string
	var vuLabelStr string
	if len(vuLevels) > 0 {
		var lb strings.Builder
		for i := range vuLevels {
			if i > 0 {
				lb.WriteRune(' ')
			}
			lb.WriteString(vuChannelLabel(len(vuLevels), i))
		}
		vuLabelStr = lb.String()
	}

	// Combine — spectrum rows + VU rows side by side, same height
	lines := make([]string, height)
	for row := 0; row < height; row++ {
		var sb strings.Builder
		sb.Grow(specWidth + vuWidth)

		// Spectrum part
		if row < len(specRows) {
			sb.WriteString(specRows[row])
			specLen := len(bands) * 2 // each band = 1 char + 1 space
			for i := specLen; i < specWidth; i++ {
				sb.WriteRune(' ')
			}
		} else {
			for i := 0; i < specWidth; i++ {
				sb.WriteRune(' ')
			}
		}

		// VU part
		if len(vuLevels) > 0 {
			sep := vuSepStyle.Render("│")
			sb.WriteString("  " + sep + "  ")
			if row < vuHeight && row < len(vuRows) {
				sb.WriteString(vuRows[row])
			} else if row == vuHeight {
				// Label row
				sb.WriteString(vuLabelStr)
			}
		}

		lines[row] = sb.String()
	}

	return strings.Join(lines, "\n")
}

// renderBar renders a single bar cell for the given value and row threshold.
func renderBar(sb *strings.Builder, val, threshold float64, height int) {
	if val >= threshold {
		colorIdx := int(val * float64(len(cachedColorStyles)-1))
		if colorIdx >= len(cachedColorStyles) {
			colorIdx = len(cachedColorStyles) - 1
		}
		sb.WriteString(cachedColorStyles[colorIdx].Render("█"))
	} else if val >= threshold-1.0/float64(height) {
		frac := (val - (threshold - 1.0/float64(height))) * float64(height)
		idx := int(frac * float64(len(spectrumBlocks)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(spectrumBlocks) {
			idx = len(spectrumBlocks) - 1
		}
		colorIdx := int(val * float64(len(cachedColorStyles)-1))
		if colorIdx >= len(cachedColorStyles) {
			colorIdx = len(cachedColorStyles) - 1
		}
		sb.WriteString(cachedColorStyles[colorIdx].Render(string(spectrumBlocks[idx])))
	} else {
		_ = sb.WriteByte(' ') //nolint:errcheck
	}
}

// renderSpectrumRows builds spectrum bar rows (top to bottom).
func renderSpectrumRows(bands []float64, height, maxWidth int) []string {
	n := len(bands)
	if n == 0 {
		return nil
	}

	maxBands := maxWidth / 2
	if n > maxBands {
		n = maxBands
	}

	rows := make([]string, height)
	for row := 0; row < height; row++ {
		threshold := float64(height-row) / float64(height)
		var sb strings.Builder
		for b := 0; b < n; b++ {
			renderBar(&sb, bands[b], threshold, height)
			sb.WriteRune(' ')
		}
		rows[row] = sb.String()
	}
	return rows
}

// renderVURows builds vertical VU meter rows (top to bottom).
func renderVURows(levels []float64, height int) []string {
	if len(levels) == 0 {
		return nil
	}

	rows := make([]string, height)
	for row := 0; row < height; row++ {
		threshold := float64(height-row) / float64(height)
		var sb strings.Builder
		for i, val := range levels {
			if i > 0 {
				sb.WriteRune(' ')
			}
			renderBar(&sb, val, threshold, height)
		}
		rows[row] = sb.String()
	}
	return rows
}

// RenderDualSpectrumWithVU renders two spectra side by side (capture + playback) with two VU sets.
// Labels: "capture" / "playback" under spectra, "▲ ▲" / "▼ ▼" under VU columns.
// Falls back to single playback spectrum when maxWidth < 60.
func RenderDualSpectrumWithVU(captureBands, captureVU, playbackBands, playbackVU []float64, height, maxWidth int) string {
	hasCap := len(captureBands) > 0 || len(captureVU) > 0
	hasPlay := len(playbackBands) > 0 || len(playbackVU) > 0
	if !hasCap && !hasPlay {
		return ""
	}
	if height <= 0 {
		height = 5
	}

	// Narrow fallback: show only playback as single spectrum
	if maxWidth < 60 {
		if hasPlay {
			return RenderSpectrumWithVU(playbackBands, playbackVU, height, maxWidth)
		}
		return RenderSpectrumWithVU(captureBands, captureVU, height, maxWidth)
	}

	// Reserve last row for labels
	vizHeight := height - 1
	if vizHeight < 1 {
		vizHeight = 1
	}
	vuBarHeight := vizHeight - 1
	if vuBarHeight < 1 {
		vuBarHeight = 1
	}

	// Compute widths for each spectrum section
	// VU: "  ┃  " (5) + channels*2 per set, two sets with 1 space gap
	capVUWidth := 0
	if len(captureVU) > 0 {
		capVUWidth = len(captureVU) * 2
	}
	playVUWidth := 0
	if len(playbackVU) > 0 {
		playVUWidth = len(playbackVU) * 2
	}
	totalVUWidth := 0
	if capVUWidth+playVUWidth > 0 {
		totalVUWidth = 5 + capVUWidth // "  ┃  " + capture VU
		if playVUWidth > 0 {
			totalVUWidth += 1 + playVUWidth // gap + playback VU
		}
	}

	// Split remaining width between two spectra with 5-char gap
	specGap := 5
	availSpec := maxWidth - totalVUWidth - specGap
	if availSpec < 8 {
		availSpec = 8
	}
	halfSpec := availSpec / 2

	// Clamp to actual band counts
	capSpecW := halfSpec
	if actual := len(captureBands) * 2; actual > 0 && actual < capSpecW {
		capSpecW = actual
	}
	playSpecW := halfSpec
	if actual := len(playbackBands) * 2; actual > 0 && actual < playSpecW {
		playSpecW = actual
	}

	// Build spectrum rows
	capSpecRows := renderSpectrumRows(captureBands, vizHeight, capSpecW)
	playSpecRows := renderSpectrumRows(playbackBands, vizHeight, playSpecW)

	// Build VU rows
	capVURows := renderVURows(captureVU, vuBarHeight)
	playVURows := renderVURows(playbackVU, vuBarHeight)

	// Build VU labels
	capVULabel := buildVULabel(captureVU)
	playVULabel := buildVULabel(playbackVU)

	// Combine rows
	lines := make([]string, height)
	sep := vuSepStyle.Render("│")

	for row := 0; row < height; row++ {
		var sb strings.Builder

		if row < vizHeight {
			// Capture spectrum
			if row < len(capSpecRows) {
				sb.WriteString(capSpecRows[row])
			}
			padToVisual(&sb, capSpecW)

			// Gap between spectra
			sb.WriteString(strings.Repeat(" ", specGap))

			// Playback spectrum
			if row < len(playSpecRows) {
				sb.WriteString(playSpecRows[row])
			}
			padToVisual(&sb, capSpecW+specGap+playSpecW)

			// VU section
			if totalVUWidth > 0 {
				sb.WriteString("  " + sep + "  ")
				vuRow := row
				if vuRow < vuBarHeight {
					if vuRow < len(capVURows) {
						sb.WriteString(capVURows[vuRow])
					}
					if playVUWidth > 0 {
						sb.WriteRune(' ')
						if vuRow < len(playVURows) {
							sb.WriteString(playVURows[vuRow])
						}
					}
				} else {
					// VU label row
					sb.WriteString(capVULabel)
					if playVUWidth > 0 {
						sb.WriteRune(' ')
						sb.WriteString(playVULabel)
					}
				}
			}
		} else {
			// Label row for spectra and VU arrows
			capLabel := "capture"
			playLabel := "playback"
			capPad := (capSpecW - len(capLabel)) / 2
			if capPad < 0 {
				capPad = 0
			}
			playPad := (playSpecW - len(playLabel)) / 2
			if playPad < 0 {
				playPad = 0
			}
			sb.WriteString(strings.Repeat(" ", capPad))
			sb.WriteString(capLabel)
			remaining := capSpecW - capPad - len(capLabel)
			if remaining > 0 {
				sb.WriteString(strings.Repeat(" ", remaining))
			}

			sb.WriteString(strings.Repeat(" ", specGap))

			sb.WriteString(strings.Repeat(" ", playPad))
			sb.WriteString(playLabel)
			remaining = playSpecW - playPad - len(playLabel)
			if remaining > 0 {
				sb.WriteString(strings.Repeat(" ", remaining))
			}

			// VU arrow labels
			if totalVUWidth > 0 {
				sb.WriteString("  " + sep + "  ")
				sb.WriteString(buildVUArrows(captureVU, "▲"))
				if playVUWidth > 0 {
					sb.WriteRune(' ')
					sb.WriteString(buildVUArrows(playbackVU, "▼"))
				}
			}
		}

		lines[row] = sb.String()
	}

	return strings.Join(lines, "\n")
}

// padToVisual pads sb with spaces so the *visual* width of the content written so far
// reaches targetVisual columns. It uses lipgloss.Width for accurate measurement even
// when ANSI escape sequences are present.
func padToVisual(sb *strings.Builder, targetVisual int) {
	cur := lipgloss.Width(sb.String())
	if cur < targetVisual {
		sb.WriteString(strings.Repeat(" ", targetVisual-cur))
	}
}

func buildVULabel(levels []float64) string {
	if len(levels) == 0 {
		return ""
	}
	var sb strings.Builder
	for i := range levels {
		if i > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(vuChannelLabel(len(levels), i))
	}
	return sb.String()
}

func buildVUArrows(levels []float64, arrow string) string {
	if len(levels) == 0 {
		return ""
	}
	var sb strings.Builder
	for i := range levels {
		if i > 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(arrow)
	}
	return sb.String()
}

// RenderSpectrum renders a frequency spectrum as vertical bars (legacy, kept for compatibility).
func RenderSpectrum(bands []float64, height, maxWidth int) string {
	return RenderSpectrumWithVU(bands, nil, height, maxWidth)
}
