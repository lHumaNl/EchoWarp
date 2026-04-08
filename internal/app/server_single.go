package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// remoteAddrConn is a minimal net.Conn implementation used to register a
// single-client in s.clients without having a real net.Conn reference.
// Only RemoteAddr and Close are meaningful; all other methods are no-ops.
type remoteAddrConn struct {
	addr    string
	closeFn func() error
}

type remoteAddrAddr string

func (a remoteAddrAddr) Network() string { return "tcp" }
func (a remoteAddrAddr) String() string  { return string(a) }

func (r *remoteAddrConn) RemoteAddr() net.Addr { return remoteAddrAddr(r.addr) }
func (r *remoteAddrConn) Close() error {
	if r.closeFn != nil {
		return r.closeFn()
	}
	return nil
}
func (r *remoteAddrConn) LocalAddr() net.Addr              { return remoteAddrAddr("") }
func (r *remoteAddrConn) Read([]byte) (int, error)         { return 0, net.ErrClosed }
func (r *remoteAddrConn) Write([]byte) (int, error)        { return 0, net.ErrClosed }
func (r *remoteAddrConn) SetDeadline(time.Time) error      { return nil }
func (r *remoteAddrConn) SetReadDeadline(time.Time) error  { return nil }
func (r *remoteAddrConn) SetWriteDeadline(time.Time) error { return nil }

func (s *ServerApp) runSingle(ctx context.Context) error {
	listenAddr := fmt.Sprintf(":%d", s.cfg.Port)
	s.logger.Info("Starting server", "port", s.cfg.Port, "mode", s.cfg.AudioMode())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.handleOneClient(ctx, listenAddr); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// context.Canceled from sigCtx means the client disconnected (PeerConnection
			// state changed to disconnected/closed). This is normal, not an error.
			if errors.Is(err, context.Canceled) {
				s.logger.Info("Client disconnected")
			} else {
				s.logger.Error("Client session error", "error", err)
			}
		}
	}
}

