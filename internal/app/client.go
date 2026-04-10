package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// ClientApp orchestrates the client-side streaming workflow. It handles:
//   - TCP signaling connection to server (plain or TLS)
//   - Authentication via HMAC-SHA256 or ECDH
//   - WebRTC connection establishment
//   - Audio playback or capture depending on direction
//
// Lifecycle:
//
//	app := NewClientApp(cfg, logger, tlsConfig)
//	err := app.Run(ctx) // Blocks until context cancellation or fatal error
type ClientApp struct {
	cfg       config.Config
	logger    *slog.Logger
	auth      transport.AuthHandler
	tlsConfig *tls.Config

	// Factories for creating signalers and peers (enables testing and decoupling)
	signalerFactory SignalerFactory
	peerFactory     PeerFactory

	// TUI stats reporting channels (optional, nil if not using TUI).
	statsCh chan<- transport.ConnectionStats
	errCh   chan<- error

	// Graceful stop: closed when TUI requests shutdown (before context cancellation).
	stopCh <-chan struct{}

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

	// Recording support (client-side).
	recorder       *audio.ConferenceRecorder
	recorderMu     sync.Mutex
	recordingCmdCh <-chan RecordingCommand

	// recState tracks recording metadata for the daemon-API
	// RecordingController adapter. See internal/app/recording_adapter.go
	// for the rationale (same pattern as ServerApp.recState).
	recState recordingAdapterState

	// Chat client for text messaging.
	chatClient *ChatClient

	// Optional channel for forwarding chat messages to TUI.
	chatMsgCh chan<- ChatMessage

	// chatEventHook is an optional callback invoked on every chat message the
	// ChatClient receives (broadcast, DM, system). The daemon/API layer uses
	// it to re-emit chat traffic as EventChatMessage on the Node's EventBus
	// so WebSocket subscribers receive chat in real time.
	chatEventHook func(ChatMessage)

	// Optional channel to notify TUI of server-assigned nickname.
	chatNicknameCh chan<- string

	// Optional channel for forwarding participant list updates to TUI.
	participantsCh chan<- ChatParticipantsPayload

	// serverStoppedCh is closed when the server sends ActionStop (graceful shutdown).
	// Used by TUI to distinguish from unexpected disconnects.
	serverStoppedCh   chan struct{}
	serverStoppedOnce sync.Once

	// serverMuteCh receives mute toggle requests from TUI (true=mute, false=unmute).
	// The client sends peer_mute/peer_unmute control messages to the server.
	serverMuteCh <-chan bool

	// capturePaused is set when the user pauses capture (outgoing audio).
	// The capture pipeline checks this and drops frames when true.
	capturePaused atomic.Bool

	// serverMutedIncoming is set when server mutes incoming from this client.
	// The capture pipeline checks this and drops encoded frames when true.
	serverMutedIncoming atomic.Bool

	// incomingMuted is toggled via MuteController.SetMuted (daemon API
	// POST /api/v1/mute). When set, the jitter playback pump zeros
	// decoded frames before writing them to the playback device — the
	// pipeline keeps running so timing stays stable and unmute is
	// instantaneous. Distinct from serverMutedIncoming which is the
	// server-initiated capture-side mute exposed by the peer_mute
	// control message.
	incomingMuted atomic.Bool

	// pauseCh receives pause toggle requests from TUI.
	// true=pause, false=resume. The client sends pause/resume control messages to the server.
	pauseCh <-chan bool

	// participantPauseCh sends participant pause state changes to TUI.
	participantPauseCh chan<- ParticipantPauseMsg

	// kickedByServer is set when server sends kick_notify.
	// Prevents auto-reconnect after disconnect.
	kickedByServer atomic.Bool
	kickReason     string

	// bannedByServer is set when server sends ban_notify.
	// Prevents auto-reconnect after disconnect.
	bannedByServer atomic.Bool
	banReason      string
	banCriteria    []string

	// conferencePartsCh sends full participants list updates to TUI.
	conferencePartsCh chan<- ConferenceParticipantsMsg

	// sessionID stores the server-assigned session UUID for reconnect support.
	// Persists in memory across reconnect attempts within the same process.
	sessionID string

	// deviceCmdCh is the internal channel into which device control commands
	// (mute/volume) are pushed by HandleDeviceCommand. Owned by the runner
	// so the API layer has a non-blocking place to deliver commands. The
	// consumer goroutine that actually applies the commands to the mixer
	// is wired up by the CLI / task 013.
	deviceCmdCh chan DeviceCommand

	// participantCmdChAPI is the internal channel for participant control
	// commands originating from the HTTP API. Client mode does not own the
	// conference mixer, so commands pushed here are accepted (for API
	// contract consistency with server mode) but never applied — the
	// consumer is a no-op. See Participants() / HandleParticipantCommand
	// in participant_handler.go.
	participantCmdChAPI chan ParticipantCommand
}

