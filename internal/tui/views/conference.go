package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// ConferenceParticipant holds display data for a conference participant.
type ConferenceParticipant struct {
	ID       string
	Volume   float32
	Muted    bool
	Speaking bool
	RMSLevel float32 // 0.0–1.0 for gradual VU meter
}

// ConferenceParams contains parameters for the conference streaming view.
type ConferenceParams struct {
	// Multi-client network stats (for clients list and aggregate traffic).
	Stats        transport.MultiClientStats
	Participants []ConferenceParticipant

	// Server identity & mode.
	IsServer    bool // true when running as server
	HubMode     bool // true when server is hub (not a participant)
	ServerMuted bool // legacy compat, same as HubMode for display

	// Navigation.
	SelectedIndex int

	// Device name for summary line.
	DeviceName string

	// Logs.
	Logs            []string
	LogsVisible     bool
	LogScrollOffset int

	// Terminal size.
	Width  int
	Height int

	// Chat panel.
	ChatView string

	// Selected participant detail data.
	SelectedStats       transport.ConnectionStats
	SelectedJitterHist  []float64
	SelectedRTTHist     []float64
	SelectedBitrateUp   float64
	SelectedBitrateDown float64
	SelectedQuality     QualityLevel
	SelectedPacketLoss  float64

	// Aggregate traffic.
	AggBytesSent   uint64
	AggBytesRecv   uint64
	AggBitrateUp   float64
	AggBitrateDown float64

	// Per-client quality for list badges.
	ClientQualities []QualityLevel

	// Visualization — dual spectrum (capture + playback).
	CaptureSpectrumBands  []float64
	CaptureVULevels       []float64
	PlaybackSpectrumBands []float64
	PlaybackVULevels      []float64

	// Paused state.
	Paused bool

	// Client-side participant list (from chat layer — for client view).
	OnlineParticipants []string // nicknames of all online participants
	MaxClients         int      // session capacity
	MyNickname         string   // own nickname for "(me)" suffix

	// Mute state (from MuteState in TUI model).
	MutedParticipants  map[string]bool // per-participant mute map
	MuteAll            bool            // true when mute-all is active
	PausedParticipants map[string]bool // per-participant pause map (broadcast from server)
}

// conferenceParticipantList builds the full navigable list of participants.
// Returns: list of entries and which index in the list maps to selected network client (for detail).
type conferenceListEntry struct {
	label     string // rendered line
	isServer  bool   // true for the server entry
	clientIdx int    // index into Stats.Clients (-1 for server)
}