func (s *ServerApp) handleOneClient(ctx context.Context, listenAddr string) error {
	// Reset mute/pause state for the new client session.
	s.clientMuted.Store(false)
	s.clientPaused.Store(false)
	connStart := time.Now()

	signaler, sigCtx, sigCancel, err := s.createAndStartSignaler(ctx, listenAddr)
	if err != nil {
		return err
	}
	defer sigCancel()
	defer func() { _ = signaler.Close() }() //nolint:errcheck

	remoteAddr, err := s.checkClientAccess(signaler)
	if err != nil {
		return err
	}

	protocol := transport.NewServerSignalingProtocol(s.auth)

	if err = s.authenticateWithProtocol(sigCtx, protocol, signaler, remoteAddr, ""); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrAuthFailed, "client authentication failed")
	}

	// Receive client metadata (nickname, HWID, session ID) per spec §2.2.
	meta, err := protocol.ReceiveClientMeta(sigCtx, signaler)
	if err != nil {
		s.logger.Warn("Failed to receive client meta", "error", err)
		// Use empty meta as fallback for backward compatibility.
		meta = &transport.ClientAuthMeta{}
	}

	// Validate per spec §2.4.
	// 1. Check session reconnect.
	var oldNickname string
	if meta.SessionID != "" {
		oldNickname, _ = s.handleReconnectBySession(meta.SessionID)
	}

	// 2. Validate nickname.
	if errMsg := s.validateNickname(meta.Nickname); errMsg != "" {
		payload, _ := json.Marshal(auth.AuthResultPayload{Success: false, Message: errMsg, Code: 409}) //nolint:errcheck
		_ = signaler.Send(transport.SignalingMessage{Type: "auth_result", Payload: payload})           //nolint:errcheck
		return ewerrors.NewError(ewerrors.ErrAuthFailed, errMsg)
	}

	// 3. Check HWID ban.
	if s.cfg.HWIDRequired && meta.HWID != "" && s.banMgr != nil && s.banMgr.IsHWIDBanned(meta.HWID) {
		payload, _ := json.Marshal(auth.AuthResultPayload{Success: false, Message: "device banned", Code: 403}) //nolint:errcheck
		_ = signaler.Send(transport.SignalingMessage{Type: "auth_result", Payload: payload})                    //nolint:errcheck
		return ewerrors.NewError(ewerrors.ErrClientBanned, "device banned")
	}

	// Assign nickname and session ID.
	nickname := s.assignNickname(meta.Nickname)
	if oldNickname != "" {
		nickname = oldNickname
	}
	sessionID := auth.NewSessionID()

	if err = s.sendAuthResultWithSession(protocol, signaler, sessionID, nickname); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrSignalingFailed, "send server config")
	}

	// Send reconnect system message if applicable.
	if oldNickname != "" && s.chatHub != nil {
		s.chatHub.BroadcastSystem(fmt.Sprintf("%s reconnected", oldNickname))
	}

	peer, direction, err := s.createWebRTCPeer()
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "create WebRTC peer")
	}
	defer func() { _ = peer.Close() }() //nolint:errcheck

	clientID := "client-1"

	// Register single client in s.clients so processCommands (kick/ban/mute) can find it.
	// Must happen before setupAudioPipeline so the mute filter can reference mc.mutedOutgoing.
	isCustomNick := meta.Nickname != ""
	s.mu.Lock()
	mc := &multiClient{
		id:           clientID,
		nickname:     nickname,
		hwid:         meta.HWID,
		sessionID:    sessionID,
		isCustomNick: isCustomNick,
		conn:         &remoteAddrConn{addr: remoteAddr, closeFn: signaler.Close},
		peer:         peer,
		joinedAt:     connStart,
	}
	s.clients[clientID] = mc
	s.notifyClientCount()
	s.mu.Unlock()
	singleGraceful := false
	defer func() {
		s.mu.Lock()
		delete(s.clients, clientID)
		if singleGraceful {
			delete(s.sessions, sessionID)
		} else {
			s.sessions[sessionID] = &sessionEntry{
				clientID:     clientID,
				nickname:     nickname,
				isCustomNick: isCustomNick,
			}
		}
		s.notifyClientCount()
		s.mu.Unlock()
	}()

	audioDone, err := s.setupAudioPipeline(sigCtx, peer, direction, clientID)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDeviceNotFound, "setup audio pipeline")
	}

	// Listen for server pause toggles from TUI (non-conference mode).
	if s.serverPauseCh != nil {
		go s.processServerPauseSingle(sigCtx, peer)
	}

	s.setupConnectionStateCallback(peer, sigCancel)

	if err := peer.CreateControlDataChannel(); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDataChannelFailed, "create control data channel")
	}

	if err := peer.CreateChatDataChannel(); err != nil {
		s.logger.Warn("Failed to create chat data channel", "error", err)
	}

	if err := protocol.HandleHandshake(sigCtx, signaler, peer); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrSignalingFailed, "WebRTC handshake failed")
	}

	// Start command processing and stats reporting for single-client mode.
	go s.processCommands(sigCtx)
	go s.reportMultiStats(sigCtx)

	loopErr := s.handleSignalingLoop(sigCtx, signaler, peer, audioDone, connStart, protocol, clientID, nickname, sessionID)
	// nil error means graceful disconnect (client sent stop, or server TUI stop).
	if loopErr == nil {
		singleGraceful = true
	}
	return loopErr
}

func (s *ServerApp) createAndStartSignaler(ctx context.Context, listenAddr string) (transport.Signaler, context.Context, context.CancelFunc, error) {
	signaler := s.signalerFactory.CreateServerSignaler(listenAddr, s.tlsConfig)

	sigCtx, sigCancel := context.WithCancel(ctx)

	go func() {
		if err := signaler.Start(sigCtx); err != nil && sigCtx.Err() == nil {
			s.logger.Error("Signaling error", "error", err)
		}
	}()

	s.logger.Info("Waiting for client connection...")

	select {
	case <-signaler.Ready():
		s.logger.Info("Client connected", "addr", signaler.RemoteAddr())
	case <-sigCtx.Done():
		sigCancel()
		return nil, nil, nil, sigCtx.Err()
	}

	return signaler, sigCtx, sigCancel, nil
}

func (s *ServerApp) checkClientAccess(signaler transport.Signaler) (string, error) {
	remoteAddr := signaler.RemoteAddr()
	if err := s.validateClientAccess(remoteAddr, ""); err != nil {
		if signaler != nil {
			s.sendBannedMessage(signaler, remoteAddr)
			_ = signaler.Close() //nolint:errcheck
		}
		return "", err
	}
	return remoteAddr, nil
}

