// Package views provides the streaming status UI screen for EchoWarp.
// It displays connection statistics, duration, bitrate, and quality metrics.
package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// AudioInfo holds codec/audio parameters for display in the streaming view.
type AudioInfo struct {
	Codec      string
	SampleRate uint32
	Channels   uint32
}

// DeviceDisplayState holds per-device info for rendering in the streaming view.
type DeviceDisplayState struct {
	ID           uint32
	Name         string
	Role         string  // "capture" or "playback"
	Volume       float64 // 0.0–2.0
	Muted        bool
	Selected     bool
	Disconnected bool
}

// StreamingParams contains all parameters needed to render the streaming screen body.
type StreamingParams struct {
	Stats           transport.ConnectionStats
	StartTime       time.Time
	Paused          bool
	Reverse         bool
	Duplex          bool
	Audio           AudioInfo
	Logs            []string
	LogsVisible     bool
	LogScrollOffset int
	JitterHistory   []float64
	RTTHistory      []float64
	DeviceName      string
	Width           int
	Height          int
	Devices         []DeviceDisplayState
	BitrateUp       float64 // instantaneous upload kbps (EMA-smoothed)
	BitrateDown     float64 // instantaneous download kbps (EMA-smoothed)
	GlobalMuted     bool
	SpectrumBands   []float64 // FFT frequency band magnitudes (0.0–1.0)
	VULevels        []float64 // Per-channel VU levels (0.0–1.0)

	// Dual spectrum fields for duplex mode (capture = outgoing, playback = incoming).
	CaptureSpectrumBands  []float64
	CaptureVULevels       []float64
	PlaybackSpectrumBands []float64
	PlaybackVULevels      []float64

	Quality       QualityLevel
	PacketLossPct float64 // packet loss percentage (0–100)
	ChatView      string  // pre-rendered chat panel (empty if not visible)
	Nickname      string  // chat nickname (shown in summary line)

	// Participant sidebar (client-side, multi-client sessions only).
	Participants []string // online participant nicknames (empty = no sidebar)
	MaxClients   int      // session capacity (0 = single-client, no sidebar)
	MyNickname   string   // own nickname for "(me)" suffix

	// SourcePaused indicates the remote audio source has paused its capture.
	SourcePaused bool
	// ServerMuted indicates we have muted the incoming server audio.
	ServerMuted bool

	// FocusedArea indicates which area has keyboard focus (0=ClientList, 1=Chat, 2=Logs).
	FocusedArea int
}

