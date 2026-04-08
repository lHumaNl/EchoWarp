package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// reportStats periodically sends connection statistics to the TUI stats channel.
// Started immediately when the signaling loop begins (before DC ready), so the TUI
// transitions to the streaming screen as soon as PeerConnection reaches "connected".
func (s *ServerApp) reportStats(ctx context.Context, peer transport.PeerManager) {
	if s.statsCh == nil {
		return
	}
	// Send initial stat immediately.
	select {
	case s.statsCh <- peer.GetStats():
	default:
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case s.statsCh <- peer.GetStats():
			default:
			}
		}
	}
}

// sendDisconnected sends a final "disconnected" stats update to the TUI.
func (s *ServerApp) sendDisconnected() {
	if s.statsCh == nil {
		return
	}
	select {
	case s.statsCh <- transport.ConnectionStats{State: "disconnected"}:
	default:
	}
}

// reportMultiStats periodically sends multi-client stats to the TUI.
func (s *ServerApp) reportMultiStats(ctx context.Context) {
	if s.multiStatsCh == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := s.collectMultiClientStats()
			select {
			case s.multiStatsCh <- stats:
			default:
			}
		}
	}
}

// collectMultiClientStats gathers stats from all connected clients.
func (s *ServerApp) collectMultiClientStats() transport.MultiClientStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type clientWithTime struct {
		info     transport.ClientInfo
		joinedAt time.Time
	}
	raw := make([]clientWithTime, 0, len(s.clients))
	for _, mc := range s.clients {
		ci := transport.ClientInfo{
			ClientID:      mc.id,
			Nickname:      mc.nickname,
			RemoteAddr:    auth.ExtractIP(mc.conn.RemoteAddr().String()),
			JoinedAt:      mc.joinedAt.Format("15:04:05"),
			Duration:      time.Since(mc.joinedAt).Round(time.Second).String(),
			Muted:         mc.muted.Load(),
			MutedOutgoing: mc.mutedOutgoing.Load(),
			MutedIncoming: mc.mutedIncoming.Load(),
			Paused:        mc.paused.Load(),
			HWID:          mc.hwid,
		}
		if mc.peer != nil {
			peerStats := mc.peer.GetStats()
			ci.BytesSent = peerStats.BytesSent
			ci.BytesRecv = peerStats.BytesRecv
			ci.Jitter = peerStats.Jitter
			ci.RoundTrip = peerStats.RoundTrip
			ci.PacketsLost = peerStats.PacketsLost
			ci.State = peerStats.State
		}
		raw = append(raw, clientWithTime{info: ci, joinedAt: mc.joinedAt})
	}
	sort.Slice(raw, func(i, j int) bool {
		return raw[i].joinedAt.Before(raw[j].joinedAt)
	})
	clients := make([]transport.ClientInfo, len(raw))
	for i, r := range raw {
		clients[i] = r.info
	}

	return transport.MultiClientStats{
		Clients:    clients,
		MaxClients: s.cfg.MaxClients,
	}
}

// processCommands handles TUI commands (kick/ban/unban) in a loop.
func (s *ServerApp) processCommands(ctx context.Context) {
	if s.cmdCh == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-s.cmdCh:
			if !ok {
				return
			}
			switch cmd.Action {
			case ActionKick:
				s.KickClient(cmd.ClientID, cmd.Reason)
			case ActionBan:
				s.BanClient(cmd.ClientID, cmd.Reason, cmd.BanCriteria)
			case ActionUnban:
				if s.banMgr != nil {
					s.banMgr.Unban(cmd.IP)
					s.logger.Info("Unbanned IP", "ip", cmd.IP)
				}
			case ActionMuteOutgoing:
				s.toggleMuteOutgoing(cmd.ClientID)
			case ActionMuteIncoming:
				s.toggleMuteIncoming(cmd.ClientID)
			case ActionVolumeUp:
				s.adjustClientVolume(cmd.ClientID, 0.1)
			case ActionVolumeDown:
				s.adjustClientVolume(cmd.ClientID, -0.1)
			}
		}
	}
}