func (s *ServerApp) validateClientAccess(remoteAddr, clientID string) error {
	if remoteAddr != "" && s.rateLimiter != nil {
		ip := auth.ExtractIP(remoteAddr)
		if !s.rateLimiter.Allow(ip) {
			if clientID != "" {
				s.logger.Warn("Rate limited connection", "clientID", clientID, "addr", remoteAddr)
			} else {
				s.logger.Warn("Rate limited connection", "addr", remoteAddr)
			}
			metrics.ConnectionsTotal.WithLabelValues(metrics.RoleServer, metrics.StatusFailed).Inc()
			return ewerrors.NewError(ewerrors.ErrRateLimited, "rate limited").
				WithContext("ip", ip)
		}
	}

	if remoteAddr != "" && s.banMgr != nil {
		ip := auth.ExtractIP(remoteAddr)
		if s.banMgr.IsBanned(ip) {
			if clientID != "" {
				s.logger.Warn("Banned client attempted connection", "clientID", clientID, "ip", ip)
			} else {
				s.logger.Warn("Banned client attempted connection", "ip", ip)
			}
			metrics.ConnectionsTotal.WithLabelValues(metrics.RoleServer, metrics.StatusFailed).Inc()
			return ewerrors.NewError(ewerrors.ErrClientBanned, "banned client").
				WithContext("ip", ip)
		}
	}

	return nil
}

func (s *ServerApp) sendBannedMessage(signaler transport.Signaler, remoteAddr string) {
	if s.banMgr != nil && s.banMgr.IsBanned(auth.ExtractIP(remoteAddr)) {
		payload, _ := json.Marshal(auth.AuthResultPayload{ //nolint:errcheck
			Success: false,
			Message: "IP banned",
			Code:    403,
		})
		_ = signaler.Send(transport.SignalingMessage{Type: "auth_result", Payload: payload}) //nolint:errcheck
	}
}

func (s *ServerApp) authenticateWithProtocol(ctx context.Context, protocol *transport.ServerSignalingProtocol, signaler transport.Signaler, remoteAddr, clientID string) error {
	if err := protocol.HandleAuth(ctx, signaler, s.cfg.Password); err != nil {
		metrics.AuthAttempts.WithLabelValues(metrics.StatusFailed).Inc()
		if s.banMgr != nil && remoteAddr != "" {
			s.banMgr.RecordFailure(auth.ExtractIP(remoteAddr))
		}
		_ = signaler.Close() //nolint:errcheck
		logFields := []interface{}{"error", err}
		if remoteAddr != "" {
			logFields = append(logFields, "addr", remoteAddr)
		}
		if clientID != "" {
			logFields = append(logFields, "clientID", clientID)
		}
		s.logger.Warn("Auth failed", logFields...)
		return err
	}
	metrics.AuthAttempts.WithLabelValues(metrics.StatusSuccess).Inc()
	if s.banMgr != nil && remoteAddr != "" {
		s.banMgr.RecordSuccess(auth.ExtractIP(remoteAddr))
	}
	s.logger.Info("Client authenticated", "addr", remoteAddr)
	return nil
}

func (s *ServerApp) buildServerConfig() map[string]interface{} {
	return map[string]interface{}{
		"reverse":          s.cfg.Reverse,
		"duplex":           s.cfg.Duplex,
		"sample_rate":      s.cfg.SampleRate,
		"channels":         s.cfg.Channels,
		"opus_bitrate":     s.cfg.OpusBitrate,
		"opus_complexity":  s.cfg.OpusComplexity,
		"opus_application": s.cfg.OpusApplication,
		"opus_dtx":         s.cfg.OpusDTX,
		"opus_fec":         s.cfg.OpusFEC,
		"max_clients":      s.cfg.MaxClients,
		"conference":       s.cfg.Conference,
		"server_muted":     s.cfg.ServerMuted,
	}
}

func (s *ServerApp) sendAuthResultViaProtocol(protocol *transport.ServerSignalingProtocol, signaler transport.Signaler) error {
	return s.sendAuthResultWithSession(protocol, signaler, "", "")
}