// StreamingView renders the streaming screen body (without header/status bar).
// Layout:
//
//	▸ server → client (normal)     Device Name
//	Remote: 192.168.1.5:51234      ▁▂▃▅▃▂▁
//	────────────────────────────────────────
//	     ↑ 2.3 MiB  142.5 kbps     Jitter  1.2 ms ▁▁▂▁
//	     ↓ 48 B                     RTT    12.4 ms ▁▁▁▁
//	                                Loss   0
//	────────────────────────────────────────
//	12:07:01 [INF] Client connected...
func StreamingView(p StreamingParams) string {
	var sections []string

	// Summary line: direction + device
	sections = append(sections, renderSummaryLine(p.Reverse, p.Duplex, p.Paused, p.SourcePaused, p.ServerMuted, p.DeviceName, p.Nickname, p.Width))

	// Duplex warning + latency estimation
	if p.Duplex {
		sections = append(sections, styles.StatValueWarn.Render("  ⚠ Duplex mode: headphones recommended to avoid echo"))
		// Latency estimation: codec (20ms × 2 encode+decode) + RTT/2 + jitter buffer
		codecMs := 40.0 // 20ms encode + 20ms decode
		jitterBuf := p.Stats.Jitter * 2
		if jitterBuf < 20 {
			jitterBuf = 20
		}
		oneWayMs := codecMs + p.Stats.RoundTrip/2 + jitterBuf
		latStyle := getValueStyle(oneWayMs, 80, 150, styles.StatValueGood, styles.StatValueWarn, styles.StatValueError)
		sections = append(sections, fmt.Sprintf("  Estimated latency: %s",
			latStyle.Render(fmt.Sprintf("%.0f ms (one-way)", oneWayMs))))
	}

	// Remote address
	if p.Stats.RemoteAddr != "" {
		sections = append(sections, renderRemoteLine(p.Stats.RemoteAddr, p.Width))
	}

	// Separator + Quality + Stats + spectrum/VU in two-column layout
	sections = append(sections, renderSep(p.Width))

	// Build stats content
	statsContent := renderStatsWithSpectrumDual(p.Stats, p.JitterHistory, p.RTTHistory, p.Width, p.BitrateUp, p.BitrateDown, p.Duplex, p.SpectrumBands, p.VULevels, p.PacketLossPct, p.Quality, p.CaptureSpectrumBands, p.CaptureVULevels, p.PlaybackSpectrumBands, p.PlaybackVULevels)

	// If participant sidebar is active, render two-column layout (sidebar | stats)
	showSidebar := len(p.Participants) > 0 && p.MaxClients > 1
	if showSidebar {
		sidebar := renderParticipantSidebar(p.Participants, p.MaxClients, p.MyNickname)
		sections = append(sections, joinSidebarAndStats(sidebar, statsContent))
	} else {
		sections = append(sections, statsContent)
	}

	// Device panel
	if len(p.Devices) > 0 {
		sections = append(sections, renderSep(p.Width), renderDevicePanel(p.Devices, p.GlobalMuted))
	}

	// Chat panel
	if p.ChatView != "" {
		sections = append(sections, renderSep(p.Width), p.ChatView)
	}

	// Logs — calculate remaining height to avoid pushing TUI off screen.
	if p.LogsVisible && len(p.Logs) > 0 && p.Height >= 20 {
		usedLines := countRenderedLines(sections)
		usedLines += 2 // separator + status bar
		remaining := p.Height - usedLines
		if remaining < 3 {
			remaining = 3
		}
		sections = append(sections, renderSep(p.Width))
		if p.FocusedArea == 2 { // FocusLogs
			sections = append(sections, styles.FocusedLabel.Render("Logs ▾"))
			remaining--
		}
		logPanel := renderLogPanelWithMax(p.Logs, p.LogScrollOffset, p.Width, remaining-1)
		sections = append(sections, logPanel)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderParticipantSidebar renders the left-column participant list for client-side multi-client sessions.
func renderParticipantSidebar(participants []string, maxClients int, myNickname string) string {
	lines := make([]string, 0, 2+len(participants))

	// Header: "Online (N/M)"
	header := fmt.Sprintf("Online (%d/%d)", len(participants), maxClients)
	lines = append(lines, styles.StatLabel.Render(header), styles.Separator.Render(strings.Repeat("─", leftColumnWidth)))

	// Participant list with bullets
	for _, nick := range participants {
		bullet := styles.ParticipantBullet.Render(" • ")
		entry := nick
		if nick == myNickname {
			entry += " (me)"
		}
		// Truncate if too wide
		maxNameW := leftColumnWidth - 3 // " • " prefix
		if runewidth.StringWidth(entry) > maxNameW {
			entry = runewidth.Truncate(entry, maxNameW-1, "") + "…"
		}
		lines = append(lines, bullet+entry)
	}

	return strings.Join(lines, "\n")
}

// joinSidebarAndStats combines the participant sidebar (left) with the stats panel (right) using a vertical separator.
func joinSidebarAndStats(sidebar, statsContent string) string {
	sidebarLines := strings.Split(sidebar, "\n")
	statsLines := strings.Split(statsContent, "\n")

	totalRows := len(sidebarLines)
	if len(statsLines) > totalRows {
		totalRows = len(statsLines)
	}

	sep := styles.ColumnSeparator.Render("│")
	combined := make([]string, totalRows)
	for i := 0; i < totalRows; i++ {
		left := ""
		if i < len(sidebarLines) {
			left = sidebarLines[i]
		}
		// Pad left to leftColumnWidth
		lw := lipgloss.Width(left)
		pad := leftColumnWidth - lw
		if pad < 0 {
			pad = 0
		}

		right := ""
		if i < len(statsLines) {
			right = statsLines[i]
		}

		combined[i] = left + strings.Repeat(" ", pad) + " " + sep + " " + right
	}

	return strings.Join(combined, "\n")
}

func renderSummaryLine(reverse, duplex, paused, sourcePaused, serverMuted bool, deviceName, nickname string, width int) string {
	var dirText string
	var style lipgloss.Style

	if paused {
		if duplex {
			dirText = "⇄ server ↔ client (duplex) [PAUSED]"
		} else if reverse {
			dirText = "▸ client → server (reverse) [PAUSED]"
		} else {
			dirText = "▸ server → client (normal) [PAUSED]"
		}
		style = styles.Paused
	} else if duplex {
		dirText = "⇄ server ↔ client (duplex)"
		style = styles.Direction
	} else if reverse {
		dirText = "▸ client → server (reverse)"
		style = styles.DirectionReverse
	} else {
		dirText = "▸ server → client (normal)"
		style = styles.Direction
	}

	if sourcePaused && !paused {
		dirText += " [SOURCE PAUSED]"
		style = styles.Paused
	}

	if serverMuted {
		dirText += " [MUTED]"
		style = styles.Paused
	}

	if nickname != "" {
		dirText += "  " + nickname
	}

	left := style.Render(dirText)
	if deviceName != "" {
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(deviceName)
		spacing := width - leftW - rightW
		if spacing < 2 {
			spacing = 2
		}
		return left + strings.Repeat(" ", spacing) + styles.StatLabel.Render(deviceName)
	}
	return left
}

func renderRemoteLine(remoteAddr string, _ int) string {
	return styles.StatLabel.Render("Remote: ") + styles.StatValueGood.Render(remoteAddr)
}

func renderSep(width int) string {
	w := width
	if w < 20 {
		w = 20
	}
	return styles.Separator.Render(strings.Repeat("─", w))
}

// thresholdBarWidth is the width of threshold bars in the stats panel.
const thresholdBarWidth = 10

// renderQualityLine renders a single quality indicator line above stats.
func renderQualityLine(q QualityLevel) string {
	return styles.StatLabel.Render("Connection Quality  ") + RenderQualityBadge(q)
}

// renderStatsLines builds the left-column stats lines (jitter, RTT, loss, traffic).
func renderStatsLines(stats transport.ConnectionStats, jitterHist, rttHist []float64, bitrateUp, bitrateDown float64, duplex bool, packetLossPct float64) []string {
	jitterStyle := getValueStyle(stats.Jitter, 30, 100, styles.StatValueGood, styles.StatValueWarn, styles.StatValueError)
	rttStyle := getValueStyle(stats.RoundTrip, 50, 150, styles.StatValueGood, styles.StatValueWarn, styles.StatValueError)
	lossStyle := getPacketLossStyle(stats.PacketsLost, styles.StatValueGood, styles.StatValueWarn, styles.StatValueError)

	jitterSpark := RenderSparkline(jitterHist, 16)
	rttSpark := RenderSparkline(rttHist, 16)

	jitterBar := JitterBar(stats.Jitter, thresholdBarWidth)
	rttBar := RTTBar(stats.RoundTrip, thresholdBarWidth)
	lossBar := LossBar(stats.PacketsLost, thresholdBarWidth)

	jitterText := fmt.Sprintf("Jitter %s  %s", jitterStyle.Render(fmt.Sprintf("%5.1f ms", stats.Jitter)), jitterBar)
	rttText := fmt.Sprintf("RTT    %s  %s", rttStyle.Render(fmt.Sprintf("%5.1f ms", stats.RoundTrip)), rttBar)
	lossText := fmt.Sprintf("Loss   %s  %s", lossStyle.Render(fmt.Sprintf("%d / %.1f%%", stats.PacketsLost, packetLossPct)), lossBar)

	if jitterSpark != "" {
		jitterText += "  " + styles.StatLabel.Render(jitterSpark)
	}
	if rttSpark != "" {
		rttText += "  " + styles.StatLabel.Render(rttSpark)
	}

	sentBytes := FormatBytes(stats.BytesSent)
	recvBytes := FormatBytes(stats.BytesRecv)
	sentRate := formatKbps(bitrateUp)
	recvRate := formatKbps(bitrateDown)
	bytesW := max(runewidth.StringWidth(sentBytes), runewidth.StringWidth(recvBytes))
	rateW := max(runewidth.StringWidth(sentRate), runewidth.StringWidth(recvRate))

	var sentPrefix, recvPrefix string
	if duplex {
		sentPrefix = "Sending   "
		recvPrefix = "Receiving "
	}
	sent := fmt.Sprintf("%s↑ %*s  %*s", sentPrefix, bytesW, sentBytes, rateW, sentRate)
	recv := fmt.Sprintf("%s↓ %*s  %*s", recvPrefix, bytesW, recvBytes, rateW, recvRate)

	return []string{
		jitterText,
		rttText,
		lossText,
		styles.StatValueGood.Render(sent),
		styles.StatValueGood.Render(recv),
	}
}

// renderStatsWithSpectrumDual is the duplex-aware entry point called from StreamingView.
func renderStatsWithSpectrumDual(stats transport.ConnectionStats, jitterHist, rttHist []float64, width int, bitrateUp, bitrateDown float64, duplex bool, specBands, vuLevels []float64, packetLossPct float64, quality QualityLevel, capBands, capVU, playBands, playVU []float64) string {
	isDual := len(capBands) > 0 || len(playBands) > 0 || len(capVU) > 0 || len(playVU) > 0
	if !isDual {
		return renderStatsWithSpectrum(stats, jitterHist, rttHist, width, bitrateUp, bitrateDown, duplex, specBands, vuLevels, packetLossPct, quality)
	}

	var statsLines []string
	if quality != QualityUnknown {
		statsLines = append(statsLines, renderQualityLine(quality))
	}
	statsLines = append(statsLines, renderStatsLines(stats, jitterHist, rttHist, bitrateUp, bitrateDown, duplex, packetLossPct)...)

	statsWidth := 0
	for _, l := range statsLines {
		w := lipgloss.Width(l)
		if w > statsWidth {
			statsWidth = w
		}
	}

	sepWidth := 5
	vizWidth := width - statsWidth - sepWidth
	if vizWidth < 10 {
		result := strings.Join(statsLines, "\n")
		viz := RenderDualSpectrumWithVU(capBands, capVU, playBands, playVU, len(statsLines), width-4)
		if viz != "" {
			result += "\n" + viz
		}
		return result
	}

	vizHeight := len(statsLines)
	viz := RenderDualSpectrumWithVU(capBands, capVU, playBands, playVU, vizHeight, vizWidth)
	vizLines := strings.Split(viz, "\n")

	totalRows := vizHeight
	if len(vizLines) > totalRows {
		totalRows = len(vizLines)
	}

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")
	combined := make([]string, totalRows)
	for i := 0; i < totalRows; i++ {
		left := ""
		if i < len(statsLines) {
			left = statsLines[i]
		}
		lw := lipgloss.Width(left)
		pad := statsWidth - lw
		if pad < 0 {
			pad = 0
		}
		right := ""
		if i < len(vizLines) {
			right = vizLines[i]
		}
		combined[i] = left + strings.Repeat(" ", pad) + "  " + sep + "  " + right
	}

	return strings.Join(combined, "\n")
}

// renderStatsWithSpectrum builds a two-column layout: stats on the left, spectrum+VU on the right.
func renderStatsWithSpectrum(stats transport.ConnectionStats, jitterHist, rttHist []float64, width int, bitrateUp, bitrateDown float64, duplex bool, specBands, vuLevels []float64, packetLossPct float64, quality QualityLevel) string {
	var statsLines []string
	if quality != QualityUnknown {
		statsLines = append(statsLines, renderQualityLine(quality))
	}
	statsLines = append(statsLines, renderStatsLines(stats, jitterHist, rttHist, bitrateUp, bitrateDown, duplex, packetLossPct)...)

	hasViz := len(specBands) > 0 || len(vuLevels) > 0
	if !hasViz {
		return strings.Join(statsLines, "\n")
	}

	// Measure stats width
	statsWidth := 0
	for _, l := range statsLines {
		w := lipgloss.Width(l)
		if w > statsWidth {
			statsWidth = w
		}
	}

	// Separator + gap = 5 chars ("  │  ")
	sepWidth := 5
	vizWidth := width - statsWidth - sepWidth
	if vizWidth < 10 {
		// Not enough room — fall back to stacked layout
		result := strings.Join(statsLines, "\n")
		viz := RenderSpectrumWithVU(specBands, vuLevels, len(statsLines), width-4)
		if viz != "" {
			result += "\n" + viz
		}
		return result
	}

	// Build spectrum+VU rows at the height matching stats lines (includes quality line)
	vizHeight := len(statsLines)
	viz := RenderSpectrumWithVU(specBands, vuLevels, vizHeight, vizWidth)
	vizLines := strings.Split(viz, "\n")

	// Combine side by side
	totalRows := vizHeight
	if len(vizLines) > totalRows {
		totalRows = len(vizLines)
	}

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")
	combined := make([]string, totalRows)
	for i := 0; i < totalRows; i++ {
		left := ""
		if i < len(statsLines) {
			left = statsLines[i]
		}
		// Pad left to statsWidth
		lw := lipgloss.Width(left)
		pad := statsWidth - lw
		if pad < 0 {
			pad = 0
		}

		right := ""
		if i < len(vizLines) {
			right = vizLines[i]
		}

		combined[i] = left + strings.Repeat(" ", pad) + "  " + sep + "  " + right
	}

	return strings.Join(combined, "\n")
}

// formatKbps formats an instantaneous bitrate value (in kbps) for display.
func formatKbps(kbps float64) string {
	if kbps >= 1000 {
		return fmt.Sprintf("%.1f Mbps", kbps/1000)
	}
	return fmt.Sprintf("%.1f kbps", kbps)
}

func renderDevicePanel(devices []DeviceDisplayState, globalMuted bool) string {
	lines := make([]string, 0, 1+len(devices))
	header := "Devices"
	if globalMuted {
		header += "  " + styles.StatValueError.Render("[GLOBAL MUTE]")
	}
	lines = append(lines, styles.StatLabel.Render(header))

	for _, dev := range devices {
		role := "[C]"
		if dev.Role == "playback" {
			role = "[P]"
		}

		volPct := int(dev.Volume * 100)
		volBar := renderVolumeBar(dev.Volume)

		muteIcon := ""
		if dev.Disconnected {
			muteIcon = " " + styles.StatValueError.Render("[disconnected]")
		} else if dev.Muted || globalMuted {
			muteIcon = " " + styles.StatValueError.Render("MUTED")
		}

		prefix := "  "
		if dev.Selected {
			prefix = styles.DirectionReverse.Render("▸ ")
		}

		line := fmt.Sprintf("%s%s %s  %s %3d%%%s",
			prefix,
			styles.StatLabel.Render(role),
			dev.Name,
			volBar,
			volPct,
			muteIcon,
		)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderVolumeBar(volume float64) string {
	const barLen = 10
	filled := int(volume / 2.0 * float64(barLen))
	if filled > barLen {
		filled = barLen
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	if volume > 1.0 {
		return styles.StatValueWarn.Render(bar)
	}
	return styles.StatValueGood.Render(bar)
}

// countRenderedLines counts the total number of lines in pre-rendered sections.
func countRenderedLines(sections []string) int {
	n := 0
	for _, s := range sections {
		n += strings.Count(s, "\n") + 1
	}
	return n
}

// renderLogPanelWithMax renders a log panel with an explicit max number of visible lines.
func renderLogPanelWithMax(logs []string, scrollOffset, width, maxVisible int) string {
	if maxVisible < 3 {
		maxVisible = 3
	}
	return renderLogPanelInner(logs, scrollOffset, width, maxVisible)
}

func renderLogPanelInner(logs []string, scrollOffset, width, maxVisible int) string {
	total := len(logs)
	end := total - scrollOffset
	if end < 0 {
		end = 0
	}
	if end > total {
		end = total
	}
	start := end - maxVisible
	if start < 0 {
		start = 0
	}
	visible := logs[start:end]

	panelWidth := width - 2
	if panelWidth < 40 {
		panelWidth = 40
	}

	var sb strings.Builder
	for _, line := range visible {
		sb.WriteString(colorizeLogLine(line, panelWidth) + "\n")
	}

	if scrollOffset > 0 {
		hint := styles.ScrollHint.Render(fmt.Sprintf("[SCROLLED ↑%d]", scrollOffset))
		sb.WriteString(hint)
	}

	return strings.TrimRight(sb.String(), "\n")
}

func colorizeLogLine(line string, maxWidth int) string {
	if runewidth.StringWidth(line) > maxWidth {
		line = runewidth.Truncate(line, maxWidth-1, "") + "…"
	}

	if strings.Contains(line, "[ERR]") {
		parts := strings.SplitN(line, "[ERR]", 2)
		return styles.LogTime.Render(parts[0]) +
			styles.LogLevelError.Render("[ERR]") +
			styles.LogError.Render(parts[1])
	}
	if strings.Contains(line, "[WRN]") {
		parts := strings.SplitN(line, "[WRN]", 2)
		return styles.LogTime.Render(parts[0]) +
			styles.LogLevelWarn.Render("[WRN]") +
			styles.LogWarn.Render(parts[1])
	}
	if strings.Contains(line, "[DBG]") {
		parts := strings.SplitN(line, "[DBG]", 2)
		return styles.LogTime.Render(parts[0]) +
			styles.LogLevelDebug.Render("[DBG]") +
			styles.LogDebug.Render(parts[1])
	}
	if strings.Contains(line, "[INF]") {
		parts := strings.SplitN(line, "[INF]", 2)
		return styles.LogTime.Render(parts[0]) +
			styles.LogLevelInfo.Render("[INF]") +
			styles.LogInfo.Render(parts[1])
	}
	return styles.LogInfo.Render(line)
}

// FormatBytes formats a byte count as a human-readable string.
// FormatBytes formats bytes using SI units (KB/MB/GB) — standard for network traffic.
func FormatBytes(b uint64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a duration as HH:MM:SS or MM:SS.
func FormatDuration(d time.Duration) string {
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

// CalculateBitrate computes human-readable bitrate from byte counts and elapsed time.
func CalculateBitrate(bytesSent, bytesRecv uint64, elapsed time.Duration) string {
	if elapsed == 0 {
		return "0.0 kbps"
	}
	totalBytes := bytesSent + bytesRecv
	bits := totalBytes * 8
	seconds := elapsed.Seconds()
	if seconds == 0 {
		return "0.0 kbps"
	}
	kbps := float64(bits) / seconds / 1000
	if kbps >= 1000 {
		return fmt.Sprintf("%.1f Mbps", kbps/1000)
	}
	return fmt.Sprintf("%.1f kbps", kbps)
}

func getValueStyle(value, warnThreshold, errorThreshold float64, goodStyle, warningStyle, errorStyle lipgloss.Style) lipgloss.Style {
	switch {
	case value >= errorThreshold:
		return errorStyle
	case value >= warnThreshold:
		return warningStyle
	default:
		return goodStyle
	}
}

func getPacketLossStyle(packetsLost uint32, goodStyle, warningStyle, errorStyle lipgloss.Style) lipgloss.Style {
	switch {
	case packetsLost >= 10:
		return errorStyle
	case packetsLost >= 3:
		return warningStyle
	default:
		return goodStyle
	}
}
