package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func (m Model) updateStreaming(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlP:
			// Pause available for any mode with capture (server normal/duplex, client reverse/duplex, conference)
			if m.pauseCh != nil {
				if m.pauseState != nil {
					// Per-device pause tracking: toggle all devices and check all-paused state.
					// Network notification is sent only when ALL devices transition to paused or
					// when at least one device resumes (from all-paused state).
					wasAllPaused := m.allPausedNotified
					nowAllPaused := m.pauseState.ToggleAll()
					m.paused = nowAllPaused
					if nowAllPaused && !wasAllPaused {
						// All devices just became paused → notify remote
						m.allPausedNotified = true
						select {
						case m.pauseCh <- true:
						default:
						}
					} else if !nowAllPaused && wasAllPaused {
						// At least one device resumed from all-paused state → notify remote
						m.allPausedNotified = false
						select {
						case m.pauseCh <- false:
						default:
						}
					}
					// Per-device pause (partial): no network notification
				} else {
					// No per-device state: simple toggle (legacy/single-device path)
					m.paused = !m.paused
					select {
					case m.pauseCh <- m.paused:
					default:
					}
				}
			}
			return m, nil
		case tea.KeyCtrlE:
			if m.config.AEC {
				m.aecActive = !m.aecActive
				if m.aecToggleFn != nil {
					m.aecToggleFn(m.aecActive)
				}
				return m, nil
			}
		case tea.KeyCtrlR:
			if m.isRecording {
				// Show status overlay (press Enter there to stop).
				m.recordingOverlay.ShowStatus(
					m.recordingDir,
					m.recordingFile,
					m.recordingSize,
					m.recordingStart,
				)
				return m, nil
			}
			// Build source lists based on mode.
			captureDevices, playbackDevices, clients := m.buildRecordingSources()
			m.recordingOverlay.ShowStart(captureDevices, playbackDevices, clients)
			return m, nil
		}

		// Recording overlay keys are now handled in Update() via handleRecordingOverlayKeys.

		// Conference mute-all (alt+m).
		if m.conference && m.muteState != nil && msg.Type == tea.KeyRunes && msg.Alt && msg.String() == "alt+m" {
			return m.handleConferenceMuteAll()
		}

		// Conference participant control keys.
		if m.conference && m.participantCmdCh != nil && len(m.conferenceStates) > 0 {
			idx := m.selectedClient
			if idx >= 0 && idx < len(m.conferenceStates) {
				pid := m.conferenceStates[idx].ID
				switch {
				case msg.Type == tea.KeyEnter:
					ps := m.conferenceStates[idx]
					m.participantOverlay.Show(pid, ps.Muted, pid == "server")
					return m, nil
				case msg.String() == "+" || msg.String() == "=":
					m.participantCmdCh <- ParticipantCommand{Action: ParticipantVolumeUp, ParticipantID: pid}
					return m, nil
				case msg.String() == "-":
					m.participantCmdCh <- ParticipantCommand{Action: ParticipantVolumeDown, ParticipantID: pid}
					return m, nil
				}
			}
		}

		// Mute incoming / mute outgoing for clients is handled via popup (Enter on client list).

		// Device control keys (only when devices panel is available)
		if m.deviceCmdCh != nil && len(m.deviceStates) > 0 {
			switch {
			case msg.Type == tea.KeyCtrlG:
				m.globalMuted = !m.globalMuted
				m.deviceCmdCh <- DeviceCommand{Action: DeviceGlobalMute}
				return m, nil
			case msg.String() == "+" || msg.String() == "=":
				dev := m.deviceStates[m.selectedDevice2]
				dev.Volume += 0.1
				if dev.Volume > 2.0 {
					dev.Volume = 2.0
				}
				m.deviceStates[m.selectedDevice2] = dev
				m.deviceCmdCh <- DeviceCommand{Action: DeviceVolumeUp, DeviceID: dev.ID}
				return m, nil
			case msg.String() == "-":
				dev := m.deviceStates[m.selectedDevice2]
				dev.Volume -= 0.1
				if dev.Volume < 0 {
					dev.Volume = 0
				}
				m.deviceStates[m.selectedDevice2] = dev
				m.deviceCmdCh <- DeviceCommand{Action: DeviceVolumeDown, DeviceID: dev.ID}
				return m, nil
			}
		}

	case StatsUpdateMsg:
		m.stats = msg.Stats

		// Compute instantaneous bitrate (delta bytes × 8 / 1s = kbps).
		// Smooth with EMA (α=0.3) to reduce noise.
		if m.prevBytesSent > 0 || m.prevBytesRecv > 0 {
			const alpha = 0.3
			// Guard against uint64 underflow when stats reset (e.g., new peer connection).
			var deltaSent, deltaRecv float64
			if msg.Stats.BytesSent >= m.prevBytesSent {
				deltaSent = float64(msg.Stats.BytesSent-m.prevBytesSent) * 8 / 1000
			}
			if msg.Stats.BytesRecv >= m.prevBytesRecv {
				deltaRecv = float64(msg.Stats.BytesRecv-m.prevBytesRecv) * 8 / 1000
			}
			m.bitrateUp = alpha*deltaSent + (1-alpha)*m.bitrateUp
			m.bitrateDown = alpha*deltaRecv + (1-alpha)*m.bitrateDown
		}
		m.prevBytesSent = msg.Stats.BytesSent
		m.prevBytesRecv = msg.Stats.BytesRecv

		// Accumulate stats for summary
		m.totalBytesSent = msg.Stats.BytesSent
		m.totalBytesRecv = msg.Stats.BytesRecv
		m.totalPacketsLost = msg.Stats.PacketsLost
		m.jitterSum += msg.Stats.Jitter
		m.rttSum += msg.Stats.RoundTrip
		m.statsCount++

		// Sparkline history
		m.jitterHistory = appendHistory(m.jitterHistory, msg.Stats.Jitter)
		m.rttHistory = appendHistory(m.rttHistory, msg.Stats.RoundTrip)

		if isDisconnectedState(msg.Stats.State) {
			m.screen = ScreenConnection
			m.err = nil
			m.reconnectCount++
			// Server doesn't "reconnect" — it just waits for the next client.
			// Only client shows reconnect attempt counter.
			if m.config.Mode != config.ModeServer {
				m.reconnectAttempt = 1
			}
			// Terminal bell on disconnect
			fmt.Print("\a")
		}
		return m, waitForStats(m.statsCh, m.errCh)

	case ErrorMsg:
		m.stats.State = "disconnected"
		m.err = msg.Err
		m.errorTimer = time.Now().Add(10 * time.Second)
		dismissCmd := tea.Tick(10*time.Second, func(time.Time) tea.Msg { return ErrorDismissMsg{} })
		return m, dismissCmd
	}

	return m, tea.Batch(cmds...)
}

