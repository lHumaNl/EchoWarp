package app

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// ServerApp orchestrates the server-side streaming workflow. It handles:
//   - TCP signaling listener (plain or TLS)
//   - Client authentication via HMAC-SHA256 or ECDH
//   - WebRTC connection establishment with ICE/TURN
//   - Multi-client mode when MaxClients > 1
//
// Lifecycle:
//
//	app := NewServerApp(cfg, logger, banMgr, tlsConfig, rateLimiter)
//	err := app.Run(ctx) // Blocks until context cancellation or fatal error
type ServerApp struct {
	cfg         config.Config
	logger      *slog.Logger
	banMgr      ban.BanManager
	auth        transport.AuthHandler
	tlsConfig   *tls.Config
	rateLimiter *auth.IPRateLimiter

	// Factories for creating signalers and peers (enables testing and decoupling)
	signalerFactory SignalerFactory
	peerFactory     PeerFactory

	mu         sync.RWMutex
	clients    map[string]*multiClient
	nextClient int

	// TUI stats reporting channels (optional, nil if not using TUI).
	statsCh chan<- transport.ConnectionStats
	errCh   chan<- error

	// Multi-client stats channel (optional, nil if not using TUI in multi-client mode).
	multiStatsCh chan<- transport.MultiClientStats

	// Command channel for receiving kick/ban commands from TUI.
	cmdCh <-chan ClientCommand

	// Conference mode: mix engine handler (nil when not in conference mode).
	conference *ConferenceHandler

	// AEC processor shared between capture and playback pipelines in duplex mode.
	aec *audio.AECProcessor

	// Spectrum analyzer for TUI visualization (optional).
	// Used for playback/decode path (incoming audio).
	spectrum *audio.SpectrumAnalyzer
	// Level meter for TUI visualization (optional).
	// Used for playback/decode path (incoming audio).
	levelMeter *audio.LevelMeter

	// Capture spectrum analyzer for TUI visualization (optional).
	// Used for capture path (outgoing audio).
	captureSpectrum *audio.SpectrumAnalyzer
	// Capture level meter for TUI visualization (optional).
	// Used for capture path (outgoing audio).
	captureLevelMeter *audio.LevelMeter

	// Conference stats channel for TUI updates (optional).
	conferenceStatsCh chan<- ConferenceStatsPayload

	// Participant command channel (stored eagerly, wired to conference handler on init).
	participantCmdCh <-chan ParticipantCommand

	// Recording command channel for receiving start/stop commands from TUI.
	recordingCmdCh <-chan RecordingCommand

	// recState tracks recording metadata (mode, start time, output
	// directory) that the underlying audio.ConferenceRecorder does not
	// expose directly. Populated by the daemon-API RecordingController
	// adapter in recording_adapter.go and consumed by RecordingStatus
	// / StopRecording. The struct carries its own mutex so callers
	// that hold s.mu do not need to coordinate here.
	recState recordingAdapterState

	// discoveryState owns the on/off lifecycle of the mDNS publisher
	// goroutine spawned by SetDiscoveryPublish (phase 5d). The state
	// lives next to recState for symmetry and to keep toggle adapters
	// grouped — see toggles_adapter.go for the SetDiscoveryPublish
	// implementation.
	discoveryState discoveryAdapterState

	// Callback invoked when client count changes (for probe/session info).
	onClientCount func(int)

	// Graceful stop: closed when TUI requests shutdown (before context cancellation).
	stopCh <-chan struct{}

	// UDP mux for constraining all WebRTC ICE traffic to a single UDP port.
	udpMux *transport.UDPMuxManager

	// Chat hub for multi-client text chat (created in Run).
	chatHub *ChatHub

	// Optional channel for forwarding chat messages to TUI.
	chatMsgCh chan<- ChatMessage

	// chatEventHook is an optional callback invoked on every ChatHub onMessage
	// fan-out. It is intended for the daemon/API layer to re-emit chat traffic
	// as an EventChatMessage on the Node's EventBus so WebSocket subscribers
	// can receive chat in real time. nil when the server is driven by the TUI
	// only.
	chatEventHook func(ChatMessage)

	// Auto-increment counter for assigning default nicknames ("Client-N").
	nextNickname int

	// serverPauseCh receives pause toggle requests for the server's capture stream from TUI.
	serverPauseCh <-chan bool

	// clientMuted is set when the single client requests server to stop sending audio.
	// Used only in single-client (non-conference) mode. Atomic for lock-free access
	// from the audio forwarding goroutine.
	clientMuted atomic.Bool

	// clientPaused is set when the client pauses its capture (outgoing) stream.
	// When true, the server drops incoming audio frames from this client.
	// Used only in single-client reverse/duplex mode. Atomic for lock-free access.
	clientPaused atomic.Bool

	// serverPaused is set when the server operator pauses capture via TUI (ctrl+p).
	// When true, the server drops outgoing audio frames from its capture device.
	serverPaused atomic.Bool

	// sessions stores session entries for reconnect support.
	// Protected by mu (same mutex as clients).
	sessions map[string]*sessionEntry

	// statsHook is an optional callback invoked on every stats tick with the
	// same ConnectionStats value that would be sent to statsCh. The daemon
	// wires it to Node.UpdateStats so GET /api/v1/stats returns live numbers
	// without requiring a TUI statsCh. Safe to leave nil; additive to the
	// TUI path — both sinks receive the same values.
	statsHook func(transport.ConnectionStats)

	// onClientJoin / onClientLeave are optional callbacks invoked on every
	// client register/unregister event in both single- and multi-client
	// modes. The daemon wires them to Node.AddClient/Node.RemoveClient so
	// GET /api/v1/clients returns the real live roster. Safe to leave nil;
	// additive to the TUI path.
	onClientJoin  func(clientID, remoteAddr string)
	onClientLeave func(clientID string)

	// deviceCmdCh is the internal channel into which device control commands
	// (mute/volume) are pushed by HandleDeviceCommand. A consumer goroutine
	// that actually applies the commands to the mixer is wired up by the CLI
	// / task 013 — the app layer only owns the buffered channel so the API
	// layer has a non-blocking place to deliver commands.
	deviceCmdCh chan DeviceCommand

	// participantCmdChAPI is the internal channel into which participant
	// control commands (mute/kick/volume) originating from the HTTP API are
	// pushed by HandleParticipantCommand. It is intentionally distinct from
	// the TUI-provided participantCmdCh (<-chan, owned by the TUI layer) —
	// the two channels can coexist and are drained independently until
	// task 013 unifies them. The consumer that applies API-level commands
	// to the conference handler is wired up by task 013; until then the
	// channel is a bounded buffer that accepts commands and returns 200
	// from the API.
	participantCmdChAPI chan ParticipantCommand
}

