package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/internal/version"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// renderLayout composes header + body + status bar into a full-screen layout.
func renderLayout(header, body, statusBar string, _ int, height int) string {
	headerHeight := lipgloss.Height(header)
	statusBarHeight := lipgloss.Height(statusBar)
	bodyHeight := height - headerHeight - statusBarHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	// Pad body to fill available space so status bar stays at the bottom.
	bodyLines := lipgloss.Height(body)
	if bodyLines < bodyHeight {
		body += strings.Repeat("\n", bodyHeight-bodyLines)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

// RecordingInfo holds recording state for header display.
type RecordingInfo struct {
	Active   bool
	Duration time.Duration
	Label    string // e.g. "(3/4)" for server, "(mic+in)" for client
	Size     uint64 // current recording size in bytes
}

// renderHeader renders the fixed top line:
// EchoWarp  SERVER  v1.0.0    🟢 Excellent  ● REC 05:23  ●  Connected  01:23:45
func renderHeader(cfg config.Config, state string, duration time.Duration, reconnectCount int, aecEnabled bool, width int, rec ...RecordingInfo) string {
	// Left side: brand + mode + version
	modeStyle := styles.ModeClient
	if cfg.Mode == config.ModeServer {
		modeStyle = styles.ModeServer
	}
	simdTag := "Pure Go"
	if lvl := audio.SIMDLevel(); lvl != "none" {
		simdTag = lvl
	}

	left := lipgloss.JoinHorizontal(lipgloss.Left,
		styles.AppTitle.Render("EchoWarp"),
		"  ",
		modeStyle.Render(strings.ToUpper(string(cfg.Mode))),
		"  ",
		styles.Version.Render("v"+version.Version+" ("+simdTag+")"),
	)

	// Recording indicator (between left and right)
	var recIndicator string
	if len(rec) > 0 && rec[0].Active {
		recDur := formatDuration(rec[0].Duration)
		recText := "● REC " + recDur
		if rec[0].Size > 0 {
			recText += " " + formatRecSize(rec[0].Size)
		}
		if rec[0].Label != "" {
			recText += " " + rec[0].Label
		}
		recIndicator = "  " + styles.RecordingIndicator.Render(recText)
		left += recIndicator
	}

	// AEC badge
	if aecEnabled {
		left += "  " + styles.StatLabel.Render(i18n.T("streaming_label_echo_cancel"))
	}

	// Right side: state indicator + state text + duration
	stateStyle := stateColor(state)
	indicator := getStateIndicator(state)
	stateText := stateStyle.Render(fmt.Sprintf("%s %s", indicator, capitalizeFirst(state)))

	if reconnectCount > 0 && strings.EqualFold(state, "connected") {
		stateText += styles.StatLabel.Render(i18n.Tf("streaming_reconnected_count", reconnectCount))
	}

	dur := formatDuration(duration)
	right := stateText + "  " + dur

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacing := width - leftWidth - rightWidth
	if spacing < 1 {
		spacing = 1
	}

	return left + strings.Repeat(" ", spacing) + right
}

// renderStatusBar renders the fixed bottom line:
// 🔒 TLS │ Opus 48kHz stereo │ :4415 │ ctrl+q: quit
func renderStatusBar(cfg config.Config, tlsEnabled bool, tlsSelfSigned bool, helpKeys string, width int) string {
	var parts []string

	// TLS indicator
	if tlsEnabled {
		label := i18n.T("layout_tls")
		if tlsSelfSigned {
			label = i18n.T("layout_tls_self_signed")
		}
		parts = append(parts, styles.TLSEnabled.Render(label))
	} else {
		parts = append(parts, styles.TLSEncrypted.Render(i18n.T("layout_aes_dtls")))
	}

	// Codec info
	channelStr := views.FormatChannels(cfg.Channels)
	parts = append(parts, fmt.Sprintf("Opus %s %s %s", views.FormatSampleRate(cfg.SampleRate), views.FormatBitrate(cfg.OpusBitrate), channelStr))

	// Address/port
	if cfg.Mode == config.ModeServer {
		parts = append(parts, fmt.Sprintf(":%d", cfg.Port))
	} else {
		parts = append(parts, fmt.Sprintf("%s:%d", cfg.Address, cfg.Port))
	}

	// Help keys
	parts = append(parts, helpKeys)

	line := styles.StatusBarStyle.Render(strings.Join(parts, " │ "))

	// Pad to full width
	lineWidth := lipgloss.Width(line)
	if lineWidth < width {
		line += strings.Repeat(" ", width-lineWidth)
	}

	return line
}

// stateColor returns the style for a connection state string.
func stateColor(state string) lipgloss.Style {
	switch strings.ToLower(state) {
	case "connected":
		return styles.StateConnected
	case "disconnected", "failed", "closed":
		return styles.StateDisconnected
	case "connecting", "new":
		return styles.StateConnecting
	default:
		return styles.StateDefault
	}
}

// getStateIndicator returns a Unicode indicator for the state.
func getStateIndicator(state string) string {
	switch strings.ToLower(state) {
	case "connected":
		return "●"
	case "disconnected", "failed", "closed":
		return "○"
	case "connecting", "new":
		return "◉"
	default:
		return "○"
	}
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatDuration formats a duration as HH:MM:SS or MM:SS.
func formatRecSize(bytes uint64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