func (s *ServerApp) sendAuthResultWithSession(protocol *transport.ServerSignalingProtocol, signaler transport.Signaler, sessionID, nickname string) error {
	configJSON, _ := json.Marshal(s.buildServerConfig()) //nolint:errcheck
	if err := protocol.SendAuthResult(signaler, transport.AuthResultMessage{
		Success:   true,
		Message:   "Authenticated",
		SessionID: sessionID,
		Nickname:  nickname,
		Config:    configJSON,
	}); err != nil {
		_ = signaler.Close() //nolint:errcheck
		return ewerrors.Wrap(err, ewerrors.ErrSignalingFailed, "send auth result")
	}
	return nil
}

func (s *ServerApp) createWebRTCPeer() (transport.PeerManager, transport.MediaDirection, error) {
	direction := transport.DirectionSend
	if s.cfg.Duplex {
		direction = transport.DirectionDuplex
	} else if s.cfg.Reverse {
		direction = transport.DirectionReceive
	}

	iceConfig := buildICEConfig(s.cfg, s.udpMux)

	peer, err := s.peerFactory.CreatePeer(direction, iceConfig)
	if err != nil {
		return nil, direction, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "create peer connection")
	}
	return peer, direction, nil
}

func (s *ServerApp) setupAudioPipeline(ctx context.Context, peer transport.PeerManager, direction transport.MediaDirection, clientID string) (<-chan error, error) {
	audioDone := make(chan error, 2) // buffer 2 for duplex (capture + playback)

	// Get multiClient entry for server-initiated mute (mutedOutgoing) flag.
	s.mu.RLock()
	mc := s.clients[clientID]
	s.mu.RUnlock()

	switch direction {
	case transport.DirectionDuplex:
		// Duplex: both capture (send) and playback (receive) simultaneously
		sendCh, err := peer.AddAudioTrack(s.cfg.SampleRate, s.cfg.Channels)
		if err != nil {
			return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "add audio track")
		}
		filteredSendCh := newCombinedMuteFilterCh(ctx, sendCh, &s.clientMuted, &mc.mutedOutgoing)
		filteredSendCh = s.newServerPauseFilterCh(ctx, filteredSendCh)
		go func() {
			audioDone <- s.runCapturePipeline(ctx, filteredSendCh)
		}()
		s.setupReverseAudio(ctx, peer, audioDone)
	case transport.DirectionSend:
		sendCh, err := peer.AddAudioTrack(s.cfg.SampleRate, s.cfg.Channels)
		if err != nil {
			return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "add audio track")
		}
		filteredSendCh := newCombinedMuteFilterCh(ctx, sendCh, &s.clientMuted, &mc.mutedOutgoing)
		filteredSendCh = s.newServerPauseFilterCh(ctx, filteredSendCh)
		go func() {
			audioDone <- s.runCapturePipeline(ctx, filteredSendCh)
		}()
	default:
		s.setupReverseAudio(ctx, peer, audioDone)
	}
	return audioDone, nil
}

// processServerPauseSingle listens for pause toggle from TUI in non-conference single-client mode.
// Notifies the connected client via datachannel so it can display the pause status.
func (s *ServerApp) processServerPauseSingle(ctx context.Context, peer transport.PeerManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case paused, ok := <-s.serverPauseCh:
			if !ok {
				return
			}
			s.serverPaused.Store(paused)
			if paused {
				s.logger.Info("Server capture paused")
			} else {
				s.logger.Info("Server capture resumed")
			}
			// Notify client about server pause state.
			_ = peer.SendControl(transport.ActionParticipantPause, map[string]interface{}{
				"participantID": "server",
				"paused":        paused,
			})
		}
	}
}

