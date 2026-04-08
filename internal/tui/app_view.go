package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func (m Model) viewServerStopped() string {
	base := views.ServerStoppedView(views.ServerStoppedParams{
		AutoReconnect:  m.autoReconnect,
		Attempt:        m.reconnectAttemptGS,
		MaxAttempts:    m.autoReconnectAttempts,
		Countdown:      m.reconnectCountdown,
		Probing:        m.reconnectProbing,
		LastError:      m.reconnectLastError,
		SelectedButton: m.kickBanBtnFocus,
		Width:          m.width,
		Height:         m.height - 2,
	})
	if len(m.criticalChanges) > 0 {
		overlay := views.CriticalChangesOverlay(views.CriticalChangesOverlayParams{
			Changes: m.criticalChanges,
			Width:   m.width,
			Height:  m.height - 2,
		})
		return lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" "),
		)
	}
	return base
}

func (m Model) viewKicked() string {
	return lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center,
		views.RenderKickedView(views.KickedViewParams{
			Reason:         m.kickedReason,
			SelectedButton: m.kickBanBtnFocus,
			Width:          m.width,
			Height:         m.height - 2,
		}),
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m Model) viewBanned() string {
	return lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center,
		views.RenderBannedView(views.BannedViewParams{
			Reason:         m.bannedReason,
			Criteria:       m.bannedCriteria,
			SelectedButton: m.kickBanBtnFocus,
			Width:          m.width,
			Height:         m.height - 2,
		}),
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m Model) View() string {
	if m.quitting {
		return m.viewSummary()
	}

	// Determine state and duration for header
	state := m.stats.State
	if state == "" {
		switch m.screen {
		case ScreenDeviceSelect:
			state = "new"
		case ScreenConnection:
			state = "connecting"
		case ScreenServerStopped:
			state = "server stopped"
		case ScreenKicked:
			state = "kicked"
		case ScreenBanned:
			state = "banned"
		}
	}

	if m.multiClient && m.screen == ScreenStreaming {
		state = fmt.Sprintf("%d/%d clients", len(m.multiStats.Clients), m.multiStats.MaxClients)
	}

	var duration time.Duration
	if !m.startTime.IsZero() {
		duration = time.Since(m.startTime)
	}

	var recInfo RecordingInfo
	if m.isRecording {
		recInfo.Active = true
		recInfo.Duration = time.Since(m.recordingStart)
		modeNames := []string{"mix", "tracks", "both"}
		modeName := modeNames[m.recordingMode]
		if m.conference {
			selected, total := m.recordingOverlay.SelectedCount()
			if m.config.Mode == config.ModeServer && selected < total {
				recInfo.Label = fmt.Sprintf("[%s] (%d/%d)", modeName, selected, total)
			} else {
				recInfo.Label = fmt.Sprintf("[%s]", modeName)
			}
		} else {
			recInfo.Label = fmt.Sprintf("[%s]", modeName)
		}
	}
	aecDisplay := m.config.AEC
	if m.screen == ScreenStreaming {
		aecDisplay = m.aecActive
	}
	header := renderHeader(m.config, state, duration, m.reconnectCount, aecDisplay, m.width, recInfo)
	// Append error banner below the header so body height is not affected (UX-11)
	if m.err != nil {
		errText := fmt.Sprintf(" Error: %v ", m.err)
		var ewErr *ewerrors.EchoWarpError
		if errors.As(m.err, &ewErr) && ewErr.Suggestion != "" {
			errText += fmt.Sprintf("| Hint: %s ", ewErr.Suggestion)
		}
		errorLine := styles.Error.Render(errText)
		// Pad to full width
		errW := lipgloss.Width(errorLine)
		if errW < m.width {
			errorLine += strings.Repeat(" ", m.width-errW)
		}
		header = header + "\n" + errorLine
	}

	var body string
	switch m.screen {
	case ScreenDeviceSelect:
		body = m.viewDeviceSelect()
	case ScreenConnection:
		body = m.viewConnection()
	case ScreenStreaming:
		body = m.viewStreaming()
	case ScreenServerStopped:
		body = m.viewServerStopped()
	case ScreenKicked:
		body = m.viewKicked()
	case ScreenBanned:
		body = m.viewBanned()
	}

	// Client popup overlay
	if m.overlay == OverlayClientPopup {
		popup := views.RenderClientPopup(views.ClientPopupParams{
			ClientNickname: m.popupClientNick,
			Items:          m.popupItems,
			SelectedIndex:  m.popupSelectedIndex,
			Width:          34,
		})
		// Center popup over body
		popupH := lipgloss.Height(popup)
		popupW := lipgloss.Width(popup)
		bodyH := m.height - 2
		padY := (bodyH - popupH) / 2
		padX := (m.width - popupW) / 2
		if padY < 0 {
			padY = 0
		}
		if padX < 0 {
			padX = 0
		}
		var result strings.Builder
		for i := 0; i < padY; i++ {
			result.WriteString("\n")
		}
		for _, line := range strings.Split(popup, "\n") {
			result.WriteString(strings.Repeat(" ", padX))
			result.WriteString(line)
			result.WriteString("\n")
		}
		body = result.String()
	}

	// Ban list overlay — available on all screens for server
	if m.overlay == OverlayBanList {
		body = views.BanListView(views.BanListParams{
			BannedIPs:     m.bannedIPs,
			SelectedIndex: m.overlaySelection,
			Width:         m.width,
			Height:        m.height - 2,
		})
	}

	// Kick overlay
	if m.overlay == OverlayKick {
		body = views.RenderKickOverlay(views.KickOverlayParams{
			ClientNickname:   m.kickClientNick,
			Reasons:          m.kickReasons,
			RecentStartIndex: m.kickRecentStart,
			CustomStartIndex: m.kickCustomStart,
			SelectedIndex:    m.kickSelectedIndex,
			CustomText:       m.kickCustomText,
			CustomEditing:    m.kickCustomEditing,
			FocusButton:      m.kickFocusButton,
			Width:            m.width,
			Height:           m.height - 2,
		})
	}

	// Ban overlay
	if m.overlay == OverlayBan {
		body = views.RenderBanOverlay(views.BanOverlayParams{
			ClientNickname:   m.banClientNick,
			IP:               m.banClientIP,
			Nickname:         m.banClientNick,
			HWID:             m.banClientHWID,
			HWIDAvailable:    m.config.HWIDRequired,
			CriteriaIP:       m.banCriteriaIP,
			CriteriaNick:     m.banCriteriaNick,
			CriteriaHWID:     m.banCriteriaHWID,
			Reasons:          m.banReasons,
			RecentStartIndex: m.banRecentStart,
			CustomStartIndex: m.banCustomStart,
			SelectedReason:   m.banSelectedReason,
			CustomText:       m.banCustomText,
			CustomEditing:    m.banCustomEditing,
			FocusSection:     m.banFocusSection,
			CriteriaIndex:    m.banCriteriaIndex,
			ButtonFocus:      m.banButtonFocus,
			ValidationError:  m.banValidationError,
			Width:            m.width,
			Height:           m.height - 2,
		})
	}

	// Device overlay
	if m.overlay == OverlayDevice {
		var items []views.DeviceOverlayItem
		for _, ds := range m.deviceStates {
			isInput := ds.Role == "capture"
			// Find matching AudioDevice for specs.
			var channels, sampleRate, bitDepth uint32
			for _, dev := range m.devices {
				if dev.ID == ds.ID {
					channels = dev.Channels
					sampleRate = dev.SampleRate
					bitDepth = dev.BitDepth
					break
				}
			}
			items = append(items, views.DeviceOverlayItem{
				ID:         ds.ID,
				Name:       ds.Name,
				IsInput:    isInput,
				Channels:   channels,
				SampleRate: sampleRate,
				BitDepth:   bitDepth,
				Muted:      ds.Muted,
			})
		}
		body = views.RenderDeviceOverlay(views.DeviceOverlayParams{
			Devices:  items,
			Section:  m.deviceOverlaySection,
			Selected: m.deviceOverlayIndex,
			Width:    m.width,
			Height:   m.height - 2,
		})
	}

	helpKeys := m.helpKeys()
	// For setup screen, build config from current field values for the status bar
	statusCfg := m.config
	if m.screen == ScreenDeviceSelect {
		statusCfg = m.setupModel.BuildConfig()
	}
	statusBar := renderStatusBar(statusCfg, statusCfg.IsTLSEnabled(), statusCfg.TLSSelfSigned, helpKeys, m.width)

	return renderLayout(header, body, statusBar, m.width, m.height)
}