// NewClientApp creates a new client application with the given configuration.
// The tlsConfig should be non-nil when TLS is enabled for signaling.
// Uses default factories if none provided.
func NewClientApp(cfg config.Config, logger *slog.Logger, tlsConfig *tls.Config) *ClientApp {
	return &ClientApp{
		cfg:                 cfg,
		logger:              logger,
		auth:                auth.NewAuthHandler(cfg.IsTLSEnabled(), cfg.Password),
		tlsConfig:           tlsConfig,
		signalerFactory:     NewTCPSignalerFactory(),
		peerFactory:         NewWebRTCPeerFactory(),
		deviceCmdCh:         make(chan DeviceCommand, 16),
		participantCmdChAPI: make(chan ParticipantCommand, 16),
	}
}

// ParticipantCommandChannel returns the internal participant command channel
// used for API-originated commands. Returns nil only for zero-valued
// ClientApps built outside NewClientApp. See server.go ParticipantCommandChannel
// for the server-mode counterpart; in client mode commands are accepted but
// never applied.
func (c *ClientApp) ParticipantCommandChannel() <-chan ParticipantCommand {
	return c.participantCmdChAPI
}

// DeviceCommandChannel returns the internal device command channel for
// consumers (e.g. the CLI-level HandleDeviceCommands goroutine wired in task
// 013). Returns nil only for zero-valued ClientApps built outside NewClientApp.
func (c *ClientApp) DeviceCommandChannel() <-chan DeviceCommand {
	return c.deviceCmdCh
}

// WithStatsChannels configures optional channels for reporting statistics to TUI.
func (c *ClientApp) WithStatsChannels(statsCh chan<- transport.ConnectionStats, errCh chan<- error) *ClientApp {
	c.statsCh = statsCh
	c.errCh = errCh
	return c
}

// WithSpectrum sets a spectrum analyzer fed from captured PCM (or decoded PCM in duplex mode).
func (c *ClientApp) WithSpectrum(sa *audio.SpectrumAnalyzer) *ClientApp {
	c.spectrum = sa
	return c
}

// WithLevelMeter sets a level meter that will be fed PCM data for per-channel VU display.
func (c *ClientApp) WithLevelMeter(lm *audio.LevelMeter) *ClientApp {
	c.levelMeter = lm
	return c
}

// WithCaptureSpectrum sets a spectrum analyzer for the capture (outgoing) audio path.
func (c *ClientApp) WithCaptureSpectrum(sa *audio.SpectrumAnalyzer) *ClientApp {
	c.captureSpectrum = sa
	return c
}

// WithCaptureLevelMeter sets a level meter for the capture (outgoing) audio path.
func (c *ClientApp) WithCaptureLevelMeter(lm *audio.LevelMeter) *ClientApp {
	c.captureLevelMeter = lm
	return c
}

// WithStopChannel sets the stop channel for graceful shutdown signaling from TUI.
func (c *ClientApp) WithStopChannel(ch <-chan struct{}) *ClientApp {
	c.stopCh = ch
	return c
}

// WithRecordingCommandChannel sets the channel for receiving recording start/stop commands from TUI.
func (c *ClientApp) WithRecordingCommandChannel(ch <-chan RecordingCommand) *ClientApp {
	c.recordingCmdCh = ch
	return c
}

// WithChatChannel configures an optional channel for forwarding chat messages to TUI.
func (c *ClientApp) WithChatChannel(ch chan<- ChatMessage) *ClientApp {
	c.chatMsgCh = ch
	return c
}

// WithChatEventHook installs a callback invoked on every chat message the
// ChatClient receives. The hook is primarily used by the daemon to re-emit
// chat traffic as EventChatMessage on the Node's EventBus so WebSocket
// clients receive chat in real time. Safe to pass nil (equivalent to unset).
func (c *ClientApp) WithChatEventHook(fn func(ChatMessage)) *ClientApp {
	c.chatEventHook = fn
	return c
}

// WithChatNicknameChannel sets the channel to notify TUI of server-assigned nickname.
func (c *ClientApp) WithChatNicknameChannel(ch chan<- string) *ClientApp {
	c.chatNicknameCh = ch
	return c
}

// WithParticipantsChannel sets the channel for forwarding participant list updates to TUI.
func (c *ClientApp) WithParticipantsChannel(ch chan<- ChatParticipantsPayload) *ClientApp {
	c.participantsCh = ch
	return c
}

// WithParticipantPauseChannel sets the channel for forwarding participant pause state changes to TUI.
func (c *ClientApp) WithParticipantPauseChannel(ch chan<- ParticipantPauseMsg) *ClientApp {
	c.participantPauseCh = ch
	return c
}

// WithConferenceParticipantsChannel sets the channel for forwarding full participant list updates to TUI.
func (c *ClientApp) WithConferenceParticipantsChannel(ch chan<- ConferenceParticipantsMsg) *ClientApp {
	c.conferencePartsCh = ch
	return c
}

// WithServerStoppedChannel sets the channel that will be closed when the server
// sends ActionStop (graceful shutdown). TUI listens on this to transition to
// the server-stopped screen instead of triggering RunWithReconnect.
func (c *ClientApp) WithServerStoppedChannel(ch chan struct{}) *ClientApp {
	c.serverStoppedCh = ch
	return c
}

