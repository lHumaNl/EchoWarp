package app

import (
	"context"
	"os"
	"sync"

	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// ---------------------------------------------------------------------------
// ClientApp implements echowarp.MuteController for the daemon / REST API
// POST /api/v1/mute endpoint. "Mute" in this context means "drop every
// decoded audio frame that would otherwise be played back" — the
// playback pipeline keeps running so unmute is instant and timing
// stays aligned with incoming RTP. The actual filter lives in
// jitterPlaybackPump (shared.go) which consults c.incomingMuted on
// every frame.
//
// Server-mode (ServerApp) deliberately does not implement
// MuteController: a server has no single incoming stream to mute, and
// per-client mutes are handled via the conference/peer control path.
// The Node wrapper returns ErrInternalState when the runner lacks
// the interface, which the API layer maps to 500 — the caller learns
// the endpoint is wired but the current runner type does not support
// it.
// ---------------------------------------------------------------------------

// SetMuted implements echowarp.MuteController. Idempotent by contract:
// setting the same value twice is a no-op. Safe for concurrent use —
// the underlying flag is an atomic.Bool.
func (c *ClientApp) SetMuted(muted bool) error {
	c.incomingMuted.Store(muted)
	if c.logger != nil {
		c.logger.Info("Incoming audio mute toggled via API", "muted", muted)
	}
	return nil
}

// IsIncomingMuted reports whether incoming audio is currently muted by
// the daemon API. Exported primarily for tests and for the WebSocket
// event bridge (if it ever wants to emit mute state changes).
func (c *ClientApp) IsIncomingMuted() bool {
	return c.incomingMuted.Load()
}

// ---------------------------------------------------------------------------
// ServerApp implements echowarp.DiscoveryPublisher for the daemon / REST
// API POST /api/v1/discovery/publish endpoint. The adapter owns a
// zeroconf publisher goroutine under its own mutex so SetDiscoveryPublish
// never blocks the HTTP handler on network I/O — Start spawns the
// goroutine and returns; Stop cancels the context and waits for the
// goroutine to exit.
//
// The daemon path currently does not auto-start mDNS (see
// internal/cli/daemon.go setupDaemonDiscovery which only logs). This
// adapter is the first place that actually registers a zeroconf service
// from the daemon path, and does so only when the API caller explicitly
// enables publishing. The TUI path continues to use the independent
// setupDiscovery helper in server_config.go — the two call sites are
// intentionally kept separate so the TUI behavior is not regressed by
// this phase.
//
// TODO(task-019-6): when phase 6 wires stats propagation, also refresh
// the TXT "clients" record periodically so the published snapshot does
// not go stale. For now the record reflects the state at the time of
// the enable call (0/maxClients).
// ---------------------------------------------------------------------------

// discoveryAdapterState holds the goroutine-lifecycle bookkeeping for
// ServerApp.SetDiscoveryPublish. A nil cancel means "not publishing".
// All fields must be accessed under mu.
type discoveryAdapterState struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// SetDiscoveryPublish implements echowarp.DiscoveryPublisher.
// Idempotent: enabling an already-published service or disabling an
// already-stopped one returns nil without side effects. Safe for
// concurrent use.
func (s *ServerApp) SetDiscoveryPublish(enabled bool) error {
	s.discoveryState.mu.Lock()

	if enabled {
		if s.discoveryState.cancel != nil {
			// Already publishing — idempotent no-op.
			s.discoveryState.mu.Unlock()
			return nil
		}
		serverName, _ := os.Hostname()
		txt := discovery.BuildTXTRecords(
			"2.0.0",
			serverName,
			s.cfg.Password != "",
			s.cfg.TLSCert != "",
			0,
			s.cfg.MaxClients,
			s.cfg.AudioMode(),
			"", // server_id left empty for daemon path — matches setupDiscovery
		)
		pub, err := discovery.NewPublisher(serverName, s.cfg.Port, txt)
		if err != nil {
			s.discoveryState.mu.Unlock()
			return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to create mDNS publisher")
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		s.discoveryState.cancel = cancel
		s.discoveryState.done = done
		go func() {
			defer close(done)
			if pubErr := pub.Publish(ctx); pubErr != nil && ctx.Err() == nil {
				if s.logger != nil {
					s.logger.Warn("mDNS publishing error", "error", pubErr)
				}
			}
		}()
		if s.logger != nil {
			s.logger.Info("mDNS publishing enabled via API", "name", serverName, "port", s.cfg.Port)
		}
		s.discoveryState.mu.Unlock()
		return nil
	}

	// Disable path.
	if s.discoveryState.cancel == nil {
		// Already stopped — idempotent no-op.
		s.discoveryState.mu.Unlock()
		return nil
	}
	cancel := s.discoveryState.cancel
	done := s.discoveryState.done
	s.discoveryState.cancel = nil
	s.discoveryState.done = nil
	// Release the adapter mutex before waiting for the publisher
	// goroutine to exit — the zeroconf library's Shutdown() is
	// synchronous and can block briefly on socket cleanup, and we
	// want concurrent status queries (IsDiscoveryPublishing) to
	// observe the cleared state immediately.
	s.discoveryState.mu.Unlock()
	cancel()
	if done != nil {
		<-done
	}
	if s.logger != nil {
		s.logger.Info("mDNS publishing disabled via API")
	}
	return nil
}

// IsDiscoveryPublishing reports whether the mDNS publisher goroutine is
// currently running. Exported for tests.
func (s *ServerApp) IsDiscoveryPublishing() bool {
	s.discoveryState.mu.Lock()
	defer s.discoveryState.mu.Unlock()
	return s.discoveryState.cancel != nil
}