// KickClient disconnects a client by ID with an optional reason. The client can reconnect.
func (s *ServerApp) KickClient(clientID string, reason string) {
	s.mu.RLock()
	mc, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		s.logger.Warn("Kick: client not found", "clientID", clientID)
		return
	}
	s.logger.Info("Kicking client", "clientID", clientID, "addr", mc.conn.RemoteAddr(), "reason", reason)
	// Send kick notification before closing
	if mc.peer != nil {
		payload := map[string]interface{}{"reason": reason}
		_ = mc.peer.SendControl(transport.ActionKickNotify, payload) //nolint:errcheck
		_ = mc.peer.Close()                                          //nolint:errcheck
	}
	_ = mc.conn.Close() //nolint:errcheck
}

// BanClient disconnects a client and bans by the given criteria.
// criteria may contain "ip", "nickname", "hwid". If empty, defaults to IP ban.
func (s *ServerApp) BanClient(clientID, reason string, criteria []string) {
	s.mu.RLock()
	mc, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		s.logger.Warn("Ban: client not found", "clientID", clientID)
		return
	}
	ip := auth.ExtractIP(mc.conn.RemoteAddr().String())
	s.logger.Info("Banning client", "clientID", clientID, "ip", ip, "criteria", criteria, "reason", reason)

	// Default to IP ban if no criteria specified.
	if len(criteria) == 0 {
		criteria = []string{"ip"}
	}

	if s.banMgr != nil {
		for _, c := range criteria {
			switch c {
			case "ip":
				s.banMgr.Ban(ip)
			case "nickname":
				if mc.nickname != "" {
					s.banMgr.BanNickname(mc.nickname)
				}
			case "hwid":
				if mc.hwid != "" {
					s.banMgr.BanHWID(mc.hwid)
				}
				// HWID implies IP fallback
				s.banMgr.Ban(ip)
			}
		}
	}

	// Send ban notification before closing.
	if mc.peer != nil {
		payload := map[string]interface{}{
			"reason":   reason,
			"criteria": criteria,
		}
		_ = mc.peer.SendControl(transport.ActionBanNotify, payload) //nolint:errcheck
		_ = mc.peer.Close()                                         //nolint:errcheck
	}
	_ = mc.conn.Close() //nolint:errcheck
}

// toggleMuteOutgoing toggles the server-initiated outgoing mute for a client
// and sends a datachannel notification so the client knows.
func (s *ServerApp) toggleMuteOutgoing(clientID string) {
	s.mu.RLock()
	mc, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		s.logger.Warn("MuteOutgoing: client not found", "clientID", clientID)
		return
	}

	// Toggle: load current, store opposite.
	newVal := !mc.mutedOutgoing.Load()
	mc.mutedOutgoing.Store(newVal)

	action := transport.ActionMuteOutgoing
	if !newVal {
		action = transport.ActionUnmuteOutgoing
	}

	nick := mc.nickname
	if nick == "" {
		nick = clientID
	}

	if mc.peer != nil {
		_ = mc.peer.SendControl(action, nil) //nolint:errcheck
	}
	s.logger.Info("Toggled outgoing mute", "clientID", clientID, "nickname", nick, "mutedOutgoing", newVal)
}

// toggleMuteIncoming toggles the server-initiated incoming mute for a client
// and sends a datachannel notification so the client knows.
func (s *ServerApp) toggleMuteIncoming(clientID string) {
	s.mu.RLock()
	mc, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		s.logger.Warn("MuteIncoming: client not found", "clientID", clientID)
		return
	}

	// Toggle: load current, store opposite.
	newVal := !mc.mutedIncoming.Load()
	mc.mutedIncoming.Store(newVal)

	action := transport.ActionMuteIncoming
	if !newVal {
		action = transport.ActionUnmuteIncoming
	}

	nick := mc.nickname
	if nick == "" {
		nick = clientID
	}

	if mc.peer != nil {
		_ = mc.peer.SendControl(action, nil) //nolint:errcheck
	}
	s.logger.Info("Toggled incoming mute", "clientID", clientID, "nickname", nick, "mutedIncoming", newVal)
}

