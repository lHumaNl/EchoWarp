package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// waitForLog waits for a single log entry on logCh and returns it as a LogMsg.
func waitForLog(logCh <-chan LogEntry) tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-logCh
		if !ok {
			return nil
		}
		return LogMsg{Entry: entry}
	}
}

// waitForStats waits for either a stats update or an end signal from the streaming goroutine.
func waitForStats(statsCh <-chan transport.ConnectionStats, errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case stats, ok := <-statsCh:
			if !ok {
				return streamingEndedMsg{}
			}
			return StatsUpdateMsg{Stats: stats}
		case err, ok := <-errCh:
			if !ok {
				return streamingEndedMsg{}
			}
			if err != nil {
				return ErrorMsg{Err: err}
			}
			return streamingEndedMsg{}
		}
	}
}

// waitForMultiStats waits for multi-client stats updates.
func waitForMultiStats(ch <-chan transport.MultiClientStats) tea.Cmd {
	return func() tea.Msg {
		stats, ok := <-ch
		if !ok {
			return streamingEndedMsg{}
		}
		return MultiStatsUpdateMsg{Stats: stats}
	}
}

// waitForConferenceStats waits for conference participant state updates.
func waitForConferenceStats(ch <-chan ConferenceStatsPayload) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		payload, ok := <-ch
		if !ok {
			return nil
		}
		return payload
	}
}

// waitForParticipants waits for a participants list update from the chat layer.
func waitForParticipants(ch <-chan app.ChatParticipantsPayload) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		payload, ok := <-ch
		if !ok {
			return nil
		}
		return ParticipantsUpdateMsg{
			Participants: payload.Participants,
			MaxClients:   payload.MaxClients,
		}
	}
}

// waitForDeviceChange waits for a device hot-plug event.
func waitForDeviceChange(ch <-chan DeviceChangeMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// waitForChat waits for a chat message on the channel.
func waitForChat(ch <-chan app.ChatMessage) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return ChatReceivedMsg{Message: msg}
	}
}

// waitForChatNickname waits for a nickname on the channel.
func waitForChatNickname(ch <-chan string) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		nick, ok := <-ch
		if !ok {
			return nil
		}
		return chatNicknameMsg(nick)
	}
}

// waitForServerStopped waits for the serverStoppedCh to be closed, then returns ServerStoppedMsg.
func waitForServerStopped(ch <-chan struct{}) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		<-ch
		return ServerStoppedMsg{}
	}
}

// doReconnectTick returns a command that triggers a ReconnectTickMsg after 1 second.
func doReconnectTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return ReconnectTickMsg{}
	})
}

// startReconnectProbe returns a tea.Cmd that probes the server and returns ReconnectProbeMsg.
func startReconnectProbe(addr string, port int) tea.Cmd {
	return func() tea.Msg {
		result, err := views.ProbeServer(addr, port)
		if err != nil {
			return ReconnectProbeMsg{Err: err}
		}
		return ReconnectProbeMsg{Result: result}
	}
}

// startPreStartProbe returns a tea.Cmd that probes the server before starting streaming.
func startPreStartProbe(addr string, port int) tea.Cmd {
	return func() tea.Msg {
		result, err := views.ProbeServer(addr, port)
		if err != nil {
			return PreStartProbeMsg{Err: err}
		}
		return PreStartProbeMsg{Result: result}
	}
}

// waitForParticipantPause waits for a participant pause state change.
func waitForParticipantPause(ch <-chan ParticipantPauseMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// waitForConferenceParticipants waits for a full participants list update.
func waitForConferenceParticipants(ch <-chan ConferenceParticipantsMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// doTick returns a command that triggers a tickMsg after tickInterval.
func doTick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// doSpectrumTick returns a command that triggers a spectrumTickMsg for spectrum animation.
func doSpectrumTick() tea.Cmd {
	return tea.Tick(spectrumTickInterval, func(t time.Time) tea.Msg {
		return spectrumTickMsg(t)
	})
}

// saveRecentServerCmd returns a tea.Cmd that persists the connected server to recent history.
// devicePreset contains the selected devices for the given audio mode; it is merged into
// the existing presets map for this server entry so presets for other modes are preserved.
func saveRecentServerCmd(cfg config.Config, probeRes *views.ProbeServerResult, selServer *views.ServerEntry, devicePreset recent.DevicePreset, mode string) tea.Cmd {
	return func() tea.Msg {
		hostname := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
		if probeRes != nil && probeRes.ServerName != "" {
			hostname = probeRes.ServerName
		} else if selServer != nil && selServer.Hostname != "" {
			hostname = selServer.Hostname
		}

		servers, _ := recent.Load()

		// Preserve existing presets for other modes.
		existingPresets := make(map[string]recent.DevicePreset)
		for _, s := range servers {
			if s.Address == cfg.Address && s.Port == cfg.Port {
				for k, v := range s.Presets {
					existingPresets[k] = v
				}
				break
			}
		}
		existingPresets[mode] = devicePreset

		var serverID string
		if probeRes != nil && probeRes.ServerID != "" {
			serverID = probeRes.ServerID
		} else if selServer != nil && selServer.ServerID != "" {
			serverID = selServer.ServerID
		}

		servers = recent.Add(servers, recent.Server{
			Address:       cfg.Address,
			Port:          cfg.Port,
			Hostname:      hostname,
			ServerID:      serverID,
			LastConnected: time.Now(),
			Presets:       existingPresets,
		})
		_ = recent.Save(servers) //nolint:errcheck
		return recentServerSavedMsg{}
	}
}

// saveServerPresetCmd returns a tea.Cmd that persists the server-side device preset and settings to disk.
func saveServerPresetCmd(devicePreset recent.DevicePreset, mode string, settings preset.ServerSettings) tea.Cmd {
	return func() tea.Msg {
		sp := preset.Load()
		sp.Set(mode, devicePreset)
		sp.Settings = settings
		_ = preset.Save(sp) //nolint:errcheck
		return nil
	}
}