// WithServerMuteChannel sets the channel for receiving server mute toggle requests from TUI.
func (c *ClientApp) WithServerMuteChannel(ch <-chan bool) *ClientApp {
	c.serverMuteCh = ch
	return c
}

// WithPauseChannel sets the channel for receiving capture pause toggle requests from TUI.
func (c *ClientApp) WithPauseChannel(ch <-chan bool) *ClientApp {
	c.pauseCh = ch
	return c
}

// IsCapturePaused returns whether capture is currently paused.
func (c *ClientApp) IsCapturePaused() bool {
	return c.capturePaused.Load()
}

// SendChatMessage sends a text message via the chat client.
// If toNickname is non-empty, the message is sent as a DM.
func (c *ClientApp) SendChatMessage(text string, toNickname ...string) error {
	if c.chatClient != nil {
		return c.chatClient.Send(text, toNickname...)
	}
	return fmt.Errorf("chat client not initialized")
}

// SendChat implements echowarp.ChatSender. It sends a chat message to the
// server via the ChatClient. Empty to broadcasts; non-empty to is delivered
// as a DM to that nickname. Returns an ErrNotRunning error if the ChatClient
// has not been initialized yet (e.g., ClientApp constructed but Run not
// called, or Run has already exited), or any send error from the underlying
// data channel (bubbled up via ErrInternalState).
func (c *ClientApp) SendChat(text, to string) error {
	if c.chatClient == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Client chat channel not initialized").
			WithSuggestion("Start the client before sending chat messages")
	}
	var err error
	if to == "" {
		err = c.chatClient.Send(text)
	} else {
		err = c.chatClient.Send(text, to)
	}
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to send chat message")
	}
	return nil
}

// GetChatClient returns the chat client (nil if not yet initialized).
func (c *ClientApp) GetChatClient() *ChatClient {
	return c.chatClient
}

// startRecordingInternal starts client-side recording. Public entry
// points: the TUI bridge (processRecordingCommands) and the daemon API
// adapter (recording_adapter.go) both call this helper. Renamed from
// the former exported StartRecording in phase 5c so the ClientApp type
// can satisfy echowarp.RecordingController with the public-typed
// signature without a name clash.
func (c *ClientApp) startRecordingInternal(mode audio.RecordingMode) error {
	c.recorderMu.Lock()
	defer c.recorderMu.Unlock()
	c.recorder = audio.NewConferenceRecorder(mode, c.cfg.SampleRate, 1)
	homeDir, _ := os.UserHomeDir() //nolint:errcheck
	baseDir := filepath.Join(homeDir, "Documents", "EchoWarp_records")
	return c.recorder.Start(baseDir)
}

// stopRecordingInternal stops client-side recording. See
// startRecordingInternal for why this is unexported.
func (c *ClientApp) stopRecordingInternal() (time.Duration, uint64, int, error) {
	c.recorderMu.Lock()
	defer c.recorderMu.Unlock()
	if c.recorder == nil {
		return 0, 0, 0, nil
	}
	dur, size, files, err := c.recorder.Stop()
	c.recorder = nil
	return dur, size, files, err
}

// IsRecording returns whether client-side recording is active.
func (c *ClientApp) IsRecording() bool {
	c.recorderMu.Lock()
	defer c.recorderMu.Unlock()
	return c.recorder != nil && c.recorder.IsActive()
}

// processRecordingCommands handles recording start/stop commands from TUI.
func (c *ClientApp) processRecordingCommands(ctx context.Context) {
	if c.recordingCmdCh == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-c.recordingCmdCh:
			if !ok {
				return
			}
			if cmd.Start {
				if err := c.startRecordingInternal(cmd.Mode); err != nil {
					c.logger.Error("Failed to start recording", "error", err)
				} else {
					c.logger.Info("Recording started", "mode", cmd.Mode)
				}
			} else {
				dur, size, files, err := c.stopRecordingInternal()
				if err != nil {
					c.logger.Error("Failed to stop recording", "error", err)
				} else {
					c.logger.Info("Recording stopped",
						"duration", dur.Round(time.Second),
						"files", files,
						"size", formatBytes(size))
				}
			}
		}
	}
}

// flushRecordingHeaders periodically flushes WAV headers for crash safety.
func (c *ClientApp) flushRecordingHeaders(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.recorderMu.Lock()
			rec := c.recorder
			c.recorderMu.Unlock()
			if rec != nil && rec.IsActive() {
				rec.FlushHeaders()
			}
		}
	}
}

// reportStats periodically sends connection statistics to the TUI stats channel.
// Started immediately when the message loop begins (before DC ready), so the TUI
// transitions to the streaming screen as soon as PeerConnection reaches "connected".
func (c *ClientApp) reportStats(ctx context.Context, peer transport.PeerManager) {
	if c.statsCh == nil {
		return
	}
	// Send initial stat immediately.
	select {
	case c.statsCh <- peer.GetStats():
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
			case c.statsCh <- peer.GetStats():
			default:
			}
		}
	}
}

// sendDisconnected sends a final "disconnected" stats update to the TUI.
func (c *ClientApp) sendDisconnected() {
	if c.statsCh == nil {
		return
	}
	select {
	case c.statsCh <- transport.ConnectionStats{State: "disconnected"}:
	default:
	}
}

