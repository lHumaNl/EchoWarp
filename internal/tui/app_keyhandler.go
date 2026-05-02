package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
)

// handleKeyMsg processes tea.KeyMsg events, extracted from the large case
// branch in Update() for readability. Returns handled=true when the key was
// consumed; when false the caller should fall through to screen-specific
// handling (e.g. forwarding to setupModel).
func (m Model) handleKeyMsg(msg tea.KeyMsg) (model Model, cmd tea.Cmd, handled bool) {
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
			return m, nil, true
		case tea.KeyTab:
			// If input starts with @, use Tab for autocomplete; otherwise cycle focus.
			if strings.HasPrefix(m.chatPanel.InputValue(), "@") {
				m.chatPanel, _ = m.chatPanel.Update(msg)
				return m, nil, true
			}
			m.chatPanel.Unfocus()
			m.focusedArea = m.nextFocusArea()
			return m, nil, true
		case tea.KeyEsc:
			// Esc unfocuses chat, returns to previous area
			m.chatPanel.Unfocus()
			if m.config.Mode == config.ModeServer {
				m.focusedArea = FocusClientList
			} else {
				m.focusedArea = FocusLogs
			}
			return m, nil, true
		default:
			var cmd tea.Cmd
			m.chatPanel, cmd = m.chatPanel.Update(msg)
			return m, cmd, true
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
		return m, nil, true
	}

	// Overlay intercepts — delegate to existing overlay handlers.
	var overlayResult tea.Model
	var overlayCmd tea.Cmd
	switch {
	case m.overlay == OverlayClientPopup:
		overlayResult, overlayCmd = m.handleClientPopupKeys(msg)
	case m.overlay == OverlayKick:
		overlayResult, overlayCmd = m.handleKickOverlayKeys(msg)
	case m.overlay == OverlayBan:
		overlayResult, overlayCmd = m.handleBanOverlayKeys(msg)
	case m.overlay == OverlayDevice:
		overlayResult, overlayCmd = m.handleDeviceOverlayKeys(msg)
	case m.overlay != OverlayNone:
		overlayResult, overlayCmd = m.handleOverlayKeys(msg)
	case m.recordingOverlay.Visible:
		overlayResult, overlayCmd = m.handleRecordingOverlayKeys(msg)
	case m.participantOverlay.Visible:
		overlayResult, overlayCmd = m.handleParticipantOverlayKeys(msg)
	}
	if overlayResult != nil {
		return overlayResult.(Model), overlayCmd, true //nolint:errcheck // tea.Model is always Model
	}

	switch msg.Type {
	case tea.KeyCtrlQ, tea.KeyCtrlC:
		m.requestQuit()
		return m, tea.Quit, true
	case tea.KeyEsc:
		// Clear error banner on Esc (UX-11)
		if m.err != nil {
			m.err = nil
			m.errorTimer = time.Time{}
			return m, nil, true
		}
	case tea.KeyCtrlL:
		if m.screen == ScreenStreaming {
			m.logsVisible = !m.logsVisible
			return m, nil, true
		}
	case tea.KeyCtrlD:
		// Open device overlay (only if devices are available)
		if m.screen == ScreenStreaming && len(m.deviceStates) > 0 {
			m.openDeviceOverlay()
			return m, nil, true
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
			return m, nil, true
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
			return m, nil, true
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
			return m, nil, true
		}
	case tea.KeyCtrlU:
		// Open ban list overlay — available on all screens for server
		if m.config.Mode == config.ModeServer && m.banListFn != nil {
			m.overlay = OverlayBanList
			m.overlaySelection = 0
			m.bannedIPs = m.banListFn()
			return m, nil, true
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
				return m, nil, true
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
			return m, nil, true
		}
	default:
	}

	// Key not consumed — let the caller's screen-specific handler process it.
	return m, nil, false
}