// newServerPauseFilterCh creates a forwarding channel that drops audio frames when serverPaused is set.
// Used to implement server-side pause of its own capture stream.
func (s *ServerApp) newServerPauseFilterCh(ctx context.Context, dst chan<- []byte) chan<- []byte {
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
				if s.serverPaused.Load() {
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

// newMuteFilterCh creates a forwarding channel that drops audio frames when the given flag is set.
// Used to implement server-side mute requested by the client (per-client in multi-client mode).
func newMuteFilterCh(ctx context.Context, dst chan<- []byte, flag *atomic.Bool) chan<- []byte {
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
				if flag.Load() {
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

// newCombinedMuteFilterCh creates a forwarding channel that drops audio frames when
// either flag1 or flag2 is true. Used for combining client-initiated and server-initiated mute.
func newCombinedMuteFilterCh(ctx context.Context, dst chan<- []byte, flag1, flag2 *atomic.Bool) chan<- []byte {
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
				if flag1.Load() || flag2.Load() {
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

func (s *ServerApp) setupReverseAudio(ctx context.Context, peer transport.PeerManager, audioDone chan<- error) {
	s.setupReverseAudioWithID(ctx, peer, audioDone, "")
}

// setupReverseAudioWithID sets up the receive audio pipeline with JitterBuffer.
// In conference mode, decoded audio is submitted to the conference mixer under the given clientID.
// AEC reference feeding is also done here.
func (s *ServerApp) setupReverseAudioWithID(ctx context.Context, peer transport.PeerManager, audioDone chan<- error, clientID string) {
	s.setupReverseAudioMuted(ctx, peer, audioDone, clientID, nil)
}

// setupReverseAudioMuted sets up the receive audio pipeline with JitterBuffer.
// In conference mode, decoded audio is submitted to the conference mixer under the given clientID.
// AEC reference feeding is also done here.
// If muteIncomingFlag is non-nil and true, incoming frames are dropped before processing.
func (s *ServerApp) setupReverseAudioMuted(ctx context.Context, peer transport.PeerManager, audioDone chan<- error, clientID string, muteIncomingFlag *atomic.Bool) {
	// If we need to intercept audio (AEC or conference or incoming mute), use channel-based decode
	// with JitterBuffer between intercept and player.
	needsIntercept := s.aec != nil || (s.conference != nil && clientID != "") || muteIncomingFlag != nil

	if !needsIntercept {
		// Simple path: decoder → JitterBuffer → player.
		startJitteredPlayback(ctx, s.logger, s.cfg, peer, s.spectrum, s.levelMeter, audioDone)
		return
	}

	// Intercept path: decoder → channel → intercept (AEC/conference/muteIncoming) → JitterBuffer → player.
	bufFrames := s.cfg.EffectiveAudioBufferFrames()
	decodeCh := make(chan []float32, bufFrames)

	// When muteIncomingFlag is present, spectrum/levelMeter are fed in the intercept goroutine
	// AFTER the mute check, so that muted audio doesn't show on spectrum/VU.
	var decSpectrum *audio.SpectrumAnalyzer
	var decLevel *audio.LevelMeter
	if muteIncomingFlag == nil {
		decSpectrum = s.spectrum
		decLevel = s.levelMeter
	}
	setupAudioDecoder(s.logger, peer, s.cfg.SampleRate, s.cfg.Channels, decodeCh, decSpectrum, decLevel)

	targetFrames := s.cfg.EffectiveAudioBufferFrames()
	maxFrames := targetFrames * 3
	if maxFrames < 10 {
		maxFrames = 10
	}
	if maxFrames > 30 {
		maxFrames = 30
	}
	jb := audio.NewJitterBuffer(targetFrames, maxFrames)
	frameSize := int(s.cfg.SampleRate) / 50 * int(s.cfg.Channels)

	playbackCh := make(chan []float32, 2)
	doneCh := make(chan struct{})

	// Intercept goroutine: decode → muteIncoming check → AEC/conference → JitterBuffer.
	go func() {
		defer close(doneCh)
		for {
			select {
			case <-ctx.Done():
				return
			case samples, ok := <-decodeCh:
				if !ok {
					return
				}
				// Drop frame if incoming mute is active.
				if muteIncomingFlag != nil && muteIncomingFlag.Load() {
					continue
				}
				// Feed spectrum/VU after mute filter (only when muteIncomingFlag is present,
				// otherwise they are fed directly in setupAudioDecoder).
				if muteIncomingFlag != nil {
					if s.spectrum != nil {
						s.spectrum.Feed(samples)
					}
					if s.levelMeter != nil {
						s.levelMeter.Feed(samples)
					}
				}
				if s.aec != nil {
					s.aec.FeedReference(samples)
				}
				if s.conference != nil && clientID != "" {
					s.conference.SubmitAudio(clientID, samples)
				}
				jb.Write(samples)
			}
		}
	}()

	go jitterPlaybackPump(ctx, jb, playbackCh, frameSize, int(s.cfg.SampleRate), int(s.cfg.Channels), s.logger, doneCh)
	startAudioPlayer(ctx, s.logger, s.cfg, playbackCh, audioDone)

	s.logger.Info("Jitter buffer enabled (intercept path)",
		"target", targetFrames,
		"max", maxFrames,
		"aec", s.aec != nil,
		"conference", clientID != "",
		"muteIncoming", muteIncomingFlag != nil,
	)
}

// setupConferenceReceiver sets up the receive-only pipeline for conference mode:
// decode client audio → submit to conference mixer. No JitterBuffer, no audio player.
// This prevents the server from playing client audio through its speakers (feedback loop).
func (s *ServerApp) setupConferenceReceiver(ctx context.Context, peer transport.PeerManager, audioDone chan<- error, clientID string, muteIncomingFlag *atomic.Bool) {
	bufFrames := s.cfg.EffectiveAudioBufferFrames()
	decodeCh := make(chan []float32, bufFrames)

	// Spectrum/level fed after mute check when muteIncomingFlag is present.
	var decSpectrum *audio.SpectrumAnalyzer
	var decLevel *audio.LevelMeter
	if muteIncomingFlag == nil {
		decSpectrum = s.spectrum
		decLevel = s.levelMeter
	}
	setupAudioDecoder(s.logger, peer, s.cfg.SampleRate, s.cfg.Channels, decodeCh, decSpectrum, decLevel)

	// Intercept goroutine: decode → muteIncoming check → AEC/conference submit.
	go func() {
		defer func() {
			audioDone <- nil
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case samples, ok := <-decodeCh:
				if !ok {
					return
				}
				if muteIncomingFlag != nil && muteIncomingFlag.Load() {
					continue
				}
				if muteIncomingFlag != nil {
					if s.spectrum != nil {
						s.spectrum.Feed(samples)
					}
					if s.levelMeter != nil {
						s.levelMeter.Feed(samples)
					}
				}
				if s.aec != nil {
					s.aec.FeedReference(samples)
				}
				if s.conference != nil && clientID != "" {
					s.conference.SubmitAudio(clientID, samples)
				}
			}
		}
	}()

	s.logger.Info("Conference receiver started (no local playback)",
		"clientID", clientID,
		"aec", s.aec != nil,
		"muteIncoming", muteIncomingFlag != nil,
	)
}

func (s *ServerApp) setupConnectionStateCallback(peer transport.PeerManager, cancel context.CancelFunc) {
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		s.logger.Info("Connection state changed", "state", state.String())
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			cancel()
		}
	})
}

func (s *ServerApp) handleSignalingLoop(ctx context.Context, signaler transport.Signaler, peer transport.PeerManager, audioDone <-chan error, connStart time.Time, protocol *transport.ServerSignalingProtocol, clientID, nickname, sessionID string) error { //nolint:revive,unparam // unified signature with multi-mode
	var tcpClosed bool
	var statsWg sync.WaitGroup
	chatRegistered := false
	defer func() {
		if chatRegistered && s.chatHub != nil {
			s.chatHub.Unregister(clientID)
		}
		if !tcpClosed && signaler != nil {
			_ = signaler.Close() //nolint:errcheck
		}
		// Wait for reportStats to finish, then send "disconnected".
		// This guarantees the TUI receives "disconnected" only after the last
		// "connected" stat, and before the next client session can start.
		statsWg.Wait()
		s.sendDisconnected()
	}()

	dcReady := peer.DCReady()
	receiveCh := signaler.Receive()
	var controlCh <-chan []byte // nil until DC is open
	var chatCh <-chan []byte    // nil until chat DC is open
	// stopCh may be nil if not using TUI; use a never-closed channel as fallback.
	stopCh := s.stopCh
	if stopCh == nil {
		stopCh = make(chan struct{})
	}

	// Start stats reporting immediately so the TUI can transition to the streaming
	// screen as soon as the PeerConnection reaches "connected" state.
	if s.statsCh != nil {
		statsWg.Add(1)
		go func() {
			defer statsWg.Done()
			s.reportStats(ctx, peer)
		}()
	}

	// Poll for control channel availability: the DC opens after ICE connects,
	// which can happen before (or without) the dc_ready handshake completing.
	// This goroutine makes control messages available as soon as the DC is open.
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
			controlReady = nil // prevent re-select
		case ch := <-chatReady:
			chatCh = ch
			chatReady = nil
			// Register with chat hub now that chat DC is open.
			if s.chatHub != nil && !chatRegistered {
				s.chatHub.Register(clientID, nickname, peer.SendChat)
				chatRegistered = true
			}
		case <-stopCh:
			// Graceful stop requested by TUI: send stop via DC or TCP signaling.
			s.logger.Info("Graceful stop requested")
			if err := peer.SendControl(transport.ActionStop, nil); err != nil {
				s.logger.Debug("Failed to send stop via DataChannel, trying TCP signaling", "error", err)
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
			// controlCh may already be set via controlReady poll; update if not.
			if controlCh == nil {
				controlCh = peer.ControlMessages()
			}
			if chatCh == nil {
				chatCh = peer.ChatMessages()
				if chatCh != nil && s.chatHub != nil && !chatRegistered {
					s.chatHub.Register(clientID, nickname, peer.SendChat)
					chatRegistered = true
				}
			}
			s.logger.Info("DataChannel ready, closing TCP signaling")
			_ = signaler.Close() //nolint:errcheck
			tcpClosed = true
		case raw, ok := <-controlCh:
			if !ok || len(raw) == 0 {
				return nil
			}
			if s.handleDCControl(raw, connStart, nickname) {
				_ = peer.Close() //nolint:errcheck
				return nil
			}
		case raw, ok := <-chatCh:
			if !ok || len(raw) == 0 {
				continue
			}
			if s.chatHub != nil {
				s.chatHub.HandleIncoming(clientID, raw)
			}
		case msg := <-receiveCh:
			if done := s.handleSignalingMessage(msg, peer, connStart, protocol); done {
				_ = peer.Close() //nolint:errcheck
				return nil
			}
		case err := <-audioDone:
			if err != nil && ctx.Err() == nil {
				return ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "audio pipeline failed")
			}
			metrics.ConnectionDuration.Observe(time.Since(connStart).Seconds())
			metrics.ConnectionsTotal.WithLabelValues(metrics.RoleServer, metrics.StatusSuccess).Inc()
			return nil
		}
	}
}