// Run connects to the server and starts streaming. It blocks until the context
// is canceled, authentication fails, or a fatal error occurs.
func (c *ClientApp) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Reset kick/ban state from any previous session.
	c.kickedByServer.Store(false)
	c.bannedByServer.Store(false)

	// Probe server to get/refresh audio parameters before connecting.
	// On reconnect this detects config changes since the last session.
	probeResult, probeErr := probe.ProbeServer(c.cfg.Address, c.cfg.Port)
	if probeErr != nil {
		c.logger.Warn("Pre-connect probe failed, using existing config", "error", probeErr)
	} else {
		changes := probe.CompareServerParams(c.cfg, probeResult)
		for _, ch := range changes {
			if ch.Critical {
				return ewerrors.NewError(ewerrors.ErrConfigInvalid,
					fmt.Sprintf("server configuration changed critically: %s %s -> %s",
						ch.Field, ch.OldValue, ch.NewValue))
			}
		}
		// Apply non-critical changes.
		if len(changes) > 0 {
			_ = probe.ApplyProbeToConfig(&c.cfg, probeResult)
			attrs := make([]any, 0, len(changes)*2)
			for _, ch := range changes {
				attrs = append(attrs, ch.Field, fmt.Sprintf("%s -> %s", ch.OldValue, ch.NewValue))
			}
			c.logger.Info("Server config updated from probe", attrs...)
		}
	}

	connStart := time.Now()
	serverAddr := fmt.Sprintf("%s:%d", c.cfg.Address, c.cfg.Port)
	c.logger.Info("Connecting to server", "address", serverAddr, "mode", c.cfg.AudioMode())

	signaler, sigCtx, sigCancel, err := c.initializeSignaler(ctx, serverAddr)
	if err != nil {
		return err
	}
	defer sigCancel()

	protocol, authResult, err := c.authenticate(ctx, signaler)
	if err != nil {
		return err
	}

	// Store session ID for reconnect support.
	if authResult != nil && authResult.SessionID != "" {
		c.sessionID = authResult.SessionID
	}

	// Use server-assigned nickname if available, otherwise fall back to config.
	chatNickname := c.cfg.Nickname
	if authResult != nil && authResult.Nickname != "" {
		chatNickname = authResult.Nickname
	}

	// Notify TUI of the resolved nickname.
	if c.chatNicknameCh != nil && chatNickname != "" {
		select {
		case c.chatNicknameCh <- chatNickname:
		default:
		}
	}

	// Initialize chat client with the resolved nickname.
	c.chatClient = NewChatClient(chatNickname, func(msg ChatMessage) {
		if c.chatMsgCh != nil {
			select {
			case c.chatMsgCh <- msg:
			default:
			}
		}
		if c.chatEventHook != nil {
			c.chatEventHook(msg)
		}
	})

	// Wire participant list updates to TUI channel.
	if c.participantsCh != nil {
		c.chatClient.SetOnParticipants(func(payload ChatParticipantsPayload) {
			select {
			case c.participantsCh <- payload:
			default:
			}
		})
	}

	peer, cleanup, err := c.initializePeer(signaler)
	if err != nil {
		return err
	}
	defer cleanup()

	audioDone, err := c.initializeAudioPipeline(sigCtx, peer, signaler)
	if err != nil {
		return err
	}

	sessCtx, sessCancel := context.WithCancel(sigCtx)
	defer sessCancel()

	// Start recording command handler and header flusher.
	go c.processRecordingCommands(sessCtx)
	go c.flushRecordingHeaders(sessCtx)
	// Stop any active recording when session ends (e.g. disconnect/reconnect).
	defer func() { _, _, _, _ = c.stopRecordingInternal() }() //nolint:errcheck

	c.setupPeerCallbacks(peer, sessCancel)

	if err := protocol.HandleHandshake(ctx, signaler, peer); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrSignalingFailed, "WebRTC handshake failed")
	}

	runErr := c.runMessageLoop(sessCtx, signaler, peer, audioDone, connStart)

	// If kicked or banned, override the error with a structured error code
	// so that RunWithReconnect and TUI can detect it.
	if c.kickedByServer.Load() {
		kickErr := ewerrors.NewError(ewerrors.ErrClientKicked, "kicked by server").
			WithContext("reason", c.kickReason)
		// Error is returned to RunWithReconnect which forwards it to errCh via CLI.
		return kickErr
	}
	if c.bannedByServer.Load() {
		banErr := ewerrors.NewError(ewerrors.ErrClientBannedByAdmin, "banned by server").
			WithContext("reason", c.banReason).
			WithContext("criteria", c.banCriteria)
		return banErr
	}

	// context.Canceled means the session ended via normal disconnect (peer state callback);
	// TUI is informed via the "disconnected" stats sent by reportStats, so don't show it as an error.
	if c.errCh != nil && runErr != nil && !errors.Is(runErr, context.Canceled) {
		select {
		case c.errCh <- runErr:
		default:
			c.logger.Error("Error channel full, error dropped", "error", runErr)
		}
	}
	return runErr
}

