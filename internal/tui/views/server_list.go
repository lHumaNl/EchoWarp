package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// ServerSource indicates where a server entry was discovered.
type ServerSource int

const (
	ServerSourceMDNS   ServerSource = iota // found via mDNS
	ServerSourceRecent                     // from recent connections history
	ServerSourceBoth                       // merged mDNS + recent
)

// ServerProbeStatus tracks the probe lifecycle for a server entry.
type ServerProbeStatus int

const (
	ServerProbePending ServerProbeStatus = iota // not yet probed
	ServerProbeProbing                          // probe in progress
	ServerProbeOnline                           // probe succeeded
	ServerProbeOffline                          // probe failed
)

// ServerEntry represents a server in the picker list.
type ServerEntry struct {
	Address       string
	Port          int
	Hostname      string
	ServerID      string // Stable per-port UUID; empty for old servers/entries.
	Source        ServerSource
	LastConnected time.Time
	ProbeStatus   ServerProbeStatus
	ProbeResult   *probe.ProbeServerResult
	ProbeError    error
}

// ID returns a unique identifier for this entry ("addr:port").
func (e ServerEntry) ID() string {
	return fmt.Sprintf("%s:%d", e.Address, e.Port)
}

// ServerSelectedMsg is returned when the user selects a server from the list.
type ServerSelectedMsg struct {
	Entry ServerEntry
}

// ScanRequestMsg is returned when the user activates the [Scan] button.
type ScanRequestMsg struct{}

// ServerListExitDownMsg is returned when Down is pressed at the bottom of the server list,
// signaling that focus should move to the settings fields below.
type ServerListExitDownMsg struct{}

// ServerListExitUpMsg is returned when Up is pressed at the top of the server list
// while scan button is not focused — currently unused but reserved for symmetry.
type ServerListExitUpMsg struct{}

// maxVisibleServers is the number of server entries visible before scrolling kicks in.
const maxVisibleServers = 5

// ServerListModel is a Bubble Tea component for displaying and navigating a server list.
type ServerListModel struct {
	entries      []ServerEntry
	cursor       int  // highlighted row
	selected     int  // index of active (selected) server, -1 = none
	scanFocused  bool // focus on [Scan] button
	spinnerFrame int
	height       int
	width        int
	scrollOffset int
	scanning     bool // true while mDNS scan is in progress
	hasRecent    bool // true if recent servers were loaded at init
}

// NewServerListModel creates a new server list component.
func NewServerListModel(width, height int) ServerListModel {
	return ServerListModel{
		selected: -1,
		width:    width,
		height:   height,
	}
}

// SetScanning sets the scanning state for empty-state display.
func (m *ServerListModel) SetScanning(scanning bool) {
	m.scanning = scanning
}

// IsScanFocused returns true if the [Scan] button is focused.
func (m *ServerListModel) IsScanFocused() bool {
	return m.scanFocused
}

// FocusScan moves focus to the [Scan] button.
func (m *ServerListModel) FocusScan() {
	m.scanFocused = true
}

// SetHasRecent marks that recent servers were loaded (affects auto-select logic).
func (m *ServerListModel) SetHasRecent(has bool) {
	m.hasRecent = has
}

// HasRecent returns true if recent servers were loaded at init.
func (m *ServerListModel) HasRecent() bool {
	return m.hasRecent
}

// SetEntries replaces the server list and sorts it.
func (m *ServerListModel) SetEntries(entries []ServerEntry) {
	// Save selected server ID before replacing
	var selectedID string
	if m.selected >= 0 && m.selected < len(m.entries) {
		e := m.entries[m.selected]
		selectedID = fmt.Sprintf("%s:%d", e.Address, e.Port)
	}

	m.entries = entries
	m.sortEntries()

	// Clamp cursor
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// Restore selected index after sort
	if selectedID != "" {
		m.selected = -1
		for i, e := range m.entries {
			if fmt.Sprintf("%s:%d", e.Address, e.Port) == selectedID {
				m.selected = i
				break
			}
		}
	}
}

