package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// Layout constants for master-detail view.
const (
	leftColumnWidth    = 26 // width of master column
	leftColumnWidthMin = 20 // minimum at narrow terminals
	minDetailWidth     = 30 // minimum for detail panel
	minTerminalWidth   = 50 // absolute minimum terminal width
)

// MultiClientParams contains parameters for the multi-client streaming view.
type MultiClientParams struct {
	// Existing
	Stats           transport.MultiClientStats
	Reverse         bool
	Duplex          bool
	DeviceName      string
	SelectedIndex   int
	Logs            []string
	LogsVisible     bool
	LogScrollOffset int
	Width           int
	Height          int

	// Per-client detail data
	SelectedStats       transport.ConnectionStats
	SelectedJitterHist  []float64
	SelectedRTTHist     []float64
	SelectedBitrateUp   float64
	SelectedBitrateDown float64
	SelectedQuality     QualityLevel
	SelectedPacketLoss  float64

	// Aggregate traffic
	AggBytesSent   uint64
	AggBytesRecv   uint64
	AggBitrateUp   float64
	AggBitrateDown float64

	// Per-client quality for list badges
	ClientQualities []QualityLevel

	// Per-client volumes (keyed by ClientID, 0.0–1.5, default 1.0)
	ClientVolumes map[string]float64

	// Visualization
	SpectrumBands []float64
	VULevels      []float64

	// Dual spectrum for duplex mode
	CaptureSpectrumBands  []float64
	CaptureVULevels       []float64
	PlaybackSpectrumBands []float64
	PlaybackVULevels      []float64

	// Chat
	ChatView string

	// Nickname for summary line
	Nickname string

	// Paused state
	Paused bool

	// FocusedArea indicates which area has keyboard focus (0=ClientList, 1=Chat, 2=Logs).
	FocusedArea int
}