func (s *ServerApp) handleSignalingMessage(msg transport.SignalingMessage, peer transport.PeerManager, connStart time.Time, protocol *transport.ServerSignalingProtocol) (done bool) {
	return s.processSignalingMessage(msg, peer, connStart, "", protocol)
}

func (s *ServerApp) processSignalingMessage(msg transport.SignalingMessage, peer transport.PeerManager, connStart time.Time, clientID string, protocol *transport.ServerSignalingProtocol) bool {
	if protocol != nil {
		msg.Payload = protocol.DecryptPayload(msg.Payload)
	}
	logFields := s.buildLogFields(clientID)

	switch msg.Type {
	case "answer":
		return s.handleAnswerMessage(msg, peer, logFields, clientID)
	case "candidate":
		s.handleCandidateMessage(msg, peer, logFields)
		return false
	case "candidate_done":
		s.logger.Debug("Remote ICE gathering complete", logFields...)
		return false
	case "control":
		return s.handleControlMessage(msg, logFields, connStart)
	}
	return false
}

func (s *ServerApp) buildLogFields(clientID string) []interface{} {
	logFields := make([]interface{}, 0, 2)
	if clientID != "" {
		logFields = append(logFields, "clientID", clientID)
	}
	return logFields
}

func (s *ServerApp) handleAnswerMessage(msg transport.SignalingMessage, peer transport.PeerManager, logFields []interface{}, clientID string) bool {
	var sdpMsg struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(msg.Payload, &sdpMsg); err != nil {
		s.logger.Warn("Failed to parse answer SDP", append(logFields, "error", err)...)
		return false
	}
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdpMsg.SDP,
	}
	if err := peer.SetRemoteDescription(answer); err != nil {
		s.logger.Error("Failed to set remote description", append(logFields, "error", err)...)
		return clientID != ""
	}
	return false
}