func (m Model) viewDeviceSelect() string {
	m.setupModel.SetSize(m.width, m.height)
	return m.setupModel.View()
}

func (m Model) viewConnection() string {
	return views.ConnectionView(views.ConnectionParams{
		Spinner:          m.spinner,
		IsServer:         m.config.Mode == config.ModeServer,
		Address:          m.config.Address,
		Port:             m.config.Port,
		Reverse:          m.config.Reverse,
		Duplex:           m.config.Duplex,
		Conference:       m.conference,
		TLSEnabled:       m.config.IsTLSEnabled(),
		TLSSelfSigned:    m.config.TLSSelfSigned,
		DeviceName:       m.deviceName,
		Logs:             m.logs,
		ReconnectAttempt: m.reconnectAttempt,
		MaxAttempts:      m.config.MaxReconnectAttempts,
		Width:            m.width,
		Height:           m.height - 2, // minus header + statusbar
	})
}

func (m Model) viewStreaming() string {
	if m.conference {
		participants := make([]views.ConferenceParticipant, 0, len(m.conferenceStates))
		for _, s := range m.conferenceStates {
			participants = append(participants, views.ConferenceParticipant{
				ID:       s.ID,
				Volume:   s.Volume,
				Muted:    s.Muted,
				Speaking: s.Speaking,
				RMSLevel: s.RMSLevel,
			})
		}

		// Build per-client quality badges.
		qualities := make([]views.QualityLevel, len(m.multiStats.Clients))
		for i, c := range m.multiStats.Clients {
			qualities[i] = views.CalculateQuality(c.RoundTrip, c.Jitter, c.PacketsLost)
		}

		// Build selected client stats.
		var selStats transport.ConnectionStats
		var selJitterHist, selRTTHist []float64
		var selBitrateUp, selBitrateDown float64
		var selQuality views.QualityLevel
		var selPacketLoss float64

		// Determine how many navigable entries there are (server + clients or just clients).
		isServer := m.config.Mode == config.ModeServer
		hubMode := isServer && m.serverMuted
		serverOffset := 0
		if isServer && !hubMode {
			serverOffset = 1 // server entry occupies index 0
		}

		clientIdx := m.selectedClient - serverOffset
		if clientIdx >= 0 && clientIdx < len(m.multiStats.Clients) {
			c := m.multiStats.Clients[clientIdx]
			selStats = transport.ConnectionStats{
				State:       c.State,
				RemoteAddr:  c.RemoteAddr,
				BytesSent:   c.BytesSent,
				BytesRecv:   c.BytesRecv,
				PacketsLost: c.PacketsLost,
				Jitter:      c.Jitter,
				RoundTrip:   c.RoundTrip,
			}
			selJitterHist = m.perClientJitterHistory[c.ClientID]
			selRTTHist = m.perClientRTTHistory[c.ClientID]
			selBitrateUp = m.perClientBitrateUp[c.ClientID]
			selBitrateDown = m.perClientBitrateDown[c.ClientID]
			if clientIdx < len(qualities) {
				selQuality = qualities[clientIdx]
			}

			if !m.startTime.IsZero() && c.PacketsLost > 0 {
				elapsed := time.Since(m.startTime).Seconds()
				if elapsed >= 1 {
					const packetsPerSec = 50.0
					pct := float64(c.PacketsLost) / (elapsed * packetsPerSec) * 100
					if pct > 100 {
						pct = 100
					}
					selPacketLoss = pct
				}
			}
		}

		// Aggregate traffic.
		var aggSent, aggRecv uint64
		var aggUp, aggDown float64
		for _, c := range m.multiStats.Clients {
			aggSent += c.BytesSent
			aggRecv += c.BytesRecv
			aggUp += m.perClientBitrateUp[c.ClientID]
			aggDown += m.perClientBitrateDown[c.ClientID]
		}

		confParams := views.ConferenceParams{
			Stats:               m.multiStats,
			Participants:        participants,
			IsServer:            isServer,
			HubMode:             hubMode,
			ServerMuted:         m.serverMuted,
			SelectedIndex:       m.selectedClient,
			DeviceName:          m.deviceName,
			Logs:                m.logs,
			LogsVisible:         m.logsVisible,
			LogScrollOffset:     m.logScrollOffset,
			Width:               m.width,
			Height:              m.height - 2,
			ChatView:            m.chatPanelView(),
			SelectedStats:       selStats,
			SelectedJitterHist:  selJitterHist,
			SelectedRTTHist:     selRTTHist,
			SelectedBitrateUp:   selBitrateUp,
			SelectedBitrateDown: selBitrateDown,
			SelectedQuality:     selQuality,
			SelectedPacketLoss:  selPacketLoss,
			AggBytesSent:        aggSent,
			AggBytesRecv:        aggRecv,
			AggBitrateUp:        aggUp,
			AggBitrateDown:      aggDown,
			ClientQualities:     qualities,
			Paused:              m.paused,
			OnlineParticipants:  m.participants,
			MaxClients:          m.maxClients,
			MyNickname:          m.chatPanel.MyNickname(),
		}

		// Populate mute state.
		if m.muteState != nil {
			confParams.MutedParticipants = m.muteState.MutedMap()
			confParams.MuteAll = m.muteState.IsMuteAll()
		}

		// Populate pause state.
		if len(m.pausedParticipants) > 0 {
			confParams.PausedParticipants = m.pausedParticipants
		}

		// Populate dual spectrum.
		confParams.CaptureSpectrumBands = m.captureSpectrumBands
		confParams.CaptureVULevels = m.captureVULevels
		confParams.PlaybackSpectrumBands = m.spectrumBands
		confParams.PlaybackVULevels = m.vuLevels

		body := views.ConferenceView(confParams)
		if m.participantOverlay.Visible {
			body = m.participantOverlay.Render(m.width, m.height-2)
		} else if m.recordingOverlay.Visible {
			body = m.recordingOverlay.Render(m.width, m.height-2)
		}
		return body
	}
	if m.multiClient {
		// Build per-client quality badges.
		qualities := make([]views.QualityLevel, len(m.multiStats.Clients))
		for i, c := range m.multiStats.Clients {
			qualities[i] = views.CalculateQuality(c.RoundTrip, c.Jitter, c.PacketsLost)
		}

		// Build selected client stats.
		var selStats transport.ConnectionStats
		var selJitterHist, selRTTHist []float64
		var selBitrateUp, selBitrateDown float64
		var selQuality views.QualityLevel
		var selPacketLoss float64

		if m.selectedClient < len(m.multiStats.Clients) {
			c := m.multiStats.Clients[m.selectedClient]
			selStats = transport.ConnectionStats{
				State:       c.State,
				RemoteAddr:  c.RemoteAddr,
				BytesSent:   c.BytesSent,
				BytesRecv:   c.BytesRecv,
				PacketsLost: c.PacketsLost,
				Jitter:      c.Jitter,
				RoundTrip:   c.RoundTrip,
			}
			selJitterHist = m.perClientJitterHistory[c.ClientID]
			selRTTHist = m.perClientRTTHistory[c.ClientID]
			selBitrateUp = m.perClientBitrateUp[c.ClientID]
			selBitrateDown = m.perClientBitrateDown[c.ClientID]
			selQuality = qualities[m.selectedClient]

			// Estimate packet loss percentage.
			if !m.startTime.IsZero() && c.PacketsLost > 0 {
				elapsed := time.Since(m.startTime).Seconds()
				if elapsed >= 1 {
					const packetsPerSec = 50.0
					pct := float64(c.PacketsLost) / (elapsed * packetsPerSec) * 100
					if pct > 100 {
						pct = 100
					}
					selPacketLoss = pct
				}
			}
		}

		// Aggregate traffic across all clients.
		var aggSent, aggRecv uint64
		var aggUp, aggDown float64
		for _, c := range m.multiStats.Clients {
			aggSent += c.BytesSent
			aggRecv += c.BytesRecv
			aggUp += m.perClientBitrateUp[c.ClientID]
			aggDown += m.perClientBitrateDown[c.ClientID]
		}

		multiParams := views.MultiClientParams{
			Stats:               m.multiStats,
			Reverse:             m.config.Reverse,
			Duplex:              m.config.Duplex,
			DeviceName:          m.deviceName,
			SelectedIndex:       m.selectedClient,
			Logs:                m.logs,
			LogsVisible:         m.logsVisible,
			LogScrollOffset:     m.logScrollOffset,
			Width:               m.width,
			Height:              m.height - 2,
			SelectedStats:       selStats,
			SelectedJitterHist:  selJitterHist,
			SelectedRTTHist:     selRTTHist,
			SelectedBitrateUp:   selBitrateUp,
			SelectedBitrateDown: selBitrateDown,
			SelectedQuality:     selQuality,
			SelectedPacketLoss:  selPacketLoss,
			AggBytesSent:        aggSent,
			AggBytesRecv:        aggRecv,
			AggBitrateUp:        aggUp,
			AggBitrateDown:      aggDown,
			ClientQualities:     qualities,
			SpectrumBands:       m.effectiveSpectrumBands(),
			VULevels:            m.effectiveVULevels(),
			ChatView:            m.chatPanelView(),
			Nickname:            m.chatPanel.MyNickname(),
			Paused:              m.paused,
			FocusedArea:         int(m.focusedArea),
		}
		// For duplex multi-client, capture = outgoing (local mic); playback will be
		// populated when per-client playback analysis is wired in.
		if m.config.Duplex {
			multiParams.CaptureSpectrumBands = m.captureSpectrumBands
			multiParams.CaptureVULevels = m.captureVULevels
			multiParams.PlaybackSpectrumBands = m.spectrumBands
			multiParams.PlaybackVULevels = m.vuLevels
		}
		body := views.MultiClientView(multiParams)
		if m.recordingOverlay.Visible {
			body = m.recordingOverlay.Render(m.width, m.height-2)
		}
		return body
	}
	var devDisplay []views.DeviceDisplayState
	for i, ds := range m.deviceStates {
		devDisplay = append(devDisplay, views.DeviceDisplayState{
			ID:           ds.ID,
			Name:         ds.Name,
			Role:         ds.Role,
			Volume:       ds.Volume,
			Muted:        ds.Muted,
			Selected:     i == m.selectedDevice2,
			Disconnected: ds.Disconnected,
		})
	}
	params := views.StreamingParams{
		Stats:           m.stats,
		StartTime:       m.startTime,
		Paused:          m.paused,
		Reverse:         m.config.Reverse,
		Duplex:          m.config.Duplex,
		Audio:           views.AudioInfo{Codec: "Opus", SampleRate: m.config.SampleRate, Channels: m.config.Channels},
		Logs:            m.logs,
		LogsVisible:     m.logsVisible,
		LogScrollOffset: m.logScrollOffset,
		JitterHistory:   m.jitterHistory,
		RTTHistory:      m.rttHistory,
		DeviceName:      m.deviceName,
		Width:           m.width,
		Height:          m.height - 2,
		Devices:         devDisplay,
		GlobalMuted:     m.globalMuted,
		BitrateUp:       m.bitrateUp,
		BitrateDown:     m.bitrateDown,
		SpectrumBands:   m.effectiveSpectrumBands(),
		VULevels:        m.effectiveVULevels(),
		Quality:         m.currentQuality(),
		PacketLossPct:   m.packetLossPct(),
		ChatView:        m.chatPanelView(),
		Nickname:        m.chatPanel.MyNickname(),
		Participants:    m.participants,
		MaxClients:      m.maxClients,
		MyNickname:      m.chatPanel.MyNickname(),
		SourcePaused:    m.pausedParticipants["server"],
		ServerMuted:     m.serverMuted,
		FocusedArea:     int(m.focusedArea),
	}

	// For duplex mode, populate both capture and playback spectrum data.
	if m.config.Duplex {
		params.CaptureSpectrumBands = m.captureSpectrumBands
		params.CaptureVULevels = m.captureVULevels
		params.PlaybackSpectrumBands = m.spectrumBands
		params.PlaybackVULevels = m.vuLevels
	}

	body := views.StreamingView(params)
	if m.recordingOverlay.Visible {
		body = m.recordingOverlay.Render(m.width, m.height-2)
	}
	return body
}