// sessionEntry stores data needed to restore a client's identity on reconnect.
type sessionEntry struct {
	clientID     string // Last client ID assigned to this session.
	nickname     string // Nickname at the time of disconnect.
	isCustomNick bool   // True if the nickname was chosen by the client (not server-assigned).
}

// multiClient tracks state for a connected client in multi-client mode.
type multiClient struct {
	id            string                // Unique client identifier.
	nickname      string                // Display name for chat.
	hwid          string                // Hardware identifier (for bans, if collected).
	sessionID     string                // UUID assigned by server (for reconnect).
	isCustomNick  bool                  // True if nickname was client-chosen (not server-assigned).
	conn          net.Conn              // TCP signaling connection.
	peer          transport.PeerManager // WebRTC peer connection (interface for decoupling).
	joinedAt      time.Time             // Connection timestamp.
	muted         atomic.Bool           // Per-client mute: server stops sending audio to this client.
	mutedOutgoing atomic.Bool           // Server-initiated mute: server stops sending audio to this client.
	mutedIncoming atomic.Bool           // Server-initiated mute: server stops receiving audio from this client.
	paused        atomic.Bool           // Per-client pause: client paused its capture.
}

// NewServerApp creates a new server application with the given configuration.
// All parameters except banMgr, tlsConfig, and rateLimiter can be nil.
// Uses default factories if none provided.
func NewServerApp(cfg config.Config, logger *slog.Logger, banMgr ban.BanManager, tlsConfig *tls.Config, rateLimiter *auth.IPRateLimiter) *ServerApp {
	return &ServerApp{
		cfg:                 cfg,
		logger:              logger,
		banMgr:              banMgr,
		auth:                auth.NewAuthHandler(cfg.IsTLSEnabled(), cfg.Password),
		tlsConfig:           tlsConfig,
		rateLimiter:         rateLimiter,
		signalerFactory:     NewTCPSignalerFactory(),
		peerFactory:         NewWebRTCPeerFactory(),
		clients:             make(map[string]*multiClient),
		sessions:            make(map[string]*sessionEntry),
		deviceCmdCh:         make(chan DeviceCommand, 16),
		participantCmdChAPI: make(chan ParticipantCommand, 16),
	}
}

// ParticipantCommandChannel returns the internal participant command channel
// used for API-originated commands. Consumers (task 013) read from it to
// apply commands to the conference handler. Returns nil only for zero-valued
// ServerApps produced in tests that skip NewServerApp.
//
// This is distinct from the TUI-provided channel wired via
// WithParticipantCommandChannel — the two channels coexist until task 013
// unifies the delivery paths.
func (s *ServerApp) ParticipantCommandChannel() <-chan ParticipantCommand {
	return s.participantCmdChAPI
}