// handleRecordingOverlayKeys intercepts all keys when the recording overlay is visible.
func (m Model) handleParticipantOverlayKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		if m.stopCh != nil {
			m.stopOnce.Do(func() { close(m.stopCh) })
		}
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.participantOverlay.Hide()
	case tea.KeyUp:
		m.participantOverlay.Up()
	case tea.KeyDown:
		m.participantOverlay.Down()
	case tea.KeyEnter:
		action := m.participantOverlay.Confirm()
		pid := m.participantOverlay.ParticipantID
		m.participantOverlay.Hide()

		switch action {
		case views.ParticipantOverlayMute:
			if m.participantCmdCh != nil {
				m.participantCmdCh <- ParticipantCommand{Action: ParticipantMutePersist, ParticipantID: pid}
			}
		case views.ParticipantOverlayUnmute:
			if m.participantCmdCh != nil {
				m.participantCmdCh <- ParticipantCommand{Action: ParticipantUnmutePersist, ParticipantID: pid}
			}
		case views.ParticipantOverlayKick:
			m = m.openKickOverlay(pid, pid)
		case views.ParticipantOverlayBan:
			ip := ""
			hwid := ""
			nick := pid
			for _, c := range m.multiStats.Clients {
				if c.ClientID == pid {
					ip = c.RemoteAddr
					hwid = c.HWID
					if c.Nickname != "" {
						nick = c.Nickname
					}
					break
				}
			}
			m = m.openBanOverlay(pid, nick, ip, hwid)
		}
	}
	return m, nil
}

func (m Model) handleRecordingOverlayKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		if m.stopCh != nil {
			m.stopOnce.Do(func() { close(m.stopCh) })
		}
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.recordingOverlay.Hide()
	case tea.KeyEnter:
		if m.recordingOverlay.IsStatusState() {
			// Stop recording.
			m.isRecording = false
			m.recordingStart = time.Time{}
			m.recordingFile = ""
			m.recordingDir = ""
			m.recordingSize = 0
			if m.recordingCmdCh != nil {
				m.recordingCmdCh <- RecordingCommand{Start: false}
			}
			m.recordingOverlay.Hide()
		} else {
			// Start recording.
			m.isRecording = true
			m.recordingMode = m.recordingOverlay.SelectedMode()
			m.recordingStart = time.Now()
			if m.recordingCmdCh != nil {
				m.recordingCmdCh <- RecordingCommand{
					Start:          true,
					Mode:           m.recordingOverlay.SelectedMode(),
					LocalDeviceIDs: m.recordingOverlay.SelectedLocalDevices(),
					PlaybackIDs:    m.recordingOverlay.SelectedPlaybackDevices(),
					RemoteIDs:      m.recordingOverlay.SelectedRemoteSources(),
				}
			}
			m.recordingOverlay.Hide()
		}
	case tea.KeyUp:
		if m.recordingOverlay.IsStartState() {
			m.recordingOverlay.Up()
		}
	case tea.KeyDown:
		if m.recordingOverlay.IsStartState() {
			m.recordingOverlay.Down()
		}
	case tea.KeySpace:
		if m.recordingOverlay.IsStartState() {
			m.recordingOverlay.Toggle()
		}
	case tea.KeyCtrlA:
		if m.recordingOverlay.IsStartState() {
			m.recordingOverlay.SelectAll()
		}
	case tea.KeyCtrlN:
		if m.recordingOverlay.IsStartState() {
			m.recordingOverlay.SelectNone()
		}
	case tea.KeyCtrlR:
		// Toggle overlay off
		m.recordingOverlay.Hide()
	}
	return m, nil
}