// isCaptureOnlyMode returns true when the local side only captures audio (no playback).
// In this mode the primary spectrum should show capture data, not the empty playback analyzer.
func (m *Model) isCaptureOnlyMode() bool {
	if m.config.Duplex {
		return false
	}
	isServer := m.config.Mode == config.ModeServer
	return (isServer && !m.config.Reverse) || (!isServer && m.config.Reverse)
}

// effectiveSpectrumBands returns capture spectrum bands when in capture-only mode,
// otherwise returns the playback spectrum bands.
func (m *Model) effectiveSpectrumBands() []float64 {
	if m.isCaptureOnlyMode() && len(m.captureSpectrumBands) > 0 {
		return m.captureSpectrumBands
	}
	return m.spectrumBands
}

// effectiveVULevels returns capture VU levels when in capture-only mode,
// otherwise returns the playback VU levels.
func (m *Model) effectiveVULevels() []float64 {
	if m.isCaptureOnlyMode() && len(m.captureVULevels) > 0 {
		return m.captureVULevels
	}
	return m.vuLevels
}

// chatPanelView renders the chat panel with appropriate sizing.
func (m *Model) chatPanelView() string {
	if !m.chatPanel.IsVisible() {
		return ""
	}
	// 30% of available height, min 5, max 12
	availH := m.height - 2 // minus header + statusbar
	chatH := availH * 30 / 100
	if chatH < 5 {
		chatH = 5
	}
	if chatH > 12 {
		chatH = 12
	}
	m.chatPanel.SetSize(m.width, chatH)
	return m.chatPanel.View()
}