// DeviceCommandChannel returns the internal device command channel. Consumers
// (e.g. the CLI-level HandleDeviceCommands goroutine wired in task 013) read
// from this channel to apply commands to the mixer. Returns nil only for
// zero-valued ServerApps produced in tests that skip NewServerApp.
func (s *ServerApp) DeviceCommandChannel() <-chan DeviceCommand {
	return s.deviceCmdCh
}

// WithStatsChannels configures optional channels for reporting statistics to TUI.
func (s *ServerApp) WithStatsChannels(statsCh chan<- transport.ConnectionStats, errCh chan<- error) *ServerApp {
	s.statsCh = statsCh
	s.errCh = errCh
	return s
}

// WithStatsHook installs a callback invoked on every stats tick (and on the
// final "disconnected" stat) with the same ConnectionStats value that would
// be delivered to the TUI statsCh. Primarily used by the daemon to forward
// live stats into Node.UpdateStats so GET /api/v1/stats reflects non-zero
// bytes during streaming. Safe to pass nil (equivalent to unset); additive
// to the TUI path — both sinks receive the same values.
func (s *ServerApp) WithStatsHook(fn func(transport.ConnectionStats)) *ServerApp {
	s.statsHook = fn
	return s
}

// WithClientTrackingHooks installs callbacks invoked on every client
// register / unregister event in both single- and multi-client modes. The
// daemon wires these to Node.AddClient / Node.RemoveClient so
// GET /api/v1/clients returns the real live roster. Safe to pass nil
// (equivalent to unset); additive to the TUI path — the TUI observes the
// same events via the onClientCount callback.
func (s *ServerApp) WithClientTrackingHooks(
	onJoin func(clientID, remoteAddr string),
	onLeave func(clientID string),
) *ServerApp {
	s.onClientJoin = onJoin
	s.onClientLeave = onLeave
	return s
}

// WithSpectrum sets a spectrum analyzer fed from captured PCM (or decoded PCM in duplex mode).
func (s *ServerApp) WithSpectrum(sa *audio.SpectrumAnalyzer) *ServerApp {
	s.spectrum = sa
	return s
}

// WithLevelMeter sets a level meter that will be fed PCM data for per-channel VU display.
func (s *ServerApp) WithLevelMeter(lm *audio.LevelMeter) *ServerApp {
	s.levelMeter = lm
	return s
}

// WithCaptureSpectrum sets a spectrum analyzer for the capture (outgoing) audio path.
func (s *ServerApp) WithCaptureSpectrum(sa *audio.SpectrumAnalyzer) *ServerApp {
	s.captureSpectrum = sa
	return s
}

// WithCaptureLevelMeter sets a level meter for the capture (outgoing) audio path.
func (s *ServerApp) WithCaptureLevelMeter(lm *audio.LevelMeter) *ServerApp {
	s.captureLevelMeter = lm
	return s
}

// WithStopChannel sets the stop channel for graceful shutdown signaling from TUI.
func (s *ServerApp) WithStopChannel(ch <-chan struct{}) *ServerApp {
	s.stopCh = ch
	return s
}

// WithMultiStatsChannel configures a channel for reporting multi-client stats to TUI.
func (s *ServerApp) WithMultiStatsChannel(ch chan<- transport.MultiClientStats) *ServerApp {
	s.multiStatsCh = ch
	return s
}

// WithClientCountCallback sets a callback invoked when the number of connected clients changes.
func (s *ServerApp) WithClientCountCallback(fn func(int)) *ServerApp {
	s.onClientCount = fn
	return s
}

// notifyClientCount calls the onClientCount callback with the current number of clients.
// Must be called with s.mu held (at least RLock).
func (s *ServerApp) notifyClientCount() {
	if s.onClientCount != nil {
		s.onClientCount(len(s.clients))
	}
}

// WithCommandChannel configures a channel for receiving commands from TUI.
func (s *ServerApp) WithCommandChannel(ch <-chan ClientCommand) *ServerApp {
	s.cmdCh = ch
	return s
}

// GetConferenceHandler returns the conference handler (nil if not in conference mode).
func (s *ServerApp) GetConferenceHandler() *ConferenceHandler {
	return s.conference
}

// AEC returns the AEC processor (nil if AEC is not enabled).
func (s *ServerApp) AEC() *audio.AECProcessor {
	return s.aec
}

// WithConferenceStatsChannel sets the channel for reporting conference participant states to TUI.
func (s *ServerApp) WithConferenceStatsChannel(ch chan<- ConferenceStatsPayload) *ServerApp {
	s.conferenceStatsCh = ch
	return s
}