// sortEntries sorts: online recent first (by LastConnected desc), then online mDNS (by nickname),
// then offline recent (by LastConnected desc).
func (m *ServerListModel) sortEntries() {
	sort.SliceStable(m.entries, func(i, j int) bool {
		ei, ej := m.entries[i], m.entries[j]
		iOnline := ei.ProbeStatus == ServerProbeOnline
		jOnline := ej.ProbeStatus == ServerProbeOnline

		// Online before offline
		if iOnline != jOnline {
			return iOnline
		}

		// Among same online status: recent before mDNS-only
		iRecent := ei.Source == ServerSourceRecent || ei.Source == ServerSourceBoth
		jRecent := ej.Source == ServerSourceRecent || ej.Source == ServerSourceBoth

		if iOnline {
			// Online group
			if iRecent != jRecent {
				return iRecent
			}
			if iRecent && jRecent {
				return ei.LastConnected.After(ej.LastConnected)
			}
			// Both mDNS-only online: by nickname
			return ei.Hostname < ej.Hostname
		}

		// Offline group: recent first by last_connected desc
		if iRecent != jRecent {
			return iRecent
		}
		if iRecent && jRecent {
			return ei.LastConnected.After(ej.LastConnected)
		}
		return ei.Hostname < ej.Hostname
	})
}

// SelectedEntry returns the currently selected server, or nil if none.
func (m *ServerListModel) SelectedEntry() *ServerEntry {
	if m.selected >= 0 && m.selected < len(m.entries) {
		e := m.entries[m.selected]
		return &e
	}
	return nil
}

// ClearSelection deselects the current server.
func (m *ServerListModel) ClearSelection() {
	m.selected = -1
}

// Select sets the selected and cursor indices to idx (for programmatic selection).
func (m *ServerListModel) Select(idx int) {
	if idx >= 0 && idx < len(m.entries) {
		m.selected = idx
		m.cursor = idx
		m.adjustScroll()
	}
}

// Entries returns the current entries (for external access).
func (m *ServerListModel) Entries() []ServerEntry {
	return m.entries
}

// HasEntries returns true if there are any servers in the list.
func (m *ServerListModel) HasEntries() bool {
	return len(m.entries) > 0
}

// IsServerSelected returns true if a server is selected.
func (m *ServerListModel) IsServerSelected() bool {
	return m.selected >= 0 && m.selected < len(m.entries)
}

// HasProbingServers returns true if any entry is in ServerProbeProbing state.
func (m *ServerListModel) HasProbingServers() bool {
	for _, e := range m.entries {
		if e.ProbeStatus == ServerProbeProbing {
			return true
		}
	}
	return false
}

// AdvanceSpinner increments the spinner frame counter.
func (m *ServerListModel) AdvanceSpinner() {
	m.spinnerFrame++
}

// SetAllProbing sets all entries to ServerProbeProbing status and clears probe results.
func (m *ServerListModel) SetAllProbing() {
	for i := range m.entries {
		m.entries[i].ProbeStatus = ServerProbeProbing
		m.entries[i].ProbeResult = nil
		m.entries[i].ProbeError = nil
	}
}

// UpdateProbeResult updates the entry matching the given address with probe results.
// Returns the updated entry (if found and currently selected) or nil.
func (m *ServerListModel) UpdateProbeResult(address string, result *probe.ProbeServerResult, err error) *ServerEntry {
	for i := range m.entries {
		if m.entries[i].ID() == address {
			if err != nil {
				m.entries[i].ProbeStatus = ServerProbeOffline
				m.entries[i].ProbeError = err
				m.entries[i].ProbeResult = nil
			} else {
				m.entries[i].ProbeStatus = ServerProbeOnline
				m.entries[i].ProbeResult = result
				m.entries[i].ProbeError = nil
				// Store ServerID from probe result if not already set.
				if result != nil && result.ServerID != "" && m.entries[i].ServerID == "" {
					m.entries[i].ServerID = result.ServerID
				}
			}
			m.sortEntries()

			// Check if the updated entry is the selected one after re-sort
			if m.selected >= 0 && m.selected < len(m.entries) && m.entries[m.selected].ID() == address {
				e := m.entries[m.selected]
				return &e
			}
			return nil
		}
	}
	return nil
}

