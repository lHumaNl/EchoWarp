package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/internal/i18n"
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
	title := styles.AppTitle.Render(i18n.T("summary_title"))
	sep := styles.Separator.Render(strings.Repeat("─", 30))

	lines := []string{
		title,
		sep,
		summaryLine(i18n.T("summary_label_duration"), FormatDuration(s.Duration)),
		summaryLine(i18n.T("summary_label_traffic"), fmt.Sprintf("↑ %s  ↓ %s", FormatBytes(s.TotalSent), FormatBytes(s.TotalRecv))),
		summaryLine(i18n.T("summary_label_avg_bitrate"), s.AvgBitrate),
		summaryLine(i18n.T("summary_label_reconnects"), fmt.Sprintf("%d", s.ReconnectCount)),
		summaryLine(i18n.T("summary_label_avg_jitter"), fmt.Sprintf("%.1f ms", s.AvgJitter)),
		summaryLine(i18n.T("summary_label_avg_rtt"), fmt.Sprintf("%.1f ms", s.AvgRTT)),
		summaryLine(i18n.T("summary_label_packet_loss"), i18n.Tf("summary_packet_loss_value", s.TotalLoss)),
	}

	if s.LogFile != "" {
		lines = append(lines, summaryLine(i18n.T("summary_label_log_file"), s.LogFile))
	}

	return strings.Join(lines, "\n") + "\n"
}

func summaryLine(label, value string) string {
	return fmt.Sprintf("  %s %s",
		styles.StatLabel.Width(14).Render(label),
		styles.StatValueGood.Render(value),
	)
}
