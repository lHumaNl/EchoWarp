package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// QualityLevel represents the assessed connection quality.
type QualityLevel int

const (
	QualityUnknown   QualityLevel = -1
	QualityExcellent QualityLevel = 0
	QualityGood      QualityLevel = 1
	QualityFair      QualityLevel = 2
	QualityPoor      QualityLevel = 3
)

// String returns the human-readable quality level name.
func (q QualityLevel) String() string {
	switch q {
	case QualityExcellent:
		return "Excellent"
	case QualityGood:
		return "Good"
	case QualityFair:
		return "Fair"
	case QualityPoor:
		return "Poor"
	default:
		return ""
	}
}

// Emoji returns the colored circle emoji for the quality level.
func (q QualityLevel) Emoji() string {
	switch q {
	case QualityExcellent:
		return "🟢"
	case QualityGood:
		return "🟡"
	case QualityFair:
		return "🟠"
	case QualityPoor:
		return "🔴"
	default:
		return ""
	}
}

// Style returns the lipgloss style appropriate for this quality level.
func (q QualityLevel) Style() lipgloss.Style {
	switch q {
	case QualityExcellent:
		return styles.StatValueGood
	case QualityGood:
		return styles.StatValueGood
	case QualityFair:
		return styles.StatValueWarn
	case QualityPoor:
		return styles.StatValueError
	default:
		return styles.StatLabel
	}
}

// CalculateQuality computes quality level from connection stats.
// rttMs: round-trip time in milliseconds, jitterMs: jitter in milliseconds,
// packetsLost: absolute packet loss count.
func CalculateQuality(rttMs, jitterMs float64, packetsLost uint32) QualityLevel {
	// Poor: any metric in critical range
	if rttMs >= 200 || jitterMs >= 100 || packetsLost >= 10 {
		return QualityPoor
	}
	// Fair
	if rttMs >= 100 || jitterMs >= 50 || packetsLost >= 5 {
		return QualityFair
	}
	// Good
	if rttMs >= 50 || jitterMs >= 30 || packetsLost >= 3 {
		return QualityGood
	}
	return QualityExcellent
}

// RenderQualityBadge returns a compact quality badge for the header: "🟢 Excellent"
func RenderQualityBadge(q QualityLevel) string {
	if q == QualityUnknown {
		return ""
	}
	return fmt.Sprintf("%s %s", q.Emoji(), q.Style().Render(q.String()))
}

// thresholdBar renders a horizontal bar showing how close a value is to the "poor" threshold.
// barWidth is the number of chars for the bar body (excluding brackets).
// The bar fills proportionally: 0 → empty, maxVal → full.
// Color follows green/yellow/red based on warn/error thresholds.
func thresholdBar(value, warnThresh, errorThresh, maxVal float64, barWidth int) string {
	if maxVal <= 0 {
		maxVal = 1
	}
	ratio := value / maxVal
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0 {
		ratio = 0
	}

	filled := int(ratio * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	// Choose color based on value vs thresholds
	var style lipgloss.Style
	switch {
	case value >= errorThresh:
		style = styles.StatValueError
	case value >= warnThresh:
		style = styles.StatValueWarn
	default:
		style = styles.StatValueGood
	}

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", barWidth-filled)

	return style.Render(filledStr) + styles.StatLabel.Render(emptyStr)
}

// JitterBar renders a threshold bar for jitter (0–100ms range).
func JitterBar(jitterMs float64, width int) string {
	return thresholdBar(jitterMs, 30, 100, 100, width)
}

// RTTBar renders a threshold bar for round-trip time (0–200ms range).
func RTTBar(rttMs float64, width int) string {
	return thresholdBar(rttMs, 50, 150, 200, width)
}

// LossBar renders a threshold bar for packet loss count (0–10 range).
func LossBar(packetsLost uint32, width int) string {
	return thresholdBar(float64(packetsLost), 3, 10, 10, width)
}