// MergeDiscoveredEntries adds new mDNS entries to the list.
// Deduplication priority:
//  1. By ServerID (if both have one) — recognizes servers that moved to a new IP.
//  2. By address:port — fallback for entries without a ServerID.
//
// Existing entries with matching key are updated to ServerSourceBoth.
// New entries are added with ServerProbeProbing status.
// Returns a slice of newly added entries (for probing).
func (m *ServerListModel) MergeDiscoveredEntries(mdnsEntries []ServerEntry) []ServerEntry {
	// Build lookup maps: by serverID and by addr:port.
	byServerID := make(map[string]int, len(m.entries))
	byAddrPort := make(map[string]int, len(m.entries))
	for i, e := range m.entries {
		if e.ServerID != "" {
			byServerID[e.ServerID] = i
		}
		byAddrPort[e.ID()] = i
	}

	var newEntries []ServerEntry
	for _, me := range mdnsEntries {
		// Find match: prefer ServerID, fall back to addr:port.
		idx := -1
		if me.ServerID != "" {
			if i, ok := byServerID[me.ServerID]; ok {
				idx = i
			}
		}
		if idx < 0 {
			if i, ok := byAddrPort[me.ID()]; ok {
				idx = i
			}
		}

		if idx >= 0 {
			// Merge: keep recent info, mark as Both.
			if m.entries[idx].Source == ServerSourceRecent {
				m.entries[idx].Source = ServerSourceBoth
			}
			// Update address/port from mDNS (server may have moved to a new network).
			m.entries[idx].Address = me.Address
			m.entries[idx].Port = me.Port
			if me.Hostname != "" {
				m.entries[idx].Hostname = me.Hostname
			}
			// Update ServerID if we now know it.
			if me.ServerID != "" && m.entries[idx].ServerID == "" {
				m.entries[idx].ServerID = me.ServerID
			}
		} else {
			me.ProbeStatus = ServerProbeProbing
			m.entries = append(m.entries, me)
			newEntries = append(newEntries, me)
		}
	}
	m.sortEntries()

	// Clamp cursor
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	return newEntries
}

// Update handles navigation keys. Returns updated model and optional command.
func (m ServerListModel) Update(msg tea.Msg) (ServerListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.scanFocused {
		switch keyMsg.Type {
		case tea.KeyEnter, tea.KeySpace:
			return m, func() tea.Msg { return ScanRequestMsg{} }
		case tea.KeyTab:
			m.scanFocused = false
			return m, nil
		case tea.KeyShiftTab:
			m.scanFocused = false
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
			return m, nil
		case tea.KeyUp:
			m.scanFocused = false
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
				return m, nil
			}
			// Empty list — exit server list downward to fields
			return m, func() tea.Msg { return ServerListExitDownMsg{} }
		case tea.KeyDown:
			// From Scan with empty list — exit to fields
			if len(m.entries) == 0 {
				m.scanFocused = false
				return m, func() tea.Msg { return ServerListExitDownMsg{} }
			}
			m.scanFocused = false
			m.cursor = 0
			return m, nil
		case tea.KeyLeft:
			m.scanFocused = false
			return m, nil
		}
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m.adjustScroll()
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.adjustScroll()
			return m, nil
		}
		// At bottom of list — signal exit downward
		return m, func() tea.Msg { return ServerListExitDownMsg{} }
	case tea.KeyEnter:
		if len(m.entries) > 0 && m.cursor < len(m.entries) {
			m.selected = m.cursor
			return m, func() tea.Msg {
				return ServerSelectedMsg{Entry: m.entries[m.selected]}
			}
		}
		return m, nil
	case tea.KeyRight:
		m.scanFocused = true
		return m, nil
	case tea.KeyTab:
		m.scanFocused = true
		return m, nil
	}
	return m, nil
}