// buildRecordingSources determines which sources to show in the recording popup
// based on the current streaming mode.
func (m Model) buildRecordingSources() (capture, playback, clients []views.RecordingSource) {
	isServer := m.config.Mode == config.ModeServer

	// Build capture device list from device states.
	for _, ds := range m.deviceStates {
		if ds.Role == "capture" {
			capture = append(capture, views.RecordingSource{
				ID:        fmt.Sprintf("%d", ds.ID),
				Name:      ds.Name,
				IsCapture: true,
			})
		}
	}

	// Build playback device list from device states.
	for _, ds := range m.deviceStates {
		if ds.Role == "playback" {
			playback = append(playback, views.RecordingSource{
				ID:   fmt.Sprintf("%d", ds.ID),
				Name: ds.Name,
			})
		}
	}

	// Build client list.
	if m.conference {
		if isServer {
			for _, ps := range m.conferenceStates {
				if ps.ID == "server" {
					continue
				}
				clients = append(clients, views.RecordingSource{
					ID:   ps.ID,
					Name: ps.ID,
				})
			}
		} else {
			clients = append(clients, views.RecordingSource{
				ID:   "incoming",
				Name: "Incoming stream",
			})
		}
	} else if m.multiClient {
		for _, c := range m.multiStats.Clients {
			name := c.ClientID
			if c.Nickname != "" {
				name = c.Nickname
			}
			clients = append(clients, views.RecordingSource{
				ID:   c.ClientID,
				Name: name,
			})
		}
	} else {
		// Single-client mode: the remote peer is one source.
		if isServer {
			clients = append(clients, views.RecordingSource{
				ID:   "client",
				Name: "Client stream",
			})
		} else {
			clients = append(clients, views.RecordingSource{
				ID:   "incoming",
				Name: "Server stream",
			})
		}
	}

	return capture, playback, clients
}

// openBanOverlay builds the reason list and opens the ban overlay for the given client.
func (m Model) openBanOverlay(clientID, nickname, ip, hwid string) Model {
	reasons := []string{"(no reason)"}
	reasons = append(reasons, PredefinedReasons...)

	recentStart := -1
	recent, _ := LoadRecentReasons()
	if len(recent) > 0 {
		recentStart = len(reasons)
		for _, r := range recent {
			reasons = append(reasons, r.Text)
		}
	}

	customStart := len(reasons)
	reasons = append(reasons, "Custom reason...")

	m.overlay = OverlayBan
	m.banClientID = clientID
	m.banClientNick = nickname
	m.banClientIP = ip
	m.banClientHWID = hwid
	m.banCriteriaIP = true // IP selected by default
	m.banCriteriaNick = false
	m.banCriteriaHWID = false
	m.banFocusSection = 0
	m.banCriteriaIndex = 0
	m.banReasons = reasons
	m.banRecentStart = recentStart
	m.banCustomStart = customStart
	m.banSelectedReason = 0
	m.banCustomText = ""
	m.banCustomEditing = false
	m.banButtonFocus = 0
	m.banValidationError = ""
	return m
}

// executeBan sends the ban command with selected criteria and reason, shows a flash message.
func (m Model) executeBan(reason string) (tea.Model, tea.Cmd) {
	// Validate: at least one criterion.
	if !m.banCriteriaIP && !m.banCriteriaNick && !m.banCriteriaHWID {
		m.banValidationError = "Select at least one ban criterion"
		return m, nil
	}
	m.banValidationError = ""

	var criteria []string
	if m.banCriteriaIP {
		criteria = append(criteria, "ip")
	}
	if m.banCriteriaNick {
		criteria = append(criteria, "nickname")
	}
	if m.banCriteriaHWID {
		criteria = append(criteria, "hwid")
	}

	if m.cmdCh != nil {
		m.cmdCh <- ClientCommand{
			Action:      ActionBan,
			ClientID:    m.banClientID,
			Reason:      reason,
			BanCriteria: criteria,
		}
	}
	if reason != "" {
		_ = SaveRecentReason(reason) //nolint:errcheck // best-effort
	}
	nick := m.banClientNick
	if nick == "" {
		nick = m.banClientID
	}
	m.overlay = OverlayNone
	m.flashMsg = nick + " banned"
	m.flashTimer = time.Now().Add(2 * time.Second)
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return views.FlashDismissMsg{} })
}