// initializeSignaler creates and starts the TCP signaler.
// Returns the signaler, context, cancel function, and any error.
func (c *ClientApp) initializeSignaler(ctx context.Context, serverAddr string) (transport.Signaler, context.Context, context.CancelFunc, error) {
	signaler := c.createSignaler(serverAddr)
	sigCtx, sigCancel := context.WithCancel(ctx)

	startErrCh := make(chan error, 1)
	go func() {
		if err := signaler.Start(sigCtx); err != nil && sigCtx.Err() == nil {
			c.logger.Error("Signaling error", "error", err)
			startErrCh <- err
		}
	}()

	select {
	case <-signaler.Ready():
		c.logger.Info("Connected to server", "addr", serverAddr)
	case err := <-startErrCh:
		sigCancel()
		return nil, nil, nil, err
	case <-sigCtx.Done():
		sigCancel()
		return nil, nil, nil, sigCtx.Err()
	}

	return signaler, sigCtx, sigCancel, nil
}

// initializePeer creates the WebRTC peer connection.
// Returns the peer, a cleanup function, and any error.
func (c *ClientApp) initializePeer(signaler transport.Signaler) (transport.PeerManager, func(), error) {
	peer, err := c.createPeer()
	if err != nil {
		_ = signaler.Close() //nolint:errcheck
		return nil, nil, err
	}
	cleanup := func() {
		_ = peer.Close() //nolint:errcheck
	}
	return peer, cleanup, nil
}

// initializeAudioPipeline sets up the audio pipeline and returns the audio done channel.
func (c *ClientApp) initializeAudioPipeline(ctx context.Context, peer transport.PeerManager, signaler transport.Signaler) (<-chan error, error) {
	bufSize := 1
	if c.cfg.Duplex {
		bufSize = 2
	}
	audioDone := make(chan error, bufSize)
	if err := c.setupAudioPipeline(ctx, peer, audioDone); err != nil {
		_ = signaler.Close() //nolint:errcheck
		return nil, err
	}
	return audioDone, nil
}

// setupPeerCallbacks configures WebRTC peer connection callbacks.
func (c *ClientApp) setupPeerCallbacks(peer transport.PeerManager, cancel context.CancelFunc) {
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		c.logger.Info("Connection state changed", "state", state.String())
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			cancel()
		}
	})
}

// authenticate handles the authentication handshake with the server.
// Returns the protocol instance for subsequent handshake operations.
func (c *ClientApp) authenticate(ctx context.Context, signaler transport.Signaler) (*transport.ClientSignalingProtocol, *transport.AuthResultMessage, error) {
	protocol := transport.NewClientSignalingProtocol(c.auth)
	if err := protocol.HandleAuth(ctx, signaler, c.cfg.Password); err != nil {
		metrics.AuthAttempts.WithLabelValues(metrics.StatusFailed).Inc()
		_ = signaler.Close() //nolint:errcheck
		return nil, nil, ewerrors.Wrap(err, ewerrors.ErrAuthFailed, "server authentication failed")
	}
	metrics.AuthAttempts.WithLabelValues(metrics.StatusSuccess).Inc()
	c.logger.Info("Authenticated successfully")

	// Send client metadata (nickname, HWID, session ID) per spec §2.2.
	meta := transport.ClientAuthMeta{
		Nickname: c.cfg.Nickname,
	}
	if c.cfg.HWIDRequired {
		hwid, err := auth.LoadOrGenerateHWID()
		if err != nil {
			c.logger.Warn("Failed to generate HWID", "error", err)
		} else {
			meta.HWID = hwid
		}
	}
	// Send sessionID from previous session for reconnect support.
	if c.sessionID != "" {
		meta.SessionID = c.sessionID
	}
	if err := protocol.SendClientMeta(signaler, meta); err != nil {
		c.logger.Warn("Failed to send client meta", "error", err)
	}

	// Wait for server's auth result containing assigned nickname and session ID.
	authResult, err := protocol.WaitForAuthResult(signaler)
	if err != nil {
		c.logger.Warn("Failed to receive auth result", "error", err)
		return protocol, nil, nil
	}
	if !authResult.Success {
		return nil, nil, ewerrors.NewError(ewerrors.ErrAuthFailed, authResult.Message)
	}

	return protocol, authResult, nil
}

func (c *ClientApp) createSignaler(serverAddr string) transport.Signaler {
	return c.signalerFactory.CreateClientSignaler(serverAddr, c.tlsConfig)
}

func (c *ClientApp) createPeer() (transport.PeerManager, error) {
	direction := transport.DirectionReceive
	if c.cfg.Duplex {
		direction = transport.DirectionDuplex
	} else if c.cfg.Reverse {
		direction = transport.DirectionSend
	}

	iceConfig := buildICEConfig(c.cfg, nil)

	peer, err := c.peerFactory.CreatePeer(direction, iceConfig)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "create peer connection")
	}
	return peer, nil
}