// adjustScroll ensures the cursor is visible within the scroll window.
func (m *ServerListModel) adjustScroll() {
	visible := maxVisibleServers
	if len(m.entries) < visible {
		visible = len(m.entries)
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

// View renders the server list component.
// When focused is false, the navigation cursor is hidden (only selection marker ● shown).
func (m ServerListModel) View(width int, focused ...bool) string {
	isFocused := true
	if len(focused) > 0 {
		isFocused = focused[0]
	}
	var b strings.Builder

	// Header: "Servers" + [Scan] button
	title := styles.SetupColumnTitle.Render("Servers")
	scanStyle := styles.SetupActionLabel
	if m.scanFocused {
		scanStyle = styles.SelectedItem
	}
	scanLabel := "[↻ Scan]"
	if !m.scanFocused && isFocused {
		scanLabel = "[↻ Scan] →"
	}
	scanBtn := scanStyle.Render(scanLabel)

	// Right-align scan button
	titleW := lipglossWidth(title)
	scanW := lipglossWidth(scanBtn)
	gap := width - titleW - scanW
	if gap < 1 {
		gap = 1
	}
	b.WriteString(title + strings.Repeat(" ", gap) + scanBtn)
	b.WriteString("\n")

	if len(m.entries) == 0 {
		if m.scanning {
			frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
			frame := frames[m.spinnerFrame%len(frames)]
			b.WriteString(styles.SetupDimValue.Render("  " + string(frame) + " Scanning..."))
		} else {
			b.WriteString(styles.SetupDimValue.Render("  No servers found — enter address manually or [↻ Scan]"))
		}
		b.WriteString("\n")
		b.WriteString(styles.Separator.Render(strings.Repeat("┈", width)))
		return b.String()
	}

	// Determine visible range for scrolling
	visible := maxVisibleServers
	if len(m.entries) <= visible {
		visible = len(m.entries)
	}
	startIdx := m.scrollOffset
	endIdx := startIdx + visible
	if endIdx > len(m.entries) {
		endIdx = len(m.entries)
		startIdx = endIdx - visible
		if startIdx < 0 {
			startIdx = 0
		}
	}

	// Scroll-up indicator
	if startIdx > 0 {
		b.WriteString(styles.SetupDimValue.Render("  ▲"))
		b.WriteString("\n")
	}

	// Render visible entries (2 lines each)
	for i := startIdx; i < endIdx; i++ {
		e := m.entries[i]
		isCursor := i == m.cursor && !m.scanFocused && isFocused
		isSelected := i == m.selected

		// Line 1: marker + nickname + IP:Port
		marker := "○"
		if isSelected {
			marker = "●"
		}
		markerStyle := styles.SetupDimValue
		if isSelected {
			markerStyle = styles.SelectedItem.Bold(true)
		}
		if isCursor {
			markerStyle = styles.SelectedItem
		}

		nick := e.Hostname
		if nick == "" {
			nick = e.ID()
		}
		if len(nick) > 20 {
			nick = nick[:20]
		}

		addr := e.ID()
		nickPad := 24 - len(nick)
		if nickPad < 1 {
			nickPad = 1
		}

		cursor := "  "
		if isCursor {
			cursor = styles.SelectedItem.Render(styles.CursorGlyph + " ")
		}

		// Highlight the entire line for the selected server
		nickStr := nick
		addrStr := styles.SetupDimValue.Render(addr)
		if isSelected && !isCursor {
			nickStr = styles.SelectedItem.Bold(true).Render(nick)
			addrStr = styles.SelectedItem.Render(addr)
		}

		line1 := cursor + markerStyle.Render(marker) + " " + nickStr + strings.Repeat(" ", nickPad) + addrStr
		b.WriteString(line1)
		b.WriteString("\n")

		// Line 2: indented details
		b.WriteString("    ") // 4 spaces indent
		line2 := m.renderEntryDetails(e)
		b.WriteString(line2)
		b.WriteString("\n")
	}

	// Scroll-down indicator
	if endIdx < len(m.entries) {
		b.WriteString(styles.SetupDimValue.Render("  ▼"))
		b.WriteString("\n")
	}

	// Separator
	b.WriteString(styles.Separator.Render(strings.Repeat("┈", width)))

	return b.String()
}

func (m ServerListModel) renderEntryDetails(e ServerEntry) string {
	switch e.ProbeStatus {
	case ServerProbePending, ServerProbeProbing:
		frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
		frame := frames[m.spinnerFrame%len(frames)]
		return styles.SetupDimValue.Render(string(frame) + " probing...")

	case ServerProbeOffline:
		return styles.StatValueError.Render("● offline")

	case ServerProbeOnline:
		if e.ProbeResult == nil {
			return styles.StatValueGood.Render("● online")
		}
		r := e.ProbeResult
		var parts []string

		// TLS icon (only shown when TLS is active)
		if r.TLSRequired {
			if r.TLSSelfSigned {
				parts = append(parts, "🔒⚠")
			} else {
				parts = append(parts, "🔒")
			}
		}

		// Password icon
		if r.PasswordRequired {
			parts = append(parts, "🔑")
		}

		// Mode
		parts = append(parts, r.Mode)

		// Clients
		clientStr := fmt.Sprintf("%d/%d", r.CurrentClients, r.MaxClients)
		if r.CurrentClients >= r.MaxClients {
			clientStr += " ⚠ full"
		}
		parts = append(parts, clientStr)

		return styles.ConnParamValue.Render(strings.Join(parts, " "))
	}

	return ""
}

// lipglossWidth returns the visual width of a styled string (strips ANSI).
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}