// banOverlayCriteriaCount returns how many criteria rows are visible.
func (m Model) banOverlayCriteriaCount() int {
	if m.config.HWIDRequired {
		return 3
	}
	return 2
}

// handleBanOverlayKeys handles key events when the ban overlay is active.
func (m Model) handleBanOverlayKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Custom editing mode: capture text input
	if m.banCustomEditing {
		switch msg.Type {
		case tea.KeyEsc:
			m.banCustomEditing = false
			return m, nil
		case tea.KeyEnter:
			text := strings.TrimSpace(m.banCustomText)
			if text == "" {
				return m.executeBan("")
			}
			return m.executeBan(text)
		case tea.KeyBackspace:
			if m.banCustomText != "" {
				m.banCustomText = m.banCustomText[:len(m.banCustomText)-1]
			}
			return m, nil
		default:
			switch msg.Type {
			case tea.KeyRunes:
				m.banCustomText += string(msg.Runes)
			case tea.KeySpace:
				m.banCustomText += " "
			}
			return m, nil
		}
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		return m, nil
	case tea.KeyTab:
		m.banFocusSection = (m.banFocusSection + 1) % 3
		return m, nil
	case tea.KeyShiftTab:
		m.banFocusSection = (m.banFocusSection + 2) % 3
		return m, nil
	case tea.KeyUp:
		switch m.banFocusSection {
		case 0: // criteria
			if m.banCriteriaIndex > 0 {
				m.banCriteriaIndex--
			}
		case 1: // reasons
			if m.banSelectedReason > 0 {
				m.banSelectedReason--
			}
		case 2: // buttons
			if m.banButtonFocus > 0 {
				m.banButtonFocus--
			}
		}
		return m, nil
	case tea.KeyDown:
		switch m.banFocusSection {
		case 0:
			if m.banCriteriaIndex < m.banOverlayCriteriaCount()-1 {
				m.banCriteriaIndex++
			}
		case 1:
			if m.banSelectedReason < len(m.banReasons)-1 {
				m.banSelectedReason++
			}
		case 2:
			if m.banButtonFocus < 1 {
				m.banButtonFocus++
			}
		}
		return m, nil
	case tea.KeySpace:
		if m.banFocusSection == 0 {
			switch m.banCriteriaIndex {
			case 0:
				m.banCriteriaIP = !m.banCriteriaIP
			case 1:
				m.banCriteriaNick = !m.banCriteriaNick
			case 2:
				m.banCriteriaHWID = !m.banCriteriaHWID
			}
			m.banValidationError = "" // clear on change
		}
		return m, nil
	case tea.KeyEnter:
		if m.banFocusSection == 2 {
			if m.banButtonFocus == 1 {
				// Cancel
				m.overlay = OverlayNone
				return m, nil
			}
			// Confirm: get selected reason
			reason := ""
			if m.banSelectedReason > 0 && m.banSelectedReason < len(m.banReasons) && m.banSelectedReason != m.banCustomStart {
				reason = m.banReasons[m.banSelectedReason]
			}
			return m.executeBan(reason)
		}
		if m.banFocusSection == 1 {
			if m.banSelectedReason == m.banCustomStart {
				m.banCustomEditing = true
				m.banCustomText = ""
				return m, nil
			}
			if m.banSelectedReason == 0 {
				return m.executeBan("")
			}
			return m.executeBan(m.banReasons[m.banSelectedReason])
		}
		// Enter in criteria section does nothing (Space toggles)
		return m, nil
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		if m.stopCh != nil {
			m.stopOnce.Do(func() { close(m.stopCh) })
		}
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handleOverlayKeys handles key events when an overlay is active.
func (m Model) handleOverlayKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.overlay == OverlayBanList {
		switch msg.Type {
		case tea.KeyEsc:
			m.overlay = OverlayNone
			return m, nil
		case tea.KeyUp:
			if m.overlaySelection > 0 {
				m.overlaySelection--
			}
			return m, nil
		case tea.KeyDown:
			if m.overlaySelection < len(m.bannedIPs)-1 {
				m.overlaySelection++
			}
			return m, nil
		case tea.KeyEnter:
			if m.overlaySelection < len(m.bannedIPs) && m.cmdCh != nil {
				ip := m.bannedIPs[m.overlaySelection]
				m.cmdCh <- ClientCommand{Action: ActionUnban, IP: ip}
				// Remove IP from local list immediately (backend processes async)
				m.bannedIPs = append(m.bannedIPs[:m.overlaySelection], m.bannedIPs[m.overlaySelection+1:]...)
				if m.overlaySelection >= len(m.bannedIPs) && m.overlaySelection > 0 {
					m.overlaySelection--
				}
				if len(m.bannedIPs) == 0 {
					m.overlay = OverlayNone
				}
			}
			return m, nil
		case tea.KeyCtrlQ, tea.KeyCtrlC:
			if m.stopCh != nil {
				m.stopOnce.Do(func() { close(m.stopCh) })
			}
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// conferenceMaxIndex returns the maximum selectable index for ↑↓ navigation.
// For conference mode it includes server entry (if not hub); for multi-client it's just clients.
func (m Model) conferenceMaxIndex() int {
	if m.conference {
		total := len(m.multiStats.Clients)
		isServer := m.config.Mode == config.ModeServer
		hubMode := isServer && m.serverMuted
		if isServer && !hubMode {
			total++ // server entry
		}
		if total == 0 {
			return 0
		}
		return total - 1
	}
	return len(m.multiStats.Clients) - 1
}

// handleConferenceParticipantMute handles 'm' key on a selected conference participant.
// It prevents muting self ("server" on server side) and updates both MuteState and backend.
func (m Model) handleConferenceParticipantMute(idx int, pid string) (tea.Model, tea.Cmd) { //nolint:unused,unparam // mute feature prepared, pending key-binding integration
	// Prevent muting self — on server, self is "server".
	isServer := m.config.Mode == config.ModeServer
	if isServer && pid == "server" {
		return m, nil
	}

	if m.muteState != nil {
		newMuted := m.muteState.ToggleMute(pid)
		action := ParticipantMutePersist
		if !newMuted {
			action = ParticipantUnmutePersist
		}
		if m.participantCmdCh != nil {
			m.participantCmdCh <- ParticipantCommand{Action: action, ParticipantID: pid}
		}
	} else {
		// Fallback: use conferenceStates directly (legacy path).
		action := ParticipantMutePersist
		if m.conferenceStates[idx].Muted {
			action = ParticipantUnmutePersist
		}
		if m.participantCmdCh != nil {
			m.participantCmdCh <- ParticipantCommand{Action: action, ParticipantID: pid}
		}
	}
	return m, nil
}

// handleConferenceMuteAll handles alt+m — toggles mute-all for conference participants.
func (m Model) handleConferenceMuteAll() (tea.Model, tea.Cmd) {
	if m.muteState == nil {
		return m, nil
	}

	// Collect all non-self participant IDs.
	isServer := m.config.Mode == config.ModeServer
	var ids []string
	for _, s := range m.conferenceStates {
		if isServer && s.ID == "server" {
			continue // skip self
		}
		ids = append(ids, s.ID)
	}

	wasMuteAll := m.muteState.IsMuteAll()
	m.muteState.ToggleMuteAll(ids)
	nowMuteAll := m.muteState.IsMuteAll()

	// Send mute/unmute commands to backend for each participant.
	if m.participantCmdCh != nil {
		if nowMuteAll {
			for _, id := range ids {
				m.participantCmdCh <- ParticipantCommand{Action: ParticipantMutePersist, ParticipantID: id}
			}
		} else if wasMuteAll {
			// Restore: unmute those not in the pre-mute-all muted set.
			mutedMap := m.muteState.MutedMap()
			for _, id := range ids {
				if mutedMap[id] {
					m.participantCmdCh <- ParticipantCommand{Action: ParticipantMutePersist, ParticipantID: id}
				} else {
					m.participantCmdCh <- ParticipantCommand{Action: ParticipantUnmutePersist, ParticipantID: id}
				}
			}
		}
	}

	return m, nil
}

// openClientPopup builds popup menu items based on the current mode and opens the popup overlay.
func (m *Model) openClientPopup(clientID, clientNick string) {
	var items []views.PopupMenuItem

	isDuplex := m.config.Duplex || m.config.Conference
	// Mute outgoing: skip for single-client (normal: duplicates client mute, duplex: duplicates ^P pause).
	showOutgoing := (!m.config.Reverse || isDuplex) && m.multiClient

	if showOutgoing {
		// Multi-client normal, or duplex/conference: show mute outgoing
		isActive := false
		for _, c := range m.multiStats.Clients {
			if c.ClientID == clientID {
				isActive = c.MutedOutgoing
				break
			}
		}
		items = append(items, views.PopupMenuItem{
			Label:    "Mute outgoing",
			Action:   "mute_outgoing",
			IsToggle: true,
			IsActive: isActive,
		})
	}
	if m.config.Reverse || isDuplex {
		// Reverse or duplex/conference: show mute incoming
		isActiveIncoming := false
		for _, c := range m.multiStats.Clients {
			if c.ClientID == clientID {
				isActiveIncoming = c.MutedIncoming
				break
			}
		}
		items = append(items, views.PopupMenuItem{
			Label:    "Mute incoming",
			Action:   "mute_incoming",
			IsToggle: true,
			IsActive: isActiveIncoming,
		})
	}

	items = append(items,
		views.PopupMenuItem{Label: "Kick", Hotkey: "Ctrl+K", Action: "kick"},
		views.PopupMenuItem{Label: "Ban", Hotkey: "Ctrl+B", Action: "ban"},
		views.PopupMenuItem{Label: "Cancel", Hotkey: "Esc", Action: "cancel"},
	)

	m.popupItems = items
	m.popupSelectedIndex = 0
	m.popupClientID = clientID
	m.popupClientNick = clientNick
	m.overlay = OverlayClientPopup
}

// handleClientPopupKeys handles key events when the client popup overlay is visible.
func (m Model) handleClientPopupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		if m.stopCh != nil {
			m.stopOnce.Do(func() { close(m.stopCh) })
		}
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		m.overlay = OverlayNone
		return m, nil
	case tea.KeyUp:
		if m.popupSelectedIndex > 0 {
			m.popupSelectedIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.popupSelectedIndex < len(m.popupItems)-1 {
			m.popupSelectedIndex++
		}
		return m, nil
	case tea.KeyEnter:
		if m.popupSelectedIndex >= 0 && m.popupSelectedIndex < len(m.popupItems) {
			action := m.popupItems[m.popupSelectedIndex].Action
			m.overlay = OverlayNone
			return m.dispatchPopupAction(action)
		}
		return m, nil
	}
	return m, nil
}

// dispatchPopupAction handles the selected popup menu action.
func (m Model) dispatchPopupAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "kick":
		nick := m.popupClientNick
		if nick == "" {
			nick = m.popupClientID
		}
		m = m.openKickOverlay(m.popupClientID, nick)
		return m, nil
	case "ban":
		ip := ""
		hwid := ""
		banNick := m.popupClientNick
		for _, c := range m.multiStats.Clients {
			if c.ClientID == m.popupClientID {
				ip = c.RemoteAddr
				hwid = c.HWID
				if c.Nickname != "" {
					banNick = c.Nickname
				}
				break
			}
		}
		m = m.openBanOverlay(m.popupClientID, banNick, ip, hwid)
		return m, nil
	case "mute_outgoing":
		if m.cmdCh != nil {
			m.cmdCh <- ClientCommand{Action: ActionMuteOutgoing, ClientID: m.popupClientID}
		}
	case "mute_incoming":
		if m.cmdCh != nil {
			m.cmdCh <- ClientCommand{Action: ActionMuteIncoming, ClientID: m.popupClientID}
		}
	case "cancel":
		// Already closed overlay above
	}
	return m, nil
}