func (c *ClientApp) setupAudioPipeline(ctx context.Context, peer transport.PeerManager, audioDone chan error) error {
	if c.cfg.Duplex {
		return c.setupDuplexAudioPipeline(ctx, peer, audioDone)
	}
	if c.cfg.Reverse {
		return c.setupSendAudioPipeline(ctx, peer, audioDone)
	}
	return c.setupReceiveAudioPipeline(ctx, peer, audioDone)
}

// setupDuplexAudioPipeline creates both capture and playback pipelines for duplex mode.
func (c *ClientApp) setupDuplexAudioPipeline(ctx context.Context, peer transport.PeerManager, audioDone chan error) error {
	// Send pipeline (capture from mic)
	if err := c.setupSendAudioPipeline(ctx, peer, audioDone); err != nil {
		return err
	}
	// Receive pipeline (playback to speaker)
	return c.setupReceiveAudioPipeline(ctx, peer, audioDone)
}

// setupSendAudioPipeline creates audio capture pipeline for reverse mode (client sends).
func (c *ClientApp) setupSendAudioPipeline(ctx context.Context, peer transport.PeerManager, audioDone chan error) error {
	sendCh, err := peer.AddAudioTrack(c.cfg.SampleRate, c.cfg.Channels)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "add audio track")
	}
	filteredSendCh := c.newPauseFilterCh(ctx, sendCh)
	filteredSendCh = c.newServerMuteIncomingFilterCh(ctx, filteredSendCh)
	go func() {
		audioDone <- c.runCapturePipeline(ctx, filteredSendCh)
	}()
	return nil
}

// newPauseFilterCh creates a forwarding channel that drops audio frames when capturePaused is set.
func (c *ClientApp) newPauseFilterCh(ctx context.Context, dst chan<- []byte) chan<- []byte {
	src := make(chan []byte, cap(dst))
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-src:
				if !ok {
					return
				}
				if c.capturePaused.Load() {
					audio.PutOpusOutput(data)
					continue
				}
				select {
				case dst <- data:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return src
}

// newServerMuteIncomingFilterCh creates a forwarding channel that drops audio frames
// when the server has muted incoming audio from this client.
func (c *ClientApp) newServerMuteIncomingFilterCh(ctx context.Context, dst chan<- []byte) chan<- []byte {
	src := make(chan []byte, cap(dst))
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-src:
				if !ok {
					return
				}
				if c.serverMutedIncoming.Load() {
					audio.PutOpusOutput(data)
					continue
				}
				select {
				case dst <- data:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return src
}

// setupReceiveAudioPipeline creates audio playback pipeline for normal mode (client receives).
// Uses JitterBuffer for adaptive buffering based on network conditions.
// The client's incoming-mute flag (toggled via SetMuted / Node.SetMuted from
// the daemon API) is passed to the pump so mute takes effect without
// tearing down the playback device.
func (c *ClientApp) setupReceiveAudioPipeline(ctx context.Context, peer transport.PeerManager, audioDone chan error) error {
	startJitteredPlayback(ctx, c.logger, c.cfg, peer, c.spectrum, c.levelMeter, audioDone, &c.incomingMuted)
	return nil
}

func (c *ClientApp) runMessageLoop(ctx context.Context, signaler transport.Signaler, peer transport.PeerManager, audioDone <-chan error, connStart time.Time) error {
	var tcpClosed bool
	var statsWg sync.WaitGroup
	defer func() {
		if !tcpClosed && signaler != nil {
			_ = signaler.Close() //nolint:errcheck
		}
		// Wait for reportStats to finish, then send "disconnected".
		// Guarantees TUI receives "disconnected" after the last "connected" stat
		// and before RunWithReconnect can start a new session.
		statsWg.Wait()
		c.sendDisconnected()
	}()

	dcReady := peer.DCReady()
	receiveCh := signaler.Receive()
	var controlCh <-chan []byte // nil until DC is open
	// stopCh may be nil if not using TUI; use a never-closed channel as fallback.
	stopCh := c.stopCh
	if stopCh == nil {
		stopCh = make(chan struct{})
	}

	// Start stats reporting immediately so the TUI can transition to the streaming
	// screen as soon as the PeerConnection reaches "connected" state.
	if c.statsCh != nil {
		statsWg.Add(1)
		go func() {
			defer statsWg.Done()
			c.reportStats(ctx, peer)
		}()
	}

	var chatCh <-chan []byte // nil until chat DC is open

	// Poll for control channel availability (same as server — DC may open before dc_ready).
	controlReady := make(chan (<-chan []byte), 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ch := peer.ControlMessages(); ch != nil {
					controlReady <- ch
					return
				}
			}
		}
	}()

	// Poll for chat channel availability.
	chatReady := make(chan (<-chan []byte), 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ch := peer.ChatMessages(); ch != nil {
					chatReady <- ch
					return
				}
			}
		}
	}()

	for {
		select {
		case ch := <-controlReady:
			controlCh = ch
			controlReady = nil
		case ch := <-chatReady:
			chatCh = ch
			chatReady = nil
			// Wire up the send function now that chat DC is open.
			if c.chatClient != nil {
				c.chatClient.SetSendFn(peer.SendChat)
			}
		case <-stopCh:
			// Graceful stop requested by TUI: send stop via DC or TCP signaling.
			c.logger.Info("Graceful stop requested")
			if err := peer.SendControl(transport.ActionStop, nil); err != nil {
				c.logger.Debug("Failed to send stop via DataChannel, trying TCP signaling", "error", err)
				if !tcpClosed {
					_ = sendControlViaTCP(signaler, transport.ActionStop) //nolint:errcheck
				}
			}
			// Brief pause to let the message flush, then close peer immediately
			// so the deferred statsWg.Wait() doesn't block on ctx.Done().
			time.Sleep(150 * time.Millisecond)
			_ = peer.Close() //nolint:errcheck
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-dcReady:
			dcReady = nil
			receiveCh = nil // TCP is closing; nil prevents infinite reads on closed channel
			if controlCh == nil {
				controlCh = peer.ControlMessages()
			}
			if chatCh == nil {
				chatCh = peer.ChatMessages()
				if chatCh != nil && c.chatClient != nil {
					c.chatClient.SetSendFn(peer.SendChat)
				}
			}
			c.logger.Info("DataChannel ready, closing TCP signaling")
			_ = signaler.Close() //nolint:errcheck
			tcpClosed = true
		case paused := <-c.pauseCh:
			c.capturePaused.Store(paused)
			action := transport.ActionPauseAll
			if !paused {
				action = transport.ActionResumeAll
			}
			if err := peer.SendControl(action, nil); err != nil {
				c.logger.Warn("Failed to send pause control", "action", action, "error", err)
			} else {
				c.logger.Info("Sent pause control", "action", action)
			}
		case muted := <-c.serverMuteCh:
			action := transport.ActionPeerMute
			if !muted {
				action = transport.ActionPeerUnmute
			}
			if err := peer.SendControl(action, nil); err != nil {
				c.logger.Warn("Failed to send server mute control", "action", action, "error", err)
			} else {
				c.logger.Info("Sent server mute control", "action", action)
			}
		case raw := <-chatCh:
			if c.chatClient != nil {
				c.chatClient.HandleIncoming(raw)
			}
		case raw := <-controlCh:
			if c.handleDCControl(raw) {
				c.recordMetrics(connStart)
				_ = peer.Close() //nolint:errcheck
				return nil
			}
		case msg := <-receiveCh:
			if c.handleSignalingMessage(msg, peer) {
				c.recordMetrics(connStart)
				_ = peer.Close() //nolint:errcheck
				return nil
			}
		case err := <-audioDone:
			if err != nil && ctx.Err() == nil {
				return ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "audio pipeline failed")
			}
			c.recordMetrics(connStart)
			return nil
		}
	}
}