// adjustClientVolume adjusts the per-client volume in the conference mixer by delta (±0.1).
// In conference mode, uses the ConferenceMixer's per-participant volume control.
func (s *ServerApp) adjustClientVolume(clientID string, delta float64) {
	s.mu.RLock()
	mc, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		s.logger.Warn("AdjustVolume: client not found", "clientID", clientID)
		return
	}
	nick := mc.nickname
	if nick == "" {
		nick = clientID
	}

	if s.conference != nil {
		// Use conference mixer per-participant volume.
		states := s.conference.GetParticipantStates()
		for _, st := range states {
			if st.ID != clientID {
				continue
			}
			newVol := st.Volume + float32(delta)
			if newVol > 2.0 {
				newVol = 2.0
			}
			if newVol < 0 {
				newVol = 0
			}
			s.conference.mixer.SetParticipantVolume(clientID, newVol)
			s.logger.Info("Client volume adjusted", "clientID", clientID, "nickname", nick, "volume", newVol)
			return
		}
		s.logger.Warn("AdjustVolume: participant not found in conference", "clientID", clientID)
		return
	}

	s.logger.Info("Client volume adjusted (no conference mixer)", "clientID", clientID, "nickname", nick, "delta", delta)
}

// processRecordingCommands handles recording start/stop commands from TUI.
func (s *ServerApp) processRecordingCommands(ctx context.Context) {
	if s.recordingCmdCh == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-s.recordingCmdCh:
			if !ok {
				return
			}
			if cmd.Start {
				if err := s.conference.StartRecording(cmd.Mode, s.cfg.SampleRate); err != nil {
					s.logger.Error("Failed to start recording", "error", err)
				} else {
					s.logger.Info("Recording started", "mode", cmd.Mode)
				}
			} else {
				dur, size, files, err := s.conference.StopRecording()
				if err != nil {
					s.logger.Error("Failed to stop recording", "error", err)
				} else {
					s.logger.Info("Recording stopped",
						"duration", dur.Round(time.Second),
						"files", files,
						"size", formatBytes(size))
				}
			}
		}
	}
}

// flushRecordingHeaders periodically flushes WAV headers for crash safety.
func (s *ServerApp) flushRecordingHeaders(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.conference != nil && s.conference.IsRecording() {
				s.conference.FlushHeaders()
			}
		}
	}
}

// recordMixLoop writes the total mix to the recorder at 20ms intervals.
func (s *ServerApp) recordMixLoop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.conference == nil || !s.conference.IsRecording() {
				continue
			}
			mix := s.conference.GetTotalMix()
			if mix == nil {
				continue
			}
			_ = s.conference.WriteMix(mix) //nolint:errcheck
			s.conference.ReturnMixBuffer(mix)
		}
	}
}

// conferenceStatsCh is set by WithConferenceStatsChannel.
// reportConferenceStats periodically sends conference participant state to the TUI.
func (s *ServerApp) reportConferenceStats(ctx context.Context) {
	if s.conferenceStatsCh == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := ConferenceStatsPayload{
				States:             s.conference.GetParticipantStates(),
				Recording:          s.conference.IsRecording(),
				PausedParticipants: s.conference.GetPausedParticipants(),
			}
			select {
			case s.conferenceStatsCh <- payload:
			default:
			}
		}
	}
}

// formatBytes returns a human-readable byte count (e.g. "12.4 MiB").
func formatBytes(b uint64) string {
	const mib = 1024 * 1024
	if b < mib {
		return fmt.Sprintf("%.1f KiB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(b)/float64(mib))
}