// openKickOverlay builds the reason list and opens the kick overlay for the given client.
func (m Model) openKickOverlay(clientID, nickname string) Model {
	reasons := []string{"(no reason)"}
	reasons = append(reasons, PredefinedReasons...)

	recentStart := -1
	recent, _ := LoadRecentReasons()
	if len(recent) > 0 {
		recentStart = len(reasons)
		for _, r := range recent {
			reasons = append(reasons, r.Text)
		}
	}

	customStart := len(reasons)
	reasons = append(reasons, "Custom reason...")

	m.overlay = OverlayKick
	m.kickClientID = clientID
	m.kickClientNick = nickname
	m.kickReasons = reasons
	m.kickRecentStart = recentStart
	m.kickCustomStart = customStart
	m.kickSelectedIndex = 0
	m.kickCustomText = ""
	m.kickCustomEditing = false
	m.kickFocusButton = 0
	return m
}

// executeKick sends the kick command with the given reason and shows a flash message.
func (m Model) executeKick(reason string) (tea.Model, tea.Cmd) {
	if m.cmdCh != nil {
		m.cmdCh <- ClientCommand{Action: ActionKick, ClientID: m.kickClientID, Reason: reason}
	}
	if reason != "" {
		_ = SaveRecentReason(reason) //nolint:errcheck // best-effort
	}
	nick := m.kickClientNick
	if nick == "" {
		nick = m.kickClientID
	}
	m.overlay = OverlayNone
	m.flashMsg = nick + " kicked"
	m.flashTimer = time.Now().Add(2 * time.Second)
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return views.FlashDismissMsg{} })
}

