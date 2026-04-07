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
		cfg:             cfg,
		logger:          logger,
		banMgr:          banMgr,
		auth:            auth.NewAuthHandler(cfg.IsTLSEnabled(), cfg.Password),
		tlsConfig:       tlsConfig,
		rateLimiter:     rateLimiter,
		signalerFactory: NewTCPSignalerFactory(),
		peerFactory:     NewWebRTCPeerFactory(),
		clients:         make(map[string]*multiClient),
		sessions:        make(map[string]*sessionEntry),
	}
}

// WithStatsChannels configures optional channels for reporting statistics to TUI.
func (s *ServerApp) WithStatsChannels(statsCh chan<- transport.ConnectionStats, errCh chan<- error) *ServerApp {
	s.statsCh = statsCh
	s.errCh = errCh
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

// GetChatHub returns the chat hub (nil if not yet initialized).
func (s *ServerApp) GetChatHub() *ChatHub {
	return s.chatHub
}

// Run starts the server and blocks until the context is canceled or a fatal error occurs.
// In single-client mode (MaxClients <= 1), handles one client at a time sequentially.
// In multi-client mode (MaxClients > 1), accepts concurrent connections up to MaxClients.
func (s *ServerApp) Run(ctx context.Context) error {
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
