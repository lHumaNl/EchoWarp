package app

import (
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// ---------------------------------------------------------------------------
// ServerApp / ClientApp implement echowarp.PauseController for the daemon
// REST API POST /api/v1/pause and /api/v1/resume endpoints. "Pause" in this
// context means "stop advancing the outgoing audio frame counter" — the
// send pipeline stays wired and playback keeps running so Resume is
// instantaneous.
//
// These adapters live next to toggles_adapter.go (SetMuted /
// SetDiscoveryPublish) so all on/off optional-interface adapters are grouped
// in one place. See runner.go PauseController godoc for the full contract.
// ---------------------------------------------------------------------------

// SetPaused implements echowarp.PauseController. It writes the serverPaused
// atomic (consulted by newServerPauseFilterCh on every outgoing frame) and
// mirrors the notification side-effects of the TUI pause-loops so connected
// peers learn about the pause state and the conference mixer keeps its
// "server" participant flag in sync.
//
// Idempotent: calling SetPaused(true) twice is equivalent to once. Safe for
// concurrent use — the underlying flag is an atomic.Bool and the
// broadcastParticipantPause helper takes s.mu.RLock internally.
func (s *ServerApp) SetPaused(paused bool) error {
	s.serverPaused.Store(paused)
	if s.logger != nil {
		if paused {
			s.logger.Info("Server capture paused via API")
		} else {
			s.logger.Info("Server capture resumed via API")
		}
	}

	// Mirror conference-mode TUI pause loop: keep the mixer's "server"
	// participant flag in sync and broadcast the state to all peers so
	// clients update their participant list.
	if s.conference != nil {
		s.conference.SetParticipantPaused("server", paused)
		s.broadcastParticipantPause("server", paused)
		return nil
	}

	// Non-conference multi/single-client mode: notify all connected peers
	// so their TUI / HTTP status reflects the new state. The broadcast is
	// best-effort — peers that have already disconnected are logged at
	// debug level by SendControl.
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mc := range s.clients {
		if mc.peer == nil {
			continue
		}
		if err := mc.peer.SendControl(transport.ActionParticipantPause, map[string]interface{}{
			"participantID": "server",
			"paused":        paused,
		}); err != nil {
			s.logger.Debug("Failed to send server pause control", "error", err)
		}
	}
	return nil
}

// SetPaused implements echowarp.PauseController for ClientApp. Writes the
// capturePaused atomic consulted by newPauseFilterCh on every outgoing
// frame. The client's capture pipeline is only active in
// reverse/duplex/conference modes; in receive-only mode the flag is still
// stored (so IsCapturePaused() reflects the API state), but there is no
// send pipeline to stop — this matches the TUI pauseCh contract.
//
// Idempotent and safe for concurrent use.
func (c *ClientApp) SetPaused(paused bool) error {
	c.capturePaused.Store(paused)
	if c.logger != nil {
		if paused {
			c.logger.Info("Client capture paused via API")
		} else {
			c.logger.Info("Client capture resumed via API")
		}
	}
	return nil
}