// ConferenceView renders the conference mode streaming screen body with master-detail layout.
func ConferenceView(p ConferenceParams) string {
	var sections []string

	// Summary line: direction + device
	sections = append(sections, renderConferenceSummaryLine(p), renderSep(p.Width))

	// Master-detail area
	if p.Width < minTerminalWidth {
		sections = append(sections, renderConferenceStacked(p))
	} else {
		sections = append(sections, renderConferenceMasterDetail(p))
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
		logPanel := renderLogPanelWithMax(p.Logs, p.LogScrollOffset, p.Width, remaining-1)
		sections = append(sections, logPanel)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderConferenceSummaryLine renders the top summary line for conference view.
func renderConferenceSummaryLine(p ConferenceParams) string {
	dirText := i18n.T("streaming_dir_conference")
	if p.HubMode {
		dirText = i18n.T("streaming_dir_conference_hub")
	}
	if p.Paused {
		dirText += " " + i18n.T("streaming_badge_paused")
	}

	var style lipgloss.Style
	if p.Paused {
		style = styles.Paused
	} else {
		style = styles.Direction
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

// buildConferenceList builds the navigable participant list for the left column.
func buildConferenceList(p ConferenceParams) []conferenceListEntry {
	var entries []conferenceListEntry

	if p.IsServer {
		// Server mode: server entry (if not hub) + all network clients.
		if !p.HubMode {
			entries = append(entries, conferenceListEntry{
				label:     i18n.T("multi_server_me"),
				isServer:  true,
				clientIdx: -1,
			})
		}
		for i, c := range p.Stats.Clients {
			nick := c.Nickname
			if nick == "" {
				nick = c.ClientID
			}
			entries = append(entries, conferenceListEntry{
				label:     nick,
				isServer:  false,
				clientIdx: i,
			})
		}
	} else {
		// Client mode: use OnlineParticipants if available, otherwise Stats.Clients.
		if len(p.OnlineParticipants) > 0 {
			for _, nick := range p.OnlineParticipants {
				entry := conferenceListEntry{
					label:     nick,
					isServer:  false,
					clientIdx: -1,
				}
				// Check if this is "server"
				if nick == "server" || nick == "Server" {
					entry.isServer = true
				}
				// Check if this is "me"
				if nick == p.MyNickname {
					entry.label = nick + i18n.T("streaming_me_suffix")
				}
				// Try to find matching client index
				for i, c := range p.Stats.Clients {
					cNick := c.Nickname
					if cNick == "" {
						cNick = c.ClientID
					}
					if cNick == nick {
						entry.clientIdx = i
						break
					}
				}
				entries = append(entries, entry)
			}
		} else {
			// Fallback: just show clients from stats
			for i, c := range p.Stats.Clients {
				nick := c.Nickname
				if nick == "" {
					nick = c.ClientID
				}
				entries = append(entries, conferenceListEntry{
					label:     nick,
					isServer:  false,
					clientIdx: i,
				})
			}
		}
	}

	return entries
}

// renderConferenceLeftColumn renders the master column: aggregate traffic + participant list.
func renderConferenceLeftColumn(p ConferenceParams, colWidth int) []string {
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

	// Build participant list
	entries := buildConferenceList(p)

	if len(entries) == 0 {
		lines = append(lines, "  "+styles.EmptyState.Render(i18n.T("multi_no_participants")))
		return lines
	}

	for i, entry := range entries {
		lines = append(lines, renderConferenceListItem(entry, p, i == p.SelectedIndex, colWidth))
	}

	return lines
}

// renderConferenceListItem renders a single participant in the master list.
func renderConferenceListItem(entry conferenceListEntry, p ConferenceParams, selected bool, maxWidth int) string {
	prefix := "  "
	if selected {
		prefix = styles.SelectedItem.Render(styles.CursorGlyph) + " "
	}

	nick := entry.label

	// For client-side "me" entries, use bullet prefix
	if !p.IsServer && strings.HasSuffix(nick, i18n.T("streaming_me_suffix")) && !selected {
		prefix = styles.ParticipantBullet.Render(" • ")
	}

	// Server entry: no badge, no duration
	if entry.isServer {
		nickMax := maxWidth - 2
		if nickMax < 4 {
			nickMax = 4
		}
		if runewidth.StringWidth(nick) > nickMax {
			nick = runewidth.Truncate(nick, nickMax-1, "…")
		}
		return prefix + styles.ClientListItem.Render(nick)
	}

	// Network client: show quality badge + duration
	var badge string
	var dur string
	if entry.clientIdx >= 0 && entry.clientIdx < len(p.Stats.Clients) {
		c := p.Stats.Clients[entry.clientIdx]
		dur = c.Duration

		if entry.clientIdx < len(p.ClientQualities) {
			q := p.ClientQualities[entry.clientIdx]
			if q != QualityUnknown {
				badge = q.Emoji()
			}
		}
	}

	// Check muted state: prefer MutedParticipants map (TUI MuteState), fallback to Participants.
	isMuted := false
	if len(p.MutedParticipants) > 0 {
		// Check by participant ID from the Participants slice or by client ID.
		for _, part := range p.Participants {
			if part.ID == entry.label || (entry.clientIdx >= 0 && entry.clientIdx < len(p.Stats.Clients) && part.ID == p.Stats.Clients[entry.clientIdx].ClientID) {
				if p.MutedParticipants[part.ID] {
					isMuted = true
				}
				break
			}
		}
	} else {
		for _, part := range p.Participants {
			if part.ID == entry.label || (entry.clientIdx >= 0 && entry.clientIdx < len(p.Stats.Clients) && part.ID == p.Stats.Clients[entry.clientIdx].ClientID) {
				isMuted = part.Muted
				break
			}
		}
	}
	if isMuted {
		badge = "🔇"
	}

	// Check paused state from PausedParticipants map.
	isPaused := false
	if len(p.PausedParticipants) > 0 {
		for _, part := range p.Participants {
			if part.ID == entry.label || (entry.clientIdx >= 0 && entry.clientIdx < len(p.Stats.Clients) && part.ID == p.Stats.Clients[entry.clientIdx].ClientID) {
				if p.PausedParticipants[part.ID] {
					isPaused = true
				}
				break
			}
		}
		// Also check by label directly (for client-side OnlineParticipants).
		if !isPaused && p.PausedParticipants[entry.label] {
			isPaused = true
		}
	}
	if isPaused {
		if badge != "" {
			badge += " ⏸"
		} else {
			badge = "⏸"
		}
	}

	// Build: prefix + nick + "  " + badge + "  " + dur
	reserved := 2 + 4 + runewidth.StringWidth(dur)
	if badge != "" {
		reserved += runewidth.StringWidth(badge) + 1
	}
	nickMax := maxWidth - reserved
	if nickMax < 4 {
		nickMax = 4
	}
	if runewidth.StringWidth(nick) > nickMax {
		nick = runewidth.Truncate(nick, nickMax-1, "…")
	}

	var item string
	if badge != "" {
		nickW := runewidth.StringWidth(nick)
		badgeW := runewidth.StringWidth(badge)
		durW := runewidth.StringWidth(dur)
		gap := maxWidth - 2 - nickW - badgeW - durW - 3
		if gap < 1 {
			gap = 1
		}
		item = prefix + styles.ClientListItem.Render(nick) + "  " + badge + strings.Repeat(" ", gap) + styles.StatLabel.Render(dur)
	} else {
		nickW := runewidth.StringWidth(nick)
		durW := runewidth.StringWidth(dur)
		gap := maxWidth - 2 - nickW - durW - 2
		if gap < 1 {
			gap = 1
		}
		item = prefix + styles.ClientListItem.Render(nick) + strings.Repeat(" ", gap) + styles.StatLabel.Render(dur)
	}

	return item
}

// renderConferenceDetailHeader renders the detail panel header for conference mode.
func renderConferenceDetailHeader(p ConferenceParams, entries []conferenceListEntry, detailWidth int) string {
	// Participants count: clients + server (if not hub)
	totalParticipants := len(p.Stats.Clients)
	maxParticipants := p.Stats.MaxClients
	if p.IsServer && !p.HubMode {
		totalParticipants++
		maxParticipants++
	}

	capacity := i18n.Tf("multi_participants_capacity", totalParticipants, maxParticipants)
	if p.MuteAll {
		capacity += " " + i18n.T("multi_mute_all_badge")
	}

	if len(entries) == 0 || p.SelectedIndex >= len(entries) {
		return styles.DetailHeader.Render(capacity)
	}

	entry := entries[p.SelectedIndex]

	// For server entry, just show capacity
	if entry.isServer {
		return styles.DetailHeader.Render(capacity) + "  " + styles.ClientListItem.Render(styles.CursorGlyph+" "+i18n.T("multi_server_me"))
	}

	// For client entry, show client info
	if entry.clientIdx >= 0 && entry.clientIdx < len(p.Stats.Clients) {
		c := p.Stats.Clients[entry.clientIdx]
		nick := c.Nickname
		if nick == "" {
			nick = c.ClientID
		}
		clientInfo := fmt.Sprintf("%s %s — %s", styles.CursorGlyph, nick, c.RemoteAddr)
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

	return styles.DetailHeader.Render(capacity)
}

// renderConferenceDetailPanel renders the full detail panel for the selected participant.
func renderConferenceDetailPanel(p ConferenceParams, entries []conferenceListEntry, detailWidth int) string {
	var lines []string

	// Header
	lines = append(lines, renderConferenceDetailHeader(p, entries, detailWidth), styles.Separator.Render(strings.Repeat("─", detailWidth)))

	if len(entries) == 0 || p.SelectedIndex >= len(entries) {
		lines = append(lines, styles.EmptyState.Render(i18n.T("multi_waiting_participants")))
		return strings.Join(lines, "\n")
	}

	entry := entries[p.SelectedIndex]

	// Server entry: no network stats available
	if entry.isServer {
		lines = append(lines, styles.StatLabel.Render(i18n.T("multi_local_participant_no_stats")))
		return strings.Join(lines, "\n")
	}

	// Client entry: show full stats + dual spectrum
	specBands := p.CaptureSpectrumBands
	vuLevels := p.CaptureVULevels
	jitterHist := p.SelectedJitterHist
	rttHist := p.SelectedRTTHist

	// Width-based graceful degradation
	if p.Width < 100 {
		specBands = nil
		vuLevels = nil
	}
	if p.Width < 60 {
		jitterHist = nil
		rttHist = nil
	}

	var capBands, capVU, playBands, playVU []float64
	if p.Width >= 100 {
		capBands = p.CaptureSpectrumBands
		capVU = p.CaptureVULevels
		playBands = p.PlaybackSpectrumBands
		playVU = p.PlaybackVULevels
	}

	_ = specBands
	_ = vuLevels

	statsPanel := renderStatsWithSpectrumDual(
		p.SelectedStats,
		jitterHist,
		rttHist,
		detailWidth,
		p.SelectedBitrateUp,
		p.SelectedBitrateDown,
		true, // conference is always duplex-like
		nil,  // single spectrum bands not used in dual mode
		nil,  // single VU not used in dual mode
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

// renderConferenceMasterDetail joins left master column and right detail column with │ separator.
func renderConferenceMasterDetail(p ConferenceParams) string {
	// 30% of terminal width, clamped to [min, max]
	colWidth := p.Width * 30 / 100
	if colWidth < leftColumnWidthMin {
		colWidth = leftColumnWidthMin
	}
	if colWidth > leftColumnWidthMax {
		colWidth = leftColumnWidthMax
	}

	detailWidth := p.Width - colWidth - 3 // 3 for " │ "
	if detailWidth < minDetailWidth {
		return renderConferenceStacked(p)
	}

	leftLines := renderConferenceLeftColumn(p, colWidth)
	entries := buildConferenceList(p)
	detailStr := renderConferenceDetailPanel(p, entries, detailWidth)
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

// renderConferenceStacked is a fallback for narrow terminals.
func renderConferenceStacked(p ConferenceParams) string {
	var lines []string

	entries := buildConferenceList(p)

	// Participant count
	totalParticipants := len(p.Stats.Clients)
	if p.IsServer && !p.HubMode {
		totalParticipants++
	}
	stackedCapacity := i18n.Tf("multi_participants_capacity", totalParticipants, p.Stats.MaxClients)
	if p.MuteAll {
		stackedCapacity += " " + i18n.T("multi_mute_all_badge")
	}
	lines = append(lines, styles.DetailHeader.Render(stackedCapacity))

	// Participant list
	if len(entries) == 0 {
		lines = append(lines, "  "+styles.EmptyState.Render(i18n.T("multi_waiting_participants")))
	} else {
		w := p.Width
		if w < leftColumnWidthMin {
			w = leftColumnWidthMin
		}
		for i, entry := range entries {
			lines = append(lines, renderConferenceListItem(entry, p, i == p.SelectedIndex, w))
		}
	}

	// Separator
	lines = append(lines, renderSep(p.Width))

	// Selected stats (no spectrum in narrow mode)
	selectedEntry := conferenceListEntry{clientIdx: -1, isServer: true}
	if p.SelectedIndex < len(entries) {
		selectedEntry = entries[p.SelectedIndex]
	}

	if selectedEntry.isServer {
		lines = append(lines, styles.StatLabel.Render(i18n.T("multi_local_participant_no_stats")))
	} else if selectedEntry.clientIdx >= 0 {
		jitterHist := p.SelectedJitterHist
		rttHist := p.SelectedRTTHist
		if p.Width < 60 {
			jitterHist = nil
			rttHist = nil
		}
		if p.SelectedQuality != QualityUnknown {
			lines = append(lines, renderQualityLine(p.SelectedQuality))
		}
		statsLines := renderStatsLines(
			p.SelectedStats,
			jitterHist,
			rttHist,
			p.SelectedBitrateUp,
			p.SelectedBitrateDown,
			true,
			p.SelectedPacketLoss,
		)
		lines = append(lines, statsLines...)
	} else {
		lines = append(lines, styles.EmptyState.Render(i18n.T("multi_waiting_participants")))
	}

	return strings.Join(lines, "\n")
}

// ConferenceParticipantCount returns the total number of navigable entries.
func ConferenceParticipantCount(p ConferenceParams) int {
	return len(buildConferenceList(p))
}
