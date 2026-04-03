package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// SessionSummary contains accumulated statistics for the exit summary screen.
type SessionSummary struct {
	Duration       time.Duration
	TotalSent      uint64
	TotalRecv      uint64
	AvgBitrate     string
	ReconnectCount int
	AvgJitter      float64
	AvgRTT         float64
	TotalLoss      uint32
	LogFile        string
}

// SummaryView renders the session summary shown on exit instead of "Goodbye!".
func SummaryView(s SessionSummary) string {
	title := styles.AppTitle.Render("Session summary")
	sep := styles.Separator.Render(strings.Repeat("─", 30))

	lines := []string{
		title,
		sep,
		summaryLine("Duration:", FormatDuration(s.Duration)),
		summaryLine("Traffic:", fmt.Sprintf("↑ %s  ↓ %s", FormatBytes(s.TotalSent), FormatBytes(s.TotalRecv))),
		summaryLine("Avg bitrate:", s.AvgBitrate),
		summaryLine("Reconnects:", fmt.Sprintf("%d", s.ReconnectCount)),
		summaryLine("Avg jitter:", fmt.Sprintf("%.1f ms", s.AvgJitter)),
		summaryLine("Avg RTT:", fmt.Sprintf("%.1f ms", s.AvgRTT)),
		summaryLine("Packet loss:", fmt.Sprintf("%d total", s.TotalLoss)),
	}

	if s.LogFile != "" {
		lines = append(lines, summaryLine("Log file:", s.LogFile))
	}

	return strings.Join(lines, "\n") + "\n"
}

func summaryLine(label, value string) string {
	return fmt.Sprintf("  %s %s",
		styles.StatLabel.Width(14).Render(label),
		styles.StatValueGood.Render(value),
	)
}