// handleKickOverlayKeys handles key events when the kick overlay is active.
func (m Model) handleKickOverlayKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Custom editing mode: capture text input
	if m.kickCustomEditing {
		switch msg.Type {
		case tea.KeyEsc:
			m.kickCustomEditing = false
			return m, nil
		case tea.KeyEnter:
			text := strings.TrimSpace(m.kickCustomText)
			if text == "" {
				// Enter with empty custom = no reason
				return m.executeKick("")
			}
			return m.executeKick(text)
		case tea.KeyBackspace:
			if m.kickCustomText != "" {
				m.kickCustomText = m.kickCustomText[:len(m.kickCustomText)-1]
			}
			return m, nil
		default:
			switch msg.Type {
			case tea.KeyRunes:
				m.kickCustomText += string(msg.Runes)
			case tea.KeySpace:
				m.kickCustomText += " "
			}
			return m, nil
		}
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = OverlayNone
		return m, nil
	case tea.KeyUp:
		if m.kickFocusButton > 0 {
			m.kickFocusButton--
			if m.kickFocusButton == 0 {
				// Back to list
				m.kickSelectedIndex = len(m.kickReasons) - 1
			}
		} else if m.kickSelectedIndex > 0 {
			m.kickSelectedIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.kickFocusButton == 0 {
			if m.kickSelectedIndex < len(m.kickReasons)-1 {
				m.kickSelectedIndex++
			} else {
				m.kickFocusButton = 1
			}
		} else if m.kickFocusButton < 2 {
			m.kickFocusButton++
		}
		return m, nil
	case tea.KeyTab:
		m.kickFocusButton = (m.kickFocusButton + 1) % 3
		return m, nil
	case tea.KeyEnter:
		if m.kickFocusButton == 2 {
			// Cancel
			m.overlay = OverlayNone
			return m, nil
		}
		if m.kickFocusButton == 1 {
			// [Kick] button: use selected reason
			reason := ""
			if m.kickSelectedIndex > 0 && m.kickSelectedIndex < len(m.kickReasons) && m.kickSelectedIndex != m.kickCustomStart {
				reason = m.kickReasons[m.kickSelectedIndex]
			}
			return m.executeKick(reason)
		}
		// Focus on list
		if m.kickSelectedIndex == m.kickCustomStart {
			// Enter custom editing mode
			m.kickCustomEditing = true
			m.kickCustomText = ""
			return m, nil
		}
		if m.kickSelectedIndex == 0 {
			// (no reason)
			return m.executeKick("")
		}
		// Predefined or recent reason
		return m.executeKick(m.kickReasons[m.kickSelectedIndex])
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		if m.stopCh != nil {
			m.stopOnce.Do(func() { close(m.stopCh) })
		}
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// appendHistory appends a value to a history slice, keeping it at maxHistoryLen.
func appendHistory(history []float64, val float64) []float64 {
	history = append(history, val)
	if len(history) > maxHistoryLen {
		history = history[len(history)-maxHistoryLen:]
	}
	return history
}

// isConnectedState returns true when the WebRTC state indicates an active connection.
func isConnectedState(state string) bool {
	return strings.EqualFold(state, "connected")
}

// isDisconnectedState returns true when the WebRTC state indicates a terminated connection.
func isDisconnectedState(state string) bool {
	s := strings.ToLower(state)
	return s == "closed" || s == "failed" || s == "disconnected"
}

func (m Model) updateServerStopped(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	const numButtons = 3 // Reconnect, Settings, Quit
	if msg, ok := msg.(tea.KeyMsg); ok {
		// Critical changes overlay is visible — handle Enter/Esc.
		if len(m.criticalChanges) > 0 {
			switch msg.Type {
			case tea.KeyEnter:
				m.criticalChanges = nil
				m.screen = ScreenDeviceSelect
				return m, tea.Batch(cmds...)
			case tea.KeyEsc:
				if m.stopCh != nil && m.stopOnce != nil {
					m.stopOnce.Do(func() { close(m.stopCh) })
				}
				m.quitting = true
				return m, tea.Quit
			}
			return m, tea.Batch(cmds...)
		}
		switch msg.String() {
		case "left", "h":
			if m.kickBanBtnFocus > 0 {
				m.kickBanBtnFocus--
			}
		case "right", "l":
			if m.kickBanBtnFocus < numButtons-1 {
				m.kickBanBtnFocus++
			}
		case "enter":
			switch m.kickBanBtnFocus {
			case 0: // Reconnect
				m.kickBanBtnFocus = 0
				if m.reconnectProbing {
					return m, tea.Batch(cmds...)
				}
				m.reconnectProbing = true
				return m, startReconnectProbe(m.savedCfg.Address, m.savedCfg.Port)
			case 1: // Settings
				m.kickBanBtnFocus = 0
				if m.stopCh != nil && m.stopOnce != nil {
					m.stopOnce.Do(func() { close(m.stopCh) })
				}
				m.screen = ScreenDeviceSelect
				return m, nil
			case 2: // Quit
				m.quitting = true
				return m, tea.Quit
			}
		case "ctrl+q":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, tea.Batch(cmds...)
}

// applyNonCriticalChanges updates savedCfg with non-critical parameter changes and logs them.
func applyNonCriticalChanges(m *Model, changes []views.ParamChange) {
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		switch c.Field {
		case "SampleRate":
			var v uint32
			_, _ = fmt.Sscanf(c.NewValue, "%d", &v)
			m.savedCfg.SampleRate = v
		case "Channels":
			var v uint32
			_, _ = fmt.Sscanf(c.NewValue, "%d", &v)
			m.savedCfg.Channels = v
		case "OpusBitrate":
			var v int
			_, _ = fmt.Sscanf(c.NewValue, "%d", &v)
			m.savedCfg.OpusBitrate = v
		case "MaxClients":
			var v int
			_, _ = fmt.Sscanf(c.NewValue, "%d", &v)
			m.savedCfg.MaxClients = v
		case "HWID":
			m.savedCfg.HWIDRequired = (c.NewValue == "on")
		}
		parts = append(parts, fmt.Sprintf("%s %s→%s", c.Field, c.OldValue, c.NewValue))
	}
	if len(parts) > 0 {
		logMsg := "[INF] Server settings changed: " + strings.Join(parts, ", ")
		m.logs = append(m.logs, logMsg)
		if len(m.logs) > maxLogs {
			m.logs = m.logs[len(m.logs)-maxLogs:]
		}
	}
}

// startReconnection initiates a new connection using savedCfg, same as initial connect from setup.
func (m Model) startReconnection() (tea.Model, tea.Cmd) {
	if m.startFunc == nil {
		m.quitting = true
		return m, tea.Quit
	}
	m.screen = ScreenConnection
	m.stats = transport.ConnectionStats{}
	m.startTime = time.Time{}
	cfg := m.savedCfg
	startFunc := m.startFunc
	m.stopCh = make(chan struct{})
	m.stopOnce = &sync.Once{}
	stopCh := m.stopCh
	var batchCmds []tea.Cmd
	batchCmds = append(batchCmds, m.spinner.Tick, func() tea.Msg {
		statsCh, errCh, srvStoppedCh := startFunc(cfg, stopCh)
		return streamingStartedMsg{statsCh: statsCh, errCh: errCh, serverStoppedCh: srvStoppedCh}
	})
	if m.logCh != nil {
		batchCmds = append(batchCmds, waitForLog(m.logCh))
	}
	return m, tea.Batch(batchCmds...)
}

// openDeviceOverlay initializes and opens the device overlay.
func (m *Model) openDeviceOverlay() {
	m.deviceOverlaySection = 0
	m.deviceOverlayIndex = 0
	// Start on first section that has devices.
	hasInput := false
	for _, ds := range m.deviceStates {
		if ds.Role == "capture" {
			hasInput = true
			break
		}
	}
	if !hasInput {
		m.deviceOverlaySection = 1
	}
	m.overlay = OverlayDevice
}

// handleDeviceOverlayKeys handles key events when the device overlay is visible.
func (m Model) handleDeviceOverlayKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	inputs, outputs := m.splitDevicesByRole()

	currentList := inputs
	if m.deviceOverlaySection == 1 {
		currentList = outputs
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlD:
		m.overlay = OverlayNone
		return m, nil
	case tea.KeyTab:
		// Toggle between input and output sections.
		if m.deviceOverlaySection == 0 && len(outputs) > 0 {
			m.deviceOverlaySection = 1
			m.deviceOverlayIndex = 0
		} else if m.deviceOverlaySection == 1 && len(inputs) > 0 {
			m.deviceOverlaySection = 0
			m.deviceOverlayIndex = 0
		}
		return m, nil
	case tea.KeyUp:
		if m.deviceOverlayIndex > 0 {
			m.deviceOverlayIndex--
		}
		return m, nil
	case tea.KeyDown:
		if m.deviceOverlayIndex < len(currentList)-1 {
			m.deviceOverlayIndex++
		}
		return m, nil
	case tea.KeyEnter:
		if m.deviceOverlayIndex < len(currentList) && m.deviceCmdCh != nil {
			ds := currentList[m.deviceOverlayIndex]
			m.deviceCmdCh <- DeviceCommand{Action: DeviceToggleMute, DeviceID: ds.ID}
			// Update local state immediately for responsive UI.
			for i := range m.deviceStates {
				if m.deviceStates[i].ID == ds.ID {
					m.deviceStates[i].Muted = !m.deviceStates[i].Muted
					break
				}
			}
		}
		return m, nil
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		if m.stopCh != nil {
			m.stopOnce.Do(func() { close(m.stopCh) })
		}
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// splitDevicesByRole splits deviceStates into capture (input) and playback (output) slices.
func (m *Model) splitDevicesByRole() (inputs, outputs []DeviceState) {
	for _, ds := range m.deviceStates {
		if ds.Role == "capture" {
			inputs = append(inputs, ds)
		} else {
			outputs = append(outputs, ds)
		}
	}
	return
}