func (s *ServerApp) handleCandidateMessage(msg transport.SignalingMessage, peer transport.PeerManager, logFields []interface{}) {
	handleCandidateMsg(s.logger, msg.Payload, peer, logFields...)
}

func (s *ServerApp) handleControlMessage(msg transport.SignalingMessage, logFields []interface{}, connStart time.Time) bool {
	if handleControlMsg(s.logger, msg.Payload, logFields...) {
		s.logger.Info("Client requested stop", logFields...)
		metrics.ConnectionDuration.Observe(time.Since(connStart).Seconds())
		metrics.ConnectionsTotal.WithLabelValues(metrics.RoleServer, metrics.StatusSuccess).Inc()
		return true
	}
	return false
}

// handleDCControl processes a raw control message received via WebRTC DataChannel.
func (s *ServerApp) handleDCControl(raw []byte, connStart time.Time, nickname string) bool {
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

	// Parse action from payload.
	var ctrl struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(msg.Payload, &ctrl); err != nil {
		return false
	}

	switch ctrl.Action {
	case transport.ActionStop:
		s.logger.Info("Client requested stop (via DataChannel)")
		metrics.ConnectionDuration.Observe(time.Since(connStart).Seconds())
		metrics.ConnectionsTotal.WithLabelValues(metrics.RoleServer, metrics.StatusSuccess).Inc()
		return true
	case transport.ActionPeerMute:
		s.clientMuted.Store(true)
		s.logger.Info(nickname + " muted server audio")
		return false
	case transport.ActionPeerUnmute:
		s.clientMuted.Store(false)
		s.logger.Info(nickname + " unmuted server audio")
		return false
	case transport.ActionPause, transport.ActionPauseAll:
		s.clientPaused.Store(true)
		s.logger.Info(nickname + " paused capture")
		return false
	case transport.ActionResume, transport.ActionResumeAll:
		s.clientPaused.Store(false)
		s.logger.Info(nickname + " resumed capture")
		return false
	}

	return false
}

