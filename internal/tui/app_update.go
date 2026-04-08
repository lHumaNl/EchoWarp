package tui

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		return m, doTick()

	case spectrumTickMsg:
		if m.screen != ScreenStreaming {
			m.spectrumTickActive = false
			return m, nil // stop tick when not streaming
		}
		if m.spectrumFunc != nil {
			m.spectrumBands = m.spectrumFunc()
		}
		if m.levelFunc != nil {
			m.vuLevels = m.levelFunc()
		}
		if m.captureSpectrumFunc != nil {
			m.captureSpectrumBands = m.captureSpectrumFunc()
		}
		if m.captureLevelFunc != nil {
			m.captureVULevels = m.captureLevelFunc()
		}
		return m, doSpectrumTick()

	case LogMsg:
		line := msg.Entry.Format()
		m.logs = append(m.logs, line)
		if len(m.logs) > maxLogs {
			m.logs = m.logs[len(m.logs)-maxLogs:]
		}
		// If user was at auto-scroll, stay there. Otherwise keep offset.
		if m.logCh != nil {
			return m, waitForLog(m.logCh)
		}
		return m, nil

	case tea.KeyMsg:
		// Chat input focus intercept — when chat is focused, consume all keys except Ctrl+Q/Ctrl+C/Ctrl+T/Tab/Esc.
		if m.screen == ScreenStreaming && m.chatPanel.IsFocused() {
			switch msg.Type {
			case tea.KeyCtrlQ, tea.KeyCtrlC:
				// Allow quit through
			case tea.KeyCtrlT:
				// Toggle chat visibility
				m.chatPanel.Unfocus()
				m.chatPanel.ToggleVisible()
				if m.config.Mode == config.ModeServer {
					m.focusedArea = FocusClientList
				} else {
					m.focusedArea = FocusLogs
				}
				return m, nil
			case tea.KeyTab:
				// If input starts with @, use Tab for autocomplete; otherwise cycle focus.
				if strings.HasPrefix(m.chatPanel.InputValue(), "@") {
					m.chatPanel, _ = m.chatPanel.Update(msg)
					return m, nil
				}
				m.chatPanel.Unfocus()
				m.focusedArea = m.nextFocusArea()
				return m, nil
			case tea.KeyEsc:
				// Esc unfocuses chat, returns to previous area
				m.chatPanel.Unfocus()
				if m.config.Mode == config.ModeServer {
					m.focusedArea = FocusClientList
				} else {
					m.focusedArea = FocusLogs
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.chatPanel, cmd = m.chatPanel.Update(msg)
				return m, cmd
			}
		}

		// Ctrl+T: chat visible+unfocused → focus; chat focused → hide; chat hidden → show+focus.
		if m.screen == ScreenStreaming && msg.Type == tea.KeyCtrlT {
			if m.chatPanel.IsVisible() && m.focusedArea == FocusChat {
				// Already focused — hide chat
				m.chatPanel.Unfocus()
				m.chatPanel.ToggleVisible()
				if m.config.Mode == config.ModeServer {
					m.focusedArea = FocusClientList
				} else {
					m.focusedArea = FocusLogs
				}
			} else if m.chatPanel.IsVisible() {
				// Visible but not focused — focus it
				m.chatPanel.Focus()
				m.focusedArea = FocusChat
			} else {
				// Hidden — show and focus
				m.chatPanel.ToggleVisible()
				m.chatPanel.Focus()
				m.focusedArea = FocusChat
			}
			return m, nil
		}

		// Client popup overlay intercept.
		if m.overlay == OverlayClientPopup {
			return m.handleClientPopupKeys(msg)
		}

		// Kick overlay intercept
		if m.overlay == OverlayKick {
			return m.handleKickOverlayKeys(msg)
		}

		// Ban overlay intercept
		if m.overlay == OverlayBan {
			return m.handleBanOverlayKeys(msg)
		}

		// Device overlay intercept
		if m.overlay == OverlayDevice {
			return m.handleDeviceOverlayKeys(msg)
		}

		// Overlay key handling
		if m.overlay != OverlayNone {
			return m.handleOverlayKeys(msg)
		}

		// Recording overlay intercept — must be before Up/Down/Tab handling below.
		if m.recordingOverlay.Visible {
			return m.handleRecordingOverlayKeys(msg)
		}

		// Participant action overlay intercept.
		if m.participantOverlay.Visible {
			return m.handleParticipantOverlayKeys(msg)
		}

		switch msg.Type {
		case tea.KeyCtrlQ, tea.KeyCtrlC:
			if m.stopCh != nil && m.stopOnce != nil {
				m.stopOnce.Do(func() { close(m.stopCh) })
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			// Clear error banner on Esc (UX-11)
			if m.err != nil {
				m.err = nil
				m.errorTimer = time.Time{}
				return m, nil
			}
		case tea.KeyCtrlL:
			if m.screen == ScreenStreaming {
				m.logsVisible = !m.logsVisible
				return m, nil
			}
		case tea.KeyCtrlD:
			// Open device overlay (only if devices are available)
			if m.screen == ScreenStreaming && len(m.deviceStates) > 0 {
				m.openDeviceOverlay()
				return m, nil
			}
		case tea.KeyTab:
			if m.screen == ScreenStreaming {
				m.focusedArea = m.nextFocusArea()
				// If switching to chat, focus the chat input
				if m.focusedArea == FocusChat {
					m.chatPanel.Focus()
				} else {
					m.chatPanel.Unfocus()
				}
				return m, nil
			}
		case tea.KeyUp:
			if m.screen == ScreenStreaming && !m.chatPanel.IsFocused() {
				switch m.focusedArea {
				case FocusClientList:
					if m.multiClient {
						if m.selectedClient > 0 {
							m.selectedClient--
						}
						if !m.conference && m.selectedClient < len(m.multiStats.Clients) {
							m.selectedClientID = m.multiStats.Clients[m.selectedClient].ClientID
						}
					} else if len(m.deviceStates) > 0 {
						if m.selectedDevice2 > 0 {
							m.selectedDevice2--
						}
					}
				case FocusLogs:
					if m.logsVisible {
						m.logScrollOffset++
						maxScroll := len(m.logs) - 3
						if m.logScrollOffset > maxScroll {
							m.logScrollOffset = maxScroll
						}
						if m.logScrollOffset < 0 {
							m.logScrollOffset = 0
						}
					}
				}
				return m, nil
			}
		case tea.KeyDown:
			if m.screen == ScreenStreaming && !m.chatPanel.IsFocused() {
				switch m.focusedArea {
				case FocusClientList:
					if m.multiClient {
						maxIdx := m.conferenceMaxIndex()
						if m.selectedClient < maxIdx {
							m.selectedClient++
						}
						if !m.conference && m.selectedClient < len(m.multiStats.Clients) {
							m.selectedClientID = m.multiStats.Clients[m.selectedClient].ClientID
						}
					} else if len(m.deviceStates) > 0 {
						if m.selectedDevice2 < len(m.deviceStates)-1 {
							m.selectedDevice2++
						}
					}
				case FocusLogs:
					if m.logsVisible {
						m.logScrollOffset--
						if m.logScrollOffset < 0 {
							m.logScrollOffset = 0
						}
					}
				}
				return m, nil
			}
		case tea.KeyCtrlU:
			// Open ban list overlay — available on all screens for server
			if m.config.Mode == config.ModeServer && m.banListFn != nil {
				m.overlay = OverlayBanList
				m.overlaySelection = 0
				m.bannedIPs = m.banListFn()
				return m, nil
			}
		case tea.KeyEnter:
			// Server mode: open client popup menu when client list is focused (non-conference)
			if m.screen == ScreenStreaming && m.config.Mode == config.ModeServer && m.focusedArea == FocusClientList && !m.conference && m.cmdCh != nil {
				if len(m.multiStats.Clients) > 0 && m.selectedClient < len(m.multiStats.Clients) {
					c := m.multiStats.Clients[m.selectedClient]
					nick := c.Nickname
					if nick == "" {
						nick = c.ClientID
					}
					m.openClientPopup(c.ClientID, nick)
					return m, nil
				}
			}
			// Client mode: Enter = mute/pause (non-conference, chat not focused)
			if m.screen == ScreenStreaming && m.config.Mode == config.ModeClient && !m.conference && !m.chatPanel.IsFocused() {
				if m.config.Reverse && !m.config.Duplex {
					// Reverse-only: toggle pause capture (stop sending)
					if m.pauseCh != nil {
						m.paused = !m.paused
						select {
						case m.pauseCh <- m.paused:
						default:
						}
					}
				} else {
					// Normal or duplex: toggle server mute (stop hearing server)
					if m.serverMuteCh != nil {
						m.serverMuted = !m.serverMuted
						select {
						case m.serverMuteCh <- m.serverMuted:
						default:
						}
					}
				}
				return m, nil
			}
		default:
		}

	case DeviceChangeMsg:
		// Mark devices as disconnected or add back
		if !msg.Added {
			for i := range m.deviceStates {
				if m.deviceStates[i].Name == msg.Name {
					m.deviceStates[i].Disconnected = true
				}
			}
		} else {
			for i := range m.deviceStates {
				if m.deviceStates[i].Name == msg.Name {
					m.deviceStates[i].Disconnected = false
				}
			}
		}
		if m.deviceChangeCh != nil {
			return m, waitForDeviceChange(m.deviceChangeCh)
		}
		return m, nil

	case MultiStatsUpdateMsg:
		m.multiStats = msg.Stats
		if m.screen == ScreenConnection && len(msg.Stats.Clients) > 0 {
			m.screen = ScreenStreaming
			m.setDefaultFocus()
			if m.startTime.IsZero() {
				m.startTime = time.Now()
			}
		}

		// Sync client nicknames to chat panel for @-autocomplete (server side).
		if m.config.Mode == config.ModeServer {
			nicks := make([]string, 0, len(msg.Stats.Clients))
			for _, c := range msg.Stats.Clients {
				if c.Nickname != "" {
					nicks = append(nicks, c.Nickname)
				} else {
					nicks = append(nicks, c.ClientID)
				}
			}
			m.chatPanel.SetParticipants(nicks)
		}

		// Initialize per-client history maps if nil.
		if m.perClientJitterHistory == nil {
			m.perClientJitterHistory = make(map[string][]float64)
			m.perClientRTTHistory = make(map[string][]float64)
			m.perClientBitrateUp = make(map[string]float64)
			m.perClientBitrateDown = make(map[string]float64)
			m.perClientPrevBytesSent = make(map[string]uint64)
			m.perClientPrevBytesRecv = make(map[string]uint64)
		}

		// Build set of active client IDs.
		activeIDs := make(map[string]struct{}, len(msg.Stats.Clients))
		const emaAlpha = 0.3
		for _, ci := range msg.Stats.Clients {
			activeIDs[ci.ClientID] = struct{}{}

			// Append jitter history, cap at maxHistoryLen.
			m.perClientJitterHistory[ci.ClientID] = append(m.perClientJitterHistory[ci.ClientID], ci.Jitter)
			if len(m.perClientJitterHistory[ci.ClientID]) > maxHistoryLen {
				m.perClientJitterHistory[ci.ClientID] = m.perClientJitterHistory[ci.ClientID][len(m.perClientJitterHistory[ci.ClientID])-maxHistoryLen:]
			}

			// Append RTT history, cap at maxHistoryLen.
			m.perClientRTTHistory[ci.ClientID] = append(m.perClientRTTHistory[ci.ClientID], ci.RoundTrip)
			if len(m.perClientRTTHistory[ci.ClientID]) > maxHistoryLen {
				m.perClientRTTHistory[ci.ClientID] = m.perClientRTTHistory[ci.ClientID][len(m.perClientRTTHistory[ci.ClientID])-maxHistoryLen:]
			}

			// Calculate EMA bitrate (kbps), alpha=0.3.
			prevSent := m.perClientPrevBytesSent[ci.ClientID]
			prevRecv := m.perClientPrevBytesRecv[ci.ClientID]
			if prevSent > 0 || prevRecv > 0 {
				deltaSent := float64(ci.BytesSent-prevSent) * 8.0 / 1000.0
				deltaRecv := float64(ci.BytesRecv-prevRecv) * 8.0 / 1000.0
				m.perClientBitrateUp[ci.ClientID] = emaAlpha*deltaSent + (1-emaAlpha)*m.perClientBitrateUp[ci.ClientID]
				m.perClientBitrateDown[ci.ClientID] = emaAlpha*deltaRecv + (1-emaAlpha)*m.perClientBitrateDown[ci.ClientID]
			}
			m.perClientPrevBytesSent[ci.ClientID] = ci.BytesSent
			m.perClientPrevBytesRecv[ci.ClientID] = ci.BytesRecv
		}

		// Clean up disconnected clients.
		for id := range m.perClientJitterHistory {
			if _, ok := activeIDs[id]; ok {
				continue
			}
			delete(m.perClientJitterHistory, id)
			delete(m.perClientRTTHistory, id)
			delete(m.perClientBitrateUp, id)
			delete(m.perClientBitrateDown, id)
			delete(m.perClientPrevBytesSent, id)
			delete(m.perClientPrevBytesRecv, id)
		}

		// Restore selectedClient by ClientID if we had a stable selection.
		if m.selectedClientID != "" {
			found := false
			for i, ci := range msg.Stats.Clients {
				if ci.ClientID == m.selectedClientID {
					m.selectedClient = i
					found = true
					break
				}
			}
			if !found {
				// Client disconnected — clamp index to valid range.
				if len(msg.Stats.Clients) == 0 {
					m.selectedClient = 0
					m.selectedClientID = ""
				} else {
					if m.selectedClient >= len(msg.Stats.Clients) {
						m.selectedClient = len(msg.Stats.Clients) - 1
					}
					m.selectedClientID = msg.Stats.Clients[m.selectedClient].ClientID
				}
			}
		} else {
			// No stable ID yet — just clamp.
			if len(msg.Stats.Clients) == 0 {
				m.selectedClient = 0
			} else if m.selectedClient >= len(msg.Stats.Clients) {
				m.selectedClient = len(msg.Stats.Clients) - 1
			}
		}

		var nextCmd tea.Cmd
		if m.multiStatsCh != nil {
			nextCmd = waitForMultiStats(m.multiStatsCh)
		}
		if m.screen == ScreenStreaming && !m.spectrumTickActive {
			m.spectrumTickActive = true
			return m, tea.Batch(nextCmd, doSpectrumTick())
		}
		return m, nextCmd

	case ParticipantsUpdateMsg:
		m.participants = msg.Participants
		m.maxClients = msg.MaxClients
		m.chatPanel.SetParticipants(msg.Participants)
		return m, waitForParticipants(m.participantsCh)

	case ParticipantPauseMsg:
		if m.pausedParticipants == nil {
			m.pausedParticipants = make(map[string]bool)
		}
		if msg.Paused {
			m.pausedParticipants[msg.ParticipantID] = true
		} else {
			delete(m.pausedParticipants, msg.ParticipantID)
		}
		return m, waitForParticipantPause(m.participantPauseCh)

	case ConferenceParticipantsMsg:
		if m.pausedParticipants == nil {
			m.pausedParticipants = make(map[string]bool)
		}
		// Reset and rebuild from full update.
		for k := range m.pausedParticipants {
			delete(m.pausedParticipants, k)
		}
		for _, p := range msg.Participants {
			if p.Paused {
				m.pausedParticipants[p.ID] = true
			}
		}
		return m, waitForConferenceParticipants(m.conferencePartsCh)

	case ConferenceStatsMsg:
		m.conferenceStates = msg.States
		// Sync recording state — detects auto-stop from disk errors.
		if m.isRecording && !msg.Recording {
			m.isRecording = false
			m.recordingStart = time.Time{}
		}
		// Sync paused participants from conference handler (server-side).
		if msg.PausedParticipants != nil {
			m.pausedParticipants = msg.PausedParticipants
		}
		return m, waitForConferenceStats(m.conferenceStatsCh)

	case chatNicknameMsg:
		nick := string(msg)
		m.chatPanel.SetMyNickname(nick)
		return m, nil

	case ChatReceivedMsg:
		m.chatPanel.AddMessage(msg.Message)
		return m, waitForChat(m.chatMsgCh)

	case RecordingStatusUpdate:
		m.recordingDir = msg.Dir
		m.recordingFile = msg.FileName
		m.recordingSize = msg.Size
		// Update status overlay if it's visible.
		if m.recordingOverlay.Visible && m.recordingOverlay.IsStatusState() {
			m.recordingOverlay.UpdateStatus(msg.Size)
		}
		return m, nil

	case views.ChatSendMsg:
		if m.chatSendFn != nil && msg.Text != "" {
			m.chatSendFn(msg.Text, msg.To)
		}
		return m, nil

	case ErrorDismissMsg:
		if !m.errorTimer.IsZero() && time.Now().After(m.errorTimer) {
			m.err = nil
			m.errorTimer = time.Time{}
		}
		return m, nil

	case views.FlashDismissMsg:
		if !m.flashTimer.IsZero() && time.Now().After(m.flashTimer) {
			m.flashMsg = ""
			m.flashTimer = time.Time{}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.deviceList.SetSize(msg.Width, msg.Height-8)
		m.setupModel.SetSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case streamingStartedMsg:
		m.statsCh = msg.statsCh
		m.errCh = msg.errCh
		if msg.serverStoppedCh != nil {
			m.serverStoppedCh = msg.serverStoppedCh
		}
		// Snapshot config for reconnection (kick/ban screens use savedCfg).
		m.savedCfg = m.config

		batchCmds := []tea.Cmd{waitForStats(m.statsCh, m.errCh)}

		// Save recent server (client mode, non-blocking)
		if m.config.Mode == config.ModeClient {
			cfg := m.config
			probeRes := m.setupModel.ProbeResult()
			selServer := m.setupModel.SelectedServer()
			devicePreset := m.setupModel.CollectPresetDevices()
			mode := cfg.AudioMode()
			batchCmds = append(batchCmds, saveRecentServerCmd(cfg, probeRes, selServer, devicePreset, mode))
		}
		// Save server-side preset and generate server UUID (server mode, non-blocking)
		if m.config.Mode == config.ModeServer {
			devicePreset := m.setupModel.CollectPresetDevices()
			serverSettings := m.setupModel.CollectServerSettings()
			mode := m.config.AudioMode()
			port := m.config.Port
			batchCmds = append(batchCmds, saveServerPresetCmd(devicePreset, mode, serverSettings), func() tea.Msg {
				_ = config.ServerID(port) // generate & persist UUID for this port
				return nil
			})
		}
		if m.logCh != nil {
			batchCmds = append(batchCmds, waitForLog(m.logCh))
		}
		if m.multiStatsCh != nil {
			batchCmds = append(batchCmds, waitForMultiStats(m.multiStatsCh))
		}
		if m.deviceChangeCh != nil {
			batchCmds = append(batchCmds, waitForDeviceChange(m.deviceChangeCh))
		}
		if m.conference && m.conferenceStatsCh != nil {
			batchCmds = append(batchCmds, waitForConferenceStats(m.conferenceStatsCh))
		}
		if m.chatMsgCh != nil {
			batchCmds = append(batchCmds, waitForChat(m.chatMsgCh))
		}
		if m.chatNicknameCh != nil {
			batchCmds = append(batchCmds, waitForChatNickname(m.chatNicknameCh))
		}
		if m.participantsCh != nil {
			batchCmds = append(batchCmds, waitForParticipants(m.participantsCh))
		}
		if m.serverStoppedCh != nil {
			batchCmds = append(batchCmds, waitForServerStopped(m.serverStoppedCh))
		}
		if m.participantPauseCh != nil {
			batchCmds = append(batchCmds, waitForParticipantPause(m.participantPauseCh))
		}
		if m.conferencePartsCh != nil {
			batchCmds = append(batchCmds, waitForConferenceParticipants(m.conferencePartsCh))
		}
		return m, tea.Batch(batchCmds...)

	case streamingEndedMsg:
		m.cleanupVirtualSinks()
		m.quitting = true
		return m, tea.Quit

	case ServerStoppedMsg:
		// Server sent ActionStop — clean up virtual sinks and transition.
		m.cleanupVirtualSinks()
		m.screen = ScreenServerStopped
		m.autoReconnect = m.config.AutoReconnect
		m.autoReconnectAttempts = m.config.AutoReconnectAttempts
		if m.autoReconnect {
			m.reconnectAttemptGS = 1
		} else {
			m.reconnectAttemptGS = 0
		}
		m.reconnectProbing = false
		m.reconnectLastError = ""
		m.criticalChanges = nil
		m.savedCfg = m.config
		if m.autoReconnect {
			m.reconnectCountdown = 10
			return m, doReconnectTick()
		}
		return m, nil

	case ReconnectTickMsg:
		if m.screen != ScreenServerStopped || !m.autoReconnect {
			return m, nil
		}
		m.reconnectCountdown--
		if m.reconnectCountdown <= 0 {
			// Countdown expired — trigger probe.
			m.reconnectProbing = true
			return m, startReconnectProbe(m.savedCfg.Address, m.savedCfg.Port)
		}
		return m, doReconnectTick()

	case ReconnectProbeMsg:
		if m.screen != ScreenServerStopped {
			return m, nil
		}
		m.reconnectProbing = false
		if msg.Err != nil {
			// Probe failed.
			m.reconnectLastError = msg.Err.Error()
			m.reconnectAttemptGS++
			if m.autoReconnect {
				// Check max attempts.
				if m.autoReconnectAttempts > 0 && m.reconnectAttemptGS >= m.autoReconnectAttempts {
					m.autoReconnect = false
					return m, nil
				}
				m.reconnectCountdown = 10
				return m, doReconnectTick()
			}
			return m, nil
		}
		// Probe succeeded — compare parameters.
		changes := views.CompareServerParams(m.savedCfg, msg.Result)
		var criticals []views.ParamChange
		var nonCriticals []views.ParamChange
		for _, c := range changes {
			if c.Critical {
				criticals = append(criticals, c)
			} else {
				nonCriticals = append(nonCriticals, c)
			}
		}
		if len(criticals) > 0 {
			m.criticalChanges = criticals
			m.autoReconnect = false // stop auto-reconnect timer
			return m, nil
		}
		// Auto-apply non-critical changes.
		if len(nonCriticals) > 0 {
			applyNonCriticalChanges(&m, nonCriticals)
		}
		// Proceed to connect.
		return m.startReconnection()
	}

	switch m.screen {
	case ScreenDeviceSelect:
		return m.updateDeviceSelect(msg, cmds)
	case ScreenConnection:
		return m.updateConnection(msg, cmds)
	case ScreenStreaming:
		return m.updateStreaming(msg, cmds)
	case ScreenServerStopped:
		return m.updateServerStopped(msg, cmds)
	case ScreenKicked:
		return m.updateKicked(msg, cmds)
	case ScreenBanned:
		return m.updateBanned(msg, cmds)
	}

	return m, tea.Batch(cmds...)
}

// updateKicked handles key events on the ScreenKicked screen.
// Buttons: [Reconnect] [Settings] [Quit]
func (m Model) updateKicked(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	const numButtons = 3
	if msg, ok := msg.(tea.KeyMsg); ok {
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
				m.kickedReason = ""
				m.kickBanBtnFocus = 0
				return m.startReconnection()
			case 1: // Settings
				m.kickedReason = ""
				m.kickBanBtnFocus = 0
				// Cancel any old connection context before going to settings.
				if m.stopCh != nil {
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

// updateBanned handles key events on the ScreenBanned screen.
// Buttons: [Settings] [Quit]
func (m Model) updateBanned(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	const numButtons = 2
	if msg, ok := msg.(tea.KeyMsg); ok {
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
			case 0: // Settings
				m.bannedReason = ""
				m.bannedCriteria = nil
				m.kickBanBtnFocus = 0
				// Cancel any old connection context before going to settings.
				if m.stopCh != nil {
					m.stopOnce.Do(func() { close(m.stopCh) })
				}
				m.screen = ScreenDeviceSelect
				return m, nil
			case 1: // Quit
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

func (m Model) updateDeviceSelect(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	// Handle PreStartProbeMsg — result of pre-start probe for client mode.
	if probeMsg, ok := msg.(PreStartProbeMsg); ok {
		if m.pendingStartCfg == nil {
			return m, tea.Batch(cmds...) // stale message, ignore
		}
		if probeMsg.Err != nil {
			// Probe failed — show error, stay on setup screen.
			m.pendingStartCfg = nil
			flashCmd := m.setupModel.SetFlash("Server unreachable — check address and port", 5*time.Second)
			cmds = append(cmds, flashCmd)
			return m, tea.Batch(cmds...)
		}
		// Compare probe result with pending config.
		pendingCfg := *m.pendingStartCfg
		changes := views.CompareServerParams(pendingCfg, probeMsg.Result)
		var criticals []views.ParamChange
		var nonCriticals []views.ParamChange
		for _, c := range changes {
			if c.Critical {
				criticals = append(criticals, c)
			} else {
				nonCriticals = append(nonCriticals, c)
			}
		}
		if len(criticals) > 0 {
			// Critical changes — update setup fields from probe, show flash, stay on setup.
			m.pendingStartCfg = nil
			m.setupModel.Fields, m.setupModel.AdvancedFields, _ = views.ApplyProbeResult(
				m.setupModel.Fields, m.setupModel.AdvancedFields, probeMsg.Result,
			)
			flashCmd := m.setupModel.SetFlash("\u26a0 Server config changed, fields updated — please review", 5*time.Second)
			cmds = append(cmds, flashCmd)
			return m, tea.Batch(cmds...)
		}
		// No critical changes — auto-apply non-critical and proceed.
		// Always apply probe to pendingCfg so Reverse/Duplex/Conference are set.
		_ = probe.ApplyProbeToConfig(&pendingCfg, probeMsg.Result)
		if len(nonCriticals) > 0 {
			m.setupModel.Fields, m.setupModel.AdvancedFields, _ = views.ApplyProbeResult(
				m.setupModel.Fields, m.setupModel.AdvancedFields, probeMsg.Result,
			)
		}
		// Proceed with start using the probe-updated config.
		m.config = pendingCfg
		m.pendingStartCfg = nil
		return m.proceedWithStart(cmds)
	}

	// Handle SetupDoneMsg from the setup screen
	if doneMsg, ok := msg.(views.SetupDoneMsg); ok {
		// Client mode: probe server before starting.
		if doneMsg.Config.Mode == config.ModeClient {
			if m.pendingStartCfg != nil {
				return m, tea.Batch(cmds...) // probe already in flight
			}
			cfg := doneMsg.Config
			m.pendingStartCfg = &cfg
			cmds = append(cmds, startPreStartProbe(cfg.Address, cfg.Port))
			return m, tea.Batch(cmds...)
		}
		m.config = doneMsg.Config
		return m.proceedWithStart(cmds)
	}

	// DeviceSelectedMsg — backward compat for --device flag
	if devMsg, ok := msg.(DeviceSelectedMsg); ok {
		m.selectedDevice = &devMsg.Device
		m.deviceName = devMsg.Device.Name
		m.config.DeviceID = &devMsg.Device.ID
		if m.startFunc == nil {
			return m, tea.Quit
		}
		m.screen = ScreenConnection
		cmds = append(cmds, m.spinner.Tick)
		cfg := m.config
		startFunc := m.startFunc
		m.stopCh = make(chan struct{})
		m.stopOnce = &sync.Once{}
		stopCh := m.stopCh
		cmds = append(cmds, func() tea.Msg {
			statsCh, errCh, srvStoppedCh := startFunc(cfg, stopCh)
			return streamingStartedMsg{statsCh: statsCh, errCh: errCh, serverStoppedCh: srvStoppedCh}
		})
		if m.logCh != nil {
			cmds = append(cmds, waitForLog(m.logCh))
		}
		return m, tea.Batch(cmds...)
	}

	// Forward everything else to the setup model
	var cmd tea.Cmd
	m.setupModel, cmd = m.setupModel.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Sync SIMD state from setup field so the header updates live.
	setupCfg := m.setupModel.BuildConfig()
	if setupCfg.NoSIMDOptimization {
		audio.DisableSIMD()
	} else {
		audio.EnableSIMD()
	}

	return m, tea.Batch(cmds...)
}

// proceedWithStart completes the setup→streaming transition after config is set.
func (m Model) proceedWithStart(cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	// Apply SIMD setting from TUI before starting streaming.
	if m.config.NoSIMDOptimization {
		audio.DisableSIMD()
	} else {
		audio.EnableSIMD()
	}

	// Initialize AEC runtime state from config.
	m.aecActive = m.config.AEC

	// Non-duplex, non-conference: disable channels that don't apply to the mode.
	if !m.config.Duplex && !m.config.Conference {
		if m.config.Reverse {
			// Reverse: server has no capture (no pause), client has no playback (no server mute).
			if m.config.Mode == config.ModeServer {
				m.pauseCh = nil
			} else {
				m.serverMuteCh = nil
			}
		} else {
			// Normal: server has capture (pause OK), client has no capture (no pause).
			if m.config.Mode == config.ModeClient {
				m.pauseCh = nil
			}
		}
	}

	// Dynamically enable multi-client/conference mode based on selected config.
	if m.config.Conference && !m.conference {
		m.multiClient = true
		m.conference = true
		m.serverMuted = m.config.ServerMuted
		m.muteState = NewMuteState()
	} else if m.config.MaxClients > 1 && !m.multiClient {
		m.multiClient = true
	}

	// Extract device info from config (populated by tryStart from multiSelect)
	if len(m.config.Devices) > 0 {
		firstID := m.config.Devices[0].ID
		m.config.DeviceID = &firstID
		m.deviceName = m.setupModel.SelectedDeviceName()
		for _, item := range m.setupModel.DeviceList.Items() {
			if di, ok := item.(deviceItem); ok && di.device.ID == firstID {
				m.selectedDevice = &di.device
				break
			}
		}
	} else if m.config.Conference && m.config.ServerMuted {
		m.deviceName = "Hub (no audio)"
	} else if selectedItem, ok := m.setupModel.DeviceList.SelectedItem().(deviceItem); ok {
		m.selectedDevice = &selectedItem.device
		m.deviceName = selectedItem.device.Name
		m.config.DeviceID = &selectedItem.device.ID
	} else {
		return m, tea.Batch(cmds...)
	}

	if m.startFunc == nil {
		return m, tea.Quit
	}
	m.screen = ScreenConnection
	cmds = append(cmds, m.spinner.Tick)
	cfg := m.config
	startFunc := m.startFunc
	m.stopCh = make(chan struct{})
	m.stopOnce = &sync.Once{}
	stopCh := m.stopCh
	cmds = append(cmds, func() tea.Msg {
		statsCh, errCh, srvStoppedCh := startFunc(cfg, stopCh)
		return streamingStartedMsg{statsCh: statsCh, errCh: errCh, serverStoppedCh: srvStoppedCh}
	})
	if m.logCh != nil {
		cmds = append(cmds, waitForLog(m.logCh))
	}
	return m, tea.Batch(cmds...)
}

// singleClientInfo returns (clientID, nickname) for the connected client in single-client mode.
// Returns ("", "") if no client is connected. Uses multiStats if populated, otherwise falls
// back to checking m.stats.State with a default clientID.
func (m *Model) singleClientInfo() (string, string) {
	if len(m.multiStats.Clients) >= 1 {
		c := m.multiStats.Clients[0]
		nick := c.Nickname
		if nick == "" {
			nick = c.ClientID
		}
		return c.ClientID, nick
	}
	if isConnectedState(m.stats.State) {
		return "client-1", "client-1"
	}
	return "", ""
}

func (m Model) updateConnection(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ConnectedMsg:
		m.stats = msg.Stats
		m.screen = ScreenStreaming
		m.setDefaultFocus()
		m.startTime = time.Now()
		cmds = append(cmds, doSpectrumTick())
		return m, tea.Batch(cmds...)

	case StatsUpdateMsg:
		m.stats = msg.Stats
		if isConnectedState(msg.Stats.State) {
			if m.screen != ScreenStreaming {
				m.setDefaultFocus()
			}
			m.screen = ScreenStreaming
			if m.startTime.IsZero() {
				m.startTime = time.Now()
			}
			m.reconnectAttempt = 0
			// Reset graceful-shutdown reconnect state after successful reconnection.
			m.reconnectAttemptGS = 0
			m.reconnectLastError = ""
			m.criticalChanges = nil
			m.reconnectProbing = false
		}
		nextCmd := waitForStats(m.statsCh, m.errCh)
		if m.screen == ScreenStreaming && !m.spectrumTickActive {
			m.spectrumTickActive = true
			return m, tea.Batch(nextCmd, doSpectrumTick())
		}
		return m, nextCmd

	case ErrorMsg:
		// Check for kicked/banned errors — transition to dedicated screens.
		var ewErr *ewerrors.EchoWarpError
		if errors.As(msg.Err, &ewErr) {
			switch ewErr.Code {
			case ewerrors.ErrClientKicked:
				m.screen = ScreenKicked
				if reason, ok := ewErr.Context["reason"].(string); ok {
					m.kickedReason = reason
				}
				m.kickBanBtnFocus = 0
				return m, nil
			case ewerrors.ErrClientBannedByAdmin:
				m.screen = ScreenBanned
				if reason, ok := ewErr.Context["reason"].(string); ok {
					m.bannedReason = reason
				}
				if criteria, ok := ewErr.Context["criteria"].([]string); ok {
					m.bannedCriteria = criteria
				}
				m.kickBanBtnFocus = 0
				return m, nil
			}
		}

		m.err = msg.Err
		m.errorTimer = time.Now().Add(10 * time.Second)
		dismissCmd := tea.Tick(10*time.Second, func(time.Time) tea.Msg { return ErrorDismissMsg{} })
		cmds = append(cmds, dismissCmd)
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

// nextFocusArea cycles to the next available focus area.
func (m *Model) nextFocusArea() FocusArea {
	// Build list of available areas.
	var areas []FocusArea
	if m.config.Mode == config.ModeServer {
		areas = append(areas, FocusClientList)
	}
	if m.chatPanel.IsVisible() {
		areas = append(areas, FocusChat)
	}
	if m.logsVisible {
		areas = append(areas, FocusLogs)
	}
	// No available areas — stay put.
	if len(areas) == 0 {
		return m.focusedArea
	}

	// Find current position and advance.
	for i, a := range areas {
		if a == m.focusedArea {
			return areas[(i+1)%len(areas)]
		}
	}
	// Current area not found in available areas — jump to first.
	return areas[0]
}

// setDefaultFocus sets the initial focus area based on the current mode.
func (m *Model) setDefaultFocus() {
	if m.config.Mode == config.ModeServer {
		m.focusedArea = FocusClientList
	} else {
		m.focusedArea = FocusLogs
	}
}

// cleanupVirtualSinks removes virtual sinks that have OnStop == SinkDelete.
func (m *Model) cleanupVirtualSinks() {
	moduleID := m.setupModel.VirtualMicModule()
	if moduleID == "" {
		return
	}
	for _, vs := range m.setupModel.SelectedVirtualSinkPresets() {
		if vs.OnStop == recent.SinkDelete {
			_ = views.RemovePulseAudioSink(moduleID)
			return
		}
	}
}