// MultiClientView renders the multi-client streaming screen body with master-detail layout.
func MultiClientView(p MultiClientParams) string {
	var sections []string

	// Summary line: direction + device
	sections = append(sections, renderMultiSummaryLine(p))

	// Duplex warning
	if p.Duplex {
		sections = append(sections, styles.StatValueWarn.Render("  ⚠ Duplex mode: headphones recommended to avoid echo"))
	}

	// Separator
	sections = append(sections, renderSep(p.Width))

	// Master-detail area
	if p.Width < minTerminalWidth {
		// Too narrow — show minimal fallback
		sections = append(sections, renderStackedLayout(p))
	} else {
		sections = append(sections, renderMasterDetail(p))
	}

	// Chat panel
	if p.ChatView != "" && p.Height >= 20 {
		sections = append(sections, renderSep(p.Width), p.ChatView)
	}

	// Logs — calculate remaining height to avoid pushing TUI off screen.
	if p.LogsVisible && len(p.Logs) > 0 && p.Height >= 25 {
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
		logPanel := renderLogPanelWithMax(p.Logs, p.LogScrollOffset, p.Width, remaining-1) // -1 for sep
		sections = append(sections, logPanel)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderMultiSummaryLine renders the top summary line for multi-client view.
func renderMultiSummaryLine(p MultiClientParams) string {
	var dirText string
	var style lipgloss.Style

	if p.Paused {
		if p.Duplex {
			dirText = "⇄ server ↔ clients (duplex) [PAUSED]"
		} else if p.Reverse {
			dirText = "▸ clients → server (reverse) [PAUSED]"
		} else {
			dirText = "▸ server → clients (normal) [PAUSED]"
		}
		style = styles.Paused
	} else if p.Duplex {
		dirText = "⇄ server ↔ clients (duplex)"
		style = styles.Direction
	} else if p.Reverse {
		dirText = "▸ clients → server (reverse)"
		style = styles.DirectionReverse
	} else {
		dirText = "▸ server → clients (normal)"
		style = styles.Direction
	}

	if p.Nickname != "" {
		dirText += "  " + p.Nickname
	}

	left := style.Render(dirText)
	if p.DeviceName != "" {
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(p.DeviceName)
		spacing := p.Width - leftW - rightW
		if spacing < 2 {
			spacing = 2
		}
		return left + strings.Repeat(" ", spacing) + styles.StatLabel.Render(p.DeviceName)
	}
	return left
}

// renderLeftColumn renders the master column: aggregate traffic + client list.
func renderLeftColumn(p MultiClientParams, colWidth int) []string {
	var lines []string

	// Aggregate traffic
	sentBytes := FormatBytes(p.AggBytesSent)
	recvBytes := FormatBytes(p.AggBytesRecv)
	sentRate := formatKbps(p.AggBitrateUp)
	recvRate := formatKbps(p.AggBitrateDown)

	lines = append(lines,
		styles.AggregateTraffic.Render(fmt.Sprintf("  ↑ %s  %s", sentBytes, sentRate)),
		styles.AggregateTraffic.Render(fmt.Sprintf("  ↓ %s  %s", recvBytes, recvRate)),
	)

	// Separator
	sep := strings.Repeat("─", colWidth)
	lines = append(lines, styles.Separator.Render(sep))

	// Client list
	if len(p.Stats.Clients) == 0 {
		lines = append(lines, "  "+styles.EmptyState.Render("No clients"))
	} else {
		for i, c := range p.Stats.Clients {
			var q QualityLevel
			if i < len(p.ClientQualities) {
				q = p.ClientQualities[i]
			}
			vol := -1.0 // no volume bar by default
			if p.ClientVolumes != nil {
				if v, ok := p.ClientVolumes[c.ClientID]; ok {
					vol = v
				} else {
					vol = 1.0 // default volume
				}
			}
			lines = append(lines, renderClientListItem(c, q, i == p.SelectedIndex, colWidth, vol))
		}
	}

	return lines
}

// renderClientListItem renders a single client in the master list: "▸ Nick  🟢  ████░░ 100%  6s"
// volume < 0 means no volume bar is shown.
func renderClientListItem(c transport.ClientInfo, q QualityLevel, selected bool, maxWidth int, volume float64) string {
	prefix := "  "
	if selected {
		prefix = styles.SelectedItem.Render(styles.CursorGlyph) + " "
	}

	nick := c.Nickname
	if nick == "" {
		nick = c.ClientID
	}

	badge := ""
	if q != QualityUnknown {
		badge = q.Emoji()
	}
	showUp := c.MutedOutgoing
	showDown := c.Muted || c.MutedIncoming
	if showUp && showDown {
		badge += " 🔇↑↓"
	} else if showUp {
		badge += " 🔇↑"
	} else if showDown {
		badge += " 🔇↓"
	}
	if c.Paused {
		badge += " ⏸"
	}
	badge = strings.TrimSpace(badge)

	// Volume bar (compact: 6 chars + space + 4 chars for pct)
	volStr := ""
	if volume >= 0 {
		volStr = renderCompactVolumeBar(volume)
	}

	dur := c.Duration
	durW := runewidth.StringWidth(dur)

	// Drop badge entirely if terminal is too narrow to show it alongside a minimum nick (4 chars).
	// Minimum viable row: prefix(2) + nick(4) + space(1) + badge + space(1) + dur
	volW := runewidth.StringWidth(volStr)
	if badge != "" {
		minWithBadge := 2 + 4 + 1 + runewidth.StringWidth(badge) + 1 + volW + 1 + durW
		if minWithBadge > maxWidth {
			badge = ""
		}
	}

	// Build: prefix + nick + "  " + badge + "  " + volBar + gap + dur
	reserved := 2 + 3 + durW
	if badge != "" {
		reserved += runewidth.StringWidth(badge) + 1
	}
	if volW > 0 {
		reserved += volW + 1
	}
	nickMax := maxWidth - reserved
	if nickMax < 4 {
		nickMax = 4
	}
	if runewidth.StringWidth(nick) > nickMax {
		nick = runewidth.Truncate(nick, nickMax-1, "…")
	}

	// Assemble the middle part (badge + volume)
	middle := ""
	if badge != "" {
		middle += badge
	}
	if volStr != "" {
		if middle != "" {
			middle += " "
		}
		middle += volStr
	}

	var item string
	if middle != "" {
		nickW := runewidth.StringWidth(nick)
		middleW := runewidth.StringWidth(middle)
		gap := maxWidth - 2 - nickW - middleW - durW - 3
		if gap < 1 {
			gap = 1
		}
		item = prefix + styles.ClientListItem.Render(nick) + "  " + middle + strings.Repeat(" ", gap) + styles.StatLabel.Render(dur)
	} else {
		nickW := runewidth.StringWidth(nick)
		gap := maxWidth - 2 - nickW - durW - 2
		if gap < 1 {
			gap = 1
		}
		item = prefix + styles.ClientListItem.Render(nick) + strings.Repeat(" ", gap) + styles.StatLabel.Render(dur)
	}

	return item
}

// renderDetailHeader renders the detail panel header: "Clients N/M  ▸ Nick — IP    duration"
func renderDetailHeader(p MultiClientParams, detailWidth int) string {
	nClients := len(p.Stats.Clients)
	capacity := fmt.Sprintf("Clients %d/%d", nClients, p.Stats.MaxClients)

	if nClients == 0 || p.SelectedIndex >= nClients {
		return styles.DetailHeader.Render(capacity)
	}

	c := p.Stats.Clients[p.SelectedIndex]
	nick := c.Nickname
	if nick == "" {
		nick = c.ClientID
	}

	statusBadges := ""
	detailUp := c.MutedOutgoing
	detailDown := c.Muted || c.MutedIncoming
	if detailUp && detailDown {
		statusBadges += " 🔇↑↓"
	} else if detailUp {
		statusBadges += " 🔇↑"
	} else if detailDown {
		statusBadges += " 🔇↓"
	}
	if c.Paused {
		statusBadges += " ⏸"
	}
	clientInfo := fmt.Sprintf("%s %s%s — %s", styles.CursorGlyph, nick, statusBadges, c.RemoteAddr)
	dur := c.Duration

	left := styles.DetailHeader.Render(capacity) + "  " + styles.ClientListItem.Render(clientInfo)
	leftW := lipgloss.Width(left)
	durW := runewidth.StringWidth(dur)
	gap := detailWidth - leftW - durW
	if gap < 2 {
		gap = 2
	}

	return left + strings.Repeat(" ", gap) + styles.StatLabel.Render(dur)
}

// renderDetailPanel renders the full detail panel for the selected client.
func renderDetailPanel(p MultiClientParams, detailWidth int) string {
	var lines []string

	// Header
	lines = append(lines, renderDetailHeader(p, detailWidth), styles.Separator.Render(strings.Repeat("─", detailWidth)))

	if len(p.Stats.Clients) == 0 || p.SelectedIndex >= len(p.Stats.Clients) {
		lines = append(lines, styles.EmptyState.Render("Waiting for clients…"))
		return strings.Join(lines, "\n")
	}

	// Width-based graceful degradation (spec 5.5):
	//   ≥100 full layout (spectrum + sparklines)
	//   80-99 no spectrum/VU
	//   60-79 no sparklines
	//   <60  minimal (handled by stacked fallback in renderMasterDetail)
	specBands := p.SpectrumBands
	vuLevels := p.VULevels
	jitterHist := p.SelectedJitterHist
	rttHist := p.SelectedRTTHist
	if p.Width < 100 {
		specBands = nil
		vuLevels = nil
	}
	if p.Width < 60 {
		jitterHist = nil
		rttHist = nil
	}

	// Stats with spectrum — reuse from streaming.go
	var capBands, capVU, playBands, playVU []float64
	if p.Width >= 100 {
		capBands = p.CaptureSpectrumBands
		capVU = p.CaptureVULevels
		playBands = p.PlaybackSpectrumBands
		playVU = p.PlaybackVULevels
	}
	statsPanel := renderStatsWithSpectrumDual(
		p.SelectedStats,
		jitterHist,
		rttHist,
		detailWidth,
		p.SelectedBitrateUp,
		p.SelectedBitrateDown,
		p.Duplex,
		specBands,
		vuLevels,
		p.SelectedPacketLoss,
		p.SelectedQuality,
		capBands,
		capVU,
		playBands,
		playVU,
	)
	lines = append(lines, statsPanel)

	return strings.Join(lines, "\n")
}

// renderMasterDetail joins left master column and right detail column with │ separator.
func renderMasterDetail(p MultiClientParams) string {
	colWidth := leftColumnWidth
	if p.Width < 80 {
		colWidth = leftColumnWidthMin
	}

	detailWidth := p.Width - colWidth - 3 // 3 for " │ "
	if detailWidth < minDetailWidth {
		// Stacked fallback
		return renderStackedLayout(p)
	}

	leftLines := renderLeftColumn(p, colWidth)
	detailStr := renderDetailPanel(p, detailWidth)
	rightLines := strings.Split(detailStr, "\n")

	sep := styles.ColumnSeparator.Render("│")

	totalRows := len(leftLines)
	if len(rightLines) > totalRows {
		totalRows = len(rightLines)
	}

	combined := make([]string, totalRows)
	for i := 0; i < totalRows; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		// Pad left to colWidth
		lw := lipgloss.Width(left)
		pad := colWidth - lw
		if pad < 0 {
			pad = 0
		}

		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}

		combined[i] = left + strings.Repeat(" ", pad) + " " + sep + " " + right
	}

	return strings.Join(combined, "\n")
}