// packetLossPct estimates packet loss percentage from elapsed time and lost packets.
// Opus sends ~50 packets/sec (20ms frames). We use this to estimate total expected packets.
func (m Model) packetLossPct() float64 {
	if m.startTime.IsZero() || m.stats.PacketsLost == 0 {
		return 0
	}
	elapsed := time.Since(m.startTime).Seconds()
	if elapsed < 1 {
		return 0
	}
	// Estimated packets per second based on Opus 20ms frames
	const packetsPerSec = 50.0
	estimatedTotal := elapsed * packetsPerSec
	pct := float64(m.stats.PacketsLost) / estimatedTotal * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

// currentQuality computes the connection quality level from current stats.
func (m Model) currentQuality() views.QualityLevel {
	if m.screen != ScreenStreaming || m.stats.State != "connected" {
		return views.QualityUnknown
	}
	return views.CalculateQuality(m.stats.RoundTrip, m.stats.Jitter, m.stats.PacketsLost)
}

func (m Model) viewSummary() string {
	var duration time.Duration
	if !m.startTime.IsZero() {
		duration = time.Since(m.startTime)
	}

	var avgJitter, avgRTT float64
	avgBitrate := "0.0 kbps"
	if m.statsCount > 0 {
		avgJitter = m.jitterSum / float64(m.statsCount)
		avgRTT = m.rttSum / float64(m.statsCount)
		avgBitrate = views.CalculateBitrate(m.totalBytesSent, m.totalBytesRecv, duration)
	}

	return views.SummaryView(views.SessionSummary{
		Duration:       duration,
		TotalSent:      m.totalBytesSent,
		TotalRecv:      m.totalBytesRecv,
		AvgBitrate:     avgBitrate,
		ReconnectCount: m.reconnectCount,
		AvgJitter:      avgJitter,
		AvgRTT:         avgRTT,
		TotalLoss:      m.totalPacketsLost,
		LogFile:        m.logFile,
	})
}

func (m Model) helpKeys() string {
	// Flash notification takes priority over normal help text.
	if m.flashMsg != "" {
		return styles.FlashSuccess.Render("✓ " + m.flashMsg)
	}

	if m.overlay == OverlayClientPopup {
		return "↑↓: select  enter: confirm  esc: close  ^Q: quit"
	}
	if m.overlay == OverlayBanList {
		return "↑↓: select  enter: unban  esc: close  ^Q: quit"
	}

	switch m.screen {
	case ScreenDeviceSelect:
		help := m.setupModel.HelpKeys()
		if m.config.Mode == config.ModeServer && m.banListFn != nil {
			help += "  ^U: ban list"
		}
		langCode := strings.ToUpper(string(i18n.CurrentLanguage()))
		help += "  [" + langCode + "]"
		return help
	case ScreenConnection:
		if m.config.Mode == config.ModeServer && m.banListFn != nil {
			return "^U: ban list  ^Q: quit"
		}
		return "^Q: quit"
	case ScreenServerStopped:
		if len(m.criticalChanges) > 0 {
			return "Enter: open setup  Esc: quit"
		}
		return "←→: select  Enter: confirm  ^Q: quit"
	case ScreenKicked:
		return "←→: select  Enter: confirm  ^Q: quit"
	case ScreenBanned:
		return "←→: select  Enter: confirm  ^Q: quit"
	case ScreenStreaming:
		// Chat focused mode — show chat-specific help
		if m.chatPanel.IsFocused() {
			return "enter: send  esc: back  ↑↓: scroll  ^Q: quit"
		}

		if m.recordingOverlay.Visible {
			if m.recordingOverlay.IsStatusState() {
				return "enter: stop recording  esc: close"
			}
			return "↑↓: move  space: toggle  ^A: all  ^N: none  enter: start  esc: cancel"
		}

		if m.conference {
			if m.participantOverlay.Visible {
				return "↑↓: select  enter: confirm  esc: cancel"
			}
			parts := []string{"↑↓: select"}
			isServer := m.config.Mode == config.ModeServer
			hubMode := isServer && m.serverMuted
			if isServer && !hubMode {
				// Server (not hub): can pause capture
				parts = append(parts, "ctrl+p: pause")
			}
			// Hub: only ↑↓: select
			parts = append(parts, "enter: actions", "+/-: vol")
			if m.isRecording {
				parts = append(parts, "^R: stop rec")
			} else {
				parts = append(parts, "^R: rec")
			}
			parts = append(parts, "^T: chat", "^Q: quit")
			return strings.Join(parts, "  ")
		}
		if m.multiClient {
			parts := []string{"↑↓: select", "enter: actions"}
			if m.pauseCh != nil {
				if m.paused {
					parts = append(parts, "^P: resume")
				} else {
					parts = append(parts, "^P: pause")
				}
			}
			parts = append(parts, "^U: unban list")
			if m.isRecording {
				parts = append(parts, "^R: stop rec")
			} else {
				parts = append(parts, "^R: rec")
			}
			if m.chatPanel.IsVisible() {
				parts = append(parts, "^T: hide chat")
			} else {
				parts = append(parts, "^T: show chat")
			}
			if m.logsVisible {
				parts = append(parts, "^L: hide logs")
			} else {
				parts = append(parts, "^L: show logs")
			}
			parts = append(parts, "^Q: quit")
			return strings.Join(parts, "  ")
		}
		parts := []string{}
		if m.config.Mode == config.ModeServer {
			// Server single-client: if a client is connected, offer Enter→popup
			if m.cmdCh != nil && len(m.multiStats.Clients) == 1 {
				parts = append(parts, "enter: actions")
			}
		} else {
			// Client mode: Enter = mute/unmute (normal/duplex), pause/resume (reverse)
			if m.config.Reverse && !m.config.Duplex {
				if m.paused {
					parts = append(parts, "enter: resume")
				} else {
					parts = append(parts, "enter: pause")
				}
			} else {
				// Normal or duplex: Enter = mute server
				if m.serverMuted {
					parts = append(parts, "enter: unmute")
				} else {
					parts = append(parts, "enter: mute")
				}
			}
		}
		if len(m.deviceStates) > 0 {
			parts = append(parts, "^D: devices")
		}
		if m.chatPanel.IsVisible() {
			parts = append(parts, "^T: hide chat")
		} else {
			parts = append(parts, "^T: show chat")
		}
		if m.logsVisible {
			parts = append(parts, "^L: hide logs")
		} else {
			parts = append(parts, "^L: show logs")
		}
		if m.config.AEC {
			if m.aecActive {
				parts = append(parts, "^E: AEC off")
			} else {
				parts = append(parts, "^E: AEC on")
			}
		}
		// Show ^P for server, and for duplex client (Enter=mute, ^P=pause).
		if m.pauseCh != nil && (m.config.Mode == config.ModeServer || m.config.Duplex) {
			if m.paused {
				parts = append(parts, "^P: resume")
			} else {
				parts = append(parts, "^P: pause")
			}
		}
		if m.isRecording {
			parts = append(parts, "^R: stop rec")
		} else {
			parts = append(parts, "^R: rec")
		}
		parts = append(parts, "^Q: quit")
		return strings.Join(parts, "  ")
	}
	return "^Q: quit"
}

// Quitting returns true if the user has requested to quit.
func (m Model) Quitting() bool { return m.quitting }

// SelectedDeviceID returns the selected audio device ID, or nil if not selected.
func (m Model) SelectedDeviceID() *uint32 {
	if m.selectedDevice != nil {
		return &m.selectedDevice.ID
	}
	return m.config.DeviceID
}