func (c *ClientApp) handleSignalingMessage(msg transport.SignalingMessage, peer transport.PeerManager) bool {
	switch msg.Type {
	case "candidate":
		handleCandidateMsg(c.logger, msg.Payload, peer)
	case "candidate_done":
		c.logger.Debug("Remote ICE gathering complete")
	case "control":
		if handleControlMsg(c.logger, msg.Payload) {
			c.logger.Info("Server requested stop")
			c.closeServerStoppedCh()
			return true
		}
	}
	return false
}

// handleDCControl processes a raw control message received via WebRTC DataChannel.
// Returns true if the session should end (e.g. stop action).
func (c *ClientApp) handleDCControl(raw []byte) bool {
	var msg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return false
	}
	if msg.Type != transport.TypeControl {
		return false
	}

	// Parse action.
	var ctrl struct {
		Action string          `json:"action"`
		Data   json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(msg.Payload, &ctrl); err != nil {
		return false
	}

	switch ctrl.Action {
	case transport.ActionStop:
		c.logger.Info("Server requested stop (via DataChannel)")
		c.closeServerStoppedCh()
		return true

	case transport.ActionParticipantPause:
		var data struct {
			ParticipantID string `json:"participantID"`
			Paused        bool   `json:"paused"`
		}
		if err := json.Unmarshal(ctrl.Data, &data); err != nil {
			c.logger.Warn("Failed to parse participant_pause data", "error", err)
			return false
		}
		c.logger.Debug("Participant pause state changed", "participantID", data.ParticipantID, "paused", data.Paused)
		if c.participantPauseCh != nil {
			select {
			case c.participantPauseCh <- ParticipantPauseMsg{ParticipantID: data.ParticipantID, Paused: data.Paused}:
			default:
			}
		}

	case transport.ActionKickNotify:
		var data struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(ctrl.Data, &data)
		c.kickedByServer.Store(true)
		c.kickReason = data.Reason
		if data.Reason != "" {
			c.logger.Info("Kicked by server", "reason", data.Reason)
		} else {
			c.logger.Info("Kicked by server")
		}
		return false // connection will close from server side

	case transport.ActionBanNotify:
		var data struct {
			Reason   string   `json:"reason"`
			Criteria []string `json:"criteria"`
		}
		_ = json.Unmarshal(ctrl.Data, &data)
		c.bannedByServer.Store(true)
		c.banReason = data.Reason
		c.banCriteria = data.Criteria
		c.logger.Info("Banned by server", "reason", data.Reason, "criteria", data.Criteria)
		return false // connection will close from server side

	case transport.ActionMuteOutgoing:
		c.logger.Info("[MUTED BY SERVER] outgoing")
		return false

	case transport.ActionUnmuteOutgoing:
		c.logger.Info("[UNMUTED BY SERVER] outgoing")
		return false

	case transport.ActionMuteIncoming:
		c.logger.Info("[MUTED BY SERVER] incoming — server stopped receiving our audio")
		c.serverMutedIncoming.Store(true)
		return false

	case transport.ActionUnmuteIncoming:
		c.logger.Info("[UNMUTED BY SERVER] incoming — server resumed receiving our audio")
		c.serverMutedIncoming.Store(false)
		return false

	case transport.ActionParticipantsUpdate:
		var data struct {
			Participants []struct {
				ID       string `json:"id"`
				Nickname string `json:"nickname"`
				Paused   bool   `json:"paused"`
			} `json:"participants"`
		}
		if err := json.Unmarshal(ctrl.Data, &data); err != nil {
			c.logger.Warn("Failed to parse participants_update data", "error", err)
			return false
		}
		c.logger.Debug("Participants update received", "count", len(data.Participants))
		if c.conferencePartsCh != nil {
			parts := make([]ConferenceParticipantInfo, len(data.Participants))
			for i, p := range data.Participants {
				parts[i] = ConferenceParticipantInfo{ID: p.ID, Nickname: p.Nickname, Paused: p.Paused}
			}
			select {
			case c.conferencePartsCh <- ConferenceParticipantsMsg{Participants: parts}:
			default:
			}
		}
	}

	return false
}