// WithParticipantCommandChannel sets the channel for receiving participant commands from TUI.
// The channel is stored and wired to the conference handler when conference mode initializes.
func (s *ServerApp) WithParticipantCommandChannel(ch <-chan ParticipantCommand) *ServerApp {
	s.participantCmdCh = ch
	return s
}

// WithRecordingCommandChannel sets the channel for receiving recording start/stop commands from TUI.
func (s *ServerApp) WithRecordingCommandChannel(ch <-chan RecordingCommand) *ServerApp {
	s.recordingCmdCh = ch
	return s
}

// WithChatChannel configures an optional channel for forwarding chat messages to TUI.
func (s *ServerApp) WithChatChannel(ch chan<- ChatMessage) *ServerApp {
	s.chatMsgCh = ch
	return s
}

// WithChatEventHook installs a callback invoked on every chat message fan-out
// from the ChatHub. The hook is primarily used by the daemon to re-emit chat
// traffic as EventChatMessage on the Node's EventBus so WebSocket clients
// receive chat in real time. Safe to pass nil (equivalent to unset).
func (s *ServerApp) WithChatEventHook(fn func(ChatMessage)) *ServerApp {
	s.chatEventHook = fn
	return s
}

// WithServerPauseChannel sets the channel for receiving server participant pause toggles from TUI.
func (s *ServerApp) WithServerPauseChannel(ch <-chan bool) *ServerApp {
	s.serverPauseCh = ch
	return s
}

// SendChatMessage sends a message from "Server" to chat participants.
// If toNickname is non-empty, the message is sent as a DM.
func (s *ServerApp) SendChatMessage(text string, toNickname ...string) {
	if s.chatHub != nil {
		s.chatHub.SendFromServer(text, toNickname...)
	}
}

// SendChat implements echowarp.ChatSender. It sends a chat message from the
// server via the ChatHub. Empty to broadcasts; non-empty to is delivered as a
// DM to that nickname. Returns an ErrNotRunning error if the ChatHub has not
// been initialized yet (e.g., ServerApp constructed but Run not called, or
// Run has already exited).
func (s *ServerApp) SendChat(text, to string) error {
	if s.chatHub == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Server chat hub not initialized").
			WithSuggestion("Start the server before sending chat messages")
	}
	if to == "" {
		s.chatHub.SendFromServer(text)
	} else {
		s.chatHub.SendFromServer(text, to)
	}
	return nil
}

// GetChatHub returns the chat hub (nil if not yet initialized).
func (s *ServerApp) GetChatHub() *ChatHub {
	return s.chatHub
}

// Run starts the server and blocks until the context is canceled or a fatal error occurs.
// In single-client mode (MaxClients <= 1), handles one client at a time sequentially.
// In multi-client mode (MaxClients > 1), accepts concurrent connections up to MaxClients.
func (s *ServerApp) Run(ctx context.Context) error {
	// Ensure any mDNS publisher started via the daemon API
	// (SetDiscoveryPublish) is torn down when Run exits, so a Node.Stop
	// followed by a fresh Node.Start does not leak the zeroconf
	// goroutine.
	defer func() { _ = s.SetDiscoveryPublish(false) }() //nolint:errcheck // adapter never errors on disable

	// Initialize AEC processor for duplex mode.
	if s.cfg.AEC && (s.cfg.Duplex || s.cfg.Conference) {
		s.aec = audio.NewAECProcessor(audio.DefaultAECConfig())
		s.logger.Info("AEC enabled", "filterLen", audio.DefaultAECConfig().FilterLength)
	}

	// Initialize chat hub for text chat support.
	s.chatHub = NewChatHub(s.logger, s.cfg.MaxClients, func(msg ChatMessage) {
		if s.chatMsgCh != nil {
			select {
			case s.chatMsgCh <- msg:
			default:
			}
		}
		if s.chatEventHook != nil {
			s.chatEventHook(msg)
		}
	})

	// Create shared UDP mux so all WebRTC connections use a single UDP port.
	udpMux, muxErr := transport.NewUDPMuxManager(s.cfg.Port)
	if muxErr != nil {
		s.logger.Warn("Failed to create UDP mux, ICE will use ephemeral ports", "error", muxErr)
	} else {
		s.udpMux = udpMux
		s.logger.Info("ICE UDP mux enabled", "udp_port", s.cfg.Port)
		defer func() {
			_ = udpMux.Close()
			s.udpMux = nil
		}()
	}

	var err error
	if s.cfg.MaxClients <= 1 {
		err = s.runSingle(ctx)
	} else {
		err = s.runMulti(ctx)
	}
	if s.errCh != nil && err != nil {
		select {
		case s.errCh <- err:
		default:
			s.logger.Error("Error channel full, error dropped", "error", err)
		}
	}
	return err
}