// renderStackedLayout is a fallback for very narrow terminals — stacks master above detail.
func renderStackedLayout(p MultiClientParams) string {
	var lines []string

	// Capacity
	nClients := len(p.Stats.Clients)
	lines = append(lines, styles.DetailHeader.Render(fmt.Sprintf("Clients %d/%d", nClients, p.Stats.MaxClients)))

	// Client list (compact)
	if nClients == 0 {
		lines = append(lines, "  "+styles.EmptyState.Render("Waiting for clients…"))
	} else {
		w := p.Width
		if w < leftColumnWidthMin {
			w = leftColumnWidthMin
		}
		for i, c := range p.Stats.Clients {
			var q QualityLevel
			if i < len(p.ClientQualities) {
				q = p.ClientQualities[i]
			}
			vol := -1.0
			if p.ClientVolumes != nil {
				if v, ok := p.ClientVolumes[c.ClientID]; ok {
					vol = v
				} else {
					vol = 1.0
				}
			}
			lines = append(lines, renderClientListItem(c, q, i == p.SelectedIndex, w, vol))
		}
	}

	// Separator
	lines = append(lines, renderSep(p.Width))

	// Selected client stats (no spectrum in narrow mode, no sparklines at <60)
	if nClients > 0 && p.SelectedIndex < nClients {
		jitterHist := p.SelectedJitterHist
		rttHist := p.SelectedRTTHist
		if p.Width < 60 {
			jitterHist = nil
			rttHist = nil
		}
		statsLines := renderStatsLines(
			p.SelectedStats,
			jitterHist,
			rttHist,
			p.SelectedBitrateUp,
			p.SelectedBitrateDown,
			p.Duplex,
			p.SelectedPacketLoss,
		)
		if p.SelectedQuality != QualityUnknown {
			lines = append(lines, renderQualityLine(p.SelectedQuality))
		}
		lines = append(lines, statsLines...)
	} else {
		lines = append(lines, styles.EmptyState.Render("Waiting for clients…"))
	}

	return strings.Join(lines, "\n")
}