// closeServerStoppedCh safely closes the serverStoppedCh channel using sync.Once
// to prevent double-close panics.
func (c *ClientApp) closeServerStoppedCh() {
	// Clear session ID on graceful server shutdown — the server will not recognize it after restart.
	c.sessionID = ""
	if c.serverStoppedCh != nil {
		c.serverStoppedOnce.Do(func() { close(c.serverStoppedCh) })
	}
}

func (c *ClientApp) recordMetrics(connStart time.Time) {
	metrics.ConnectionDuration.Observe(time.Since(connStart).Seconds())
	metrics.ConnectionsTotal.WithLabelValues(metrics.RoleClient, metrics.StatusSuccess).Inc()
}

func (c *ClientApp) runCapturePipeline(ctx context.Context, sendCh chan<- []byte) error {
	encCfg := EncoderConfig{
		Bitrate:     c.cfg.OpusBitrate,
		Application: c.cfg.OpusApplication,
	}

	// In duplex mode, separate analyzers are used for capture vs playback.
	// In non-duplex mode, captureSpectrum may be nil — fall back to c.spectrum.
	captureSpectrum := c.captureSpectrum
	captureLevel := c.captureLevelMeter
	if captureSpectrum == nil {
		captureSpectrum = c.spectrum
	}
	if captureLevel == nil {
		captureLevel = c.levelMeter
	}

	// Multi-device capture
	captureDevices := c.cfg.CaptureDevices()
	if len(captureDevices) > 1 {
		pipeline := NewMultiCapturePipeline(MultiCapturePipelineConfig{
			Devices:       captureDevices,
			SampleRate:    c.cfg.SampleRate,
			Channels:      c.cfg.Channels,
			EncoderConfig: encCfg,
			BufferFrames:  c.cfg.EffectiveAudioBufferFrames(),
			Spectrum:      captureSpectrum,
			LevelMeter:    captureLevel,
			AGCProcessors: buildAGCProcessors(captureDevices, c.cfg.SampleRate),
		}, c.logger)
		return pipeline.Run(ctx, sendCh)
	}

	// Single-device capture
	if c.cfg.DeviceID == nil && len(captureDevices) == 0 {
		return ewerrors.NewError(ewerrors.ErrConfigInvalid, "no device ID specified").
			WithSuggestion("Select an audio device with --device flag or interactive selection")
	}

	deviceID := uint32(0)
	if len(captureDevices) == 1 {
		deviceID = captureDevices[0].ID
	} else if c.cfg.DeviceID != nil {
		deviceID = *c.cfg.DeviceID
	}

	// Build AGC processor for single-device capture if AGC is enabled.
	var agcProc *audio.AGCProcessor
	agcMap := buildAGCProcessors(captureDevices, c.cfg.SampleRate)
	if agcMap != nil {
		agcProc = agcMap[deviceID]
	}

	pipeline := NewClientCapturePipeline(CapturePipelineConfig{
		SampleRate:           c.cfg.SampleRate,
		Channels:             c.cfg.Channels,
		DeviceID:             deviceID,
		AudioBufferFrames:    c.cfg.AudioBufferFrames,
		IsLoopback:           c.cfg.Loopback,
		LoopbackOutputDevice: c.cfg.LoopbackOutputDevice,
		LoopbackBlackHole:    c.cfg.LoopbackBlackHole,
		EncoderConfig:        encCfg,
		AGC:                  agcProc,
		Spectrum:             captureSpectrum,
		LevelMeter:           captureLevel,
	}, c.logger)

	return pipeline.Run(ctx, sendCh)
}