func (s *ServerApp) runCapturePipeline(ctx context.Context, sendCh chan<- []byte) error {
	encCfg := EncoderConfig{
		Bitrate:     s.cfg.OpusBitrate,
		Complexity:  s.cfg.OpusComplexity,
		DTX:         s.cfg.OpusDTX,
		FEC:         s.cfg.OpusFEC,
		Application: s.cfg.OpusApplication,
	}

	// In duplex mode, separate analyzers are used for capture vs playback.
	// In non-duplex mode, captureSpectrum may be nil — fall back to s.spectrum.
	captureSpectrum := s.captureSpectrum
	captureLevel := s.captureLevelMeter
	if captureSpectrum == nil {
		captureSpectrum = s.spectrum
	}
	if captureLevel == nil {
		captureLevel = s.levelMeter
	}

	// Multi-device capture: use MultiCapturePipeline
	captureDevices := s.cfg.CaptureDevices()
	if len(captureDevices) > 1 {
		pipeline := NewServerMultiCapturePipeline(MultiCapturePipelineConfig{
			Devices:       captureDevices,
			SampleRate:    s.cfg.SampleRate,
			Channels:      s.cfg.Channels,
			EncoderConfig: encCfg,
			BufferFrames:  s.cfg.EffectiveAudioBufferFrames(),
			Spectrum:      captureSpectrum,
			LevelMeter:    captureLevel,
		}, s.logger)
		return pipeline.Run(ctx, sendCh)
	}

	// Single-device capture (legacy path)
	if s.cfg.DeviceID == nil && len(captureDevices) == 0 {
		return ewerrors.NewError(ewerrors.ErrConfigInvalid, "no device ID specified").
			WithSuggestion("Select an audio device with --device flag or interactive selection")
	}

	deviceID := uint32(0)
	if len(captureDevices) == 1 {
		deviceID = captureDevices[0].ID
	} else if s.cfg.DeviceID != nil {
		deviceID = *s.cfg.DeviceID
	}

	pipeline := NewServerCapturePipeline(CapturePipelineConfig{
		SampleRate:           s.cfg.SampleRate,
		Channels:             s.cfg.Channels,
		DeviceID:             deviceID,
		AudioBufferFrames:    s.cfg.AudioBufferFrames,
		IsLoopback:           s.cfg.Loopback,
		LoopbackOutputDevice: s.cfg.LoopbackOutputDevice,
		LoopbackBlackHole:    s.cfg.LoopbackBlackHole,
		EncoderConfig:        encCfg,
		AEC:                  s.aec,
		Spectrum:             captureSpectrum,
		LevelMeter:           captureLevel,
	}, s.logger)

	return pipeline.Run(ctx, sendCh)
}
