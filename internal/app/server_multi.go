package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func (s *ServerApp) runMulti(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	listenAddr := fmt.Sprintf(":%d", s.cfg.Port)
	s.logger.Info("Starting multi-client server", "port", s.cfg.Port, "maxClients", s.cfg.MaxClients)

	listener, err := s.createListener(listenAddr)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrNetworkConnection, "start TCP listener")
	}
	defer func() { _ = listener.Close() }() //nolint:errcheck

	go s.closeListenerOnContextDone(ctx, listener)
	go s.processCommands(ctx)
	go s.reportMultiStats(ctx)

	// Initialize conference mix engine if in conference mode.
	if s.cfg.Conference {
		frameSize := int(s.cfg.SampleRate) * 20 / 1000 // 20ms frames
		s.conference = NewConferenceHandler(frameSize, s.cfg.SampleRate, s.cfg.ServerMuted, s.logger)
		if s.participantCmdCh != nil {
			s.conference.WithParticipantCommands(s.participantCmdCh)
		}
		go s.conference.ProcessCommands(ctx)
		go s.reportConferenceStats(ctx)

		// Add server as participant and start server capture → mixer (unless server-muted).
		if !s.cfg.ServerMuted {
			s.conference.AddParticipant("server")
			go s.runServerConferenceCapture(ctx)
		}

		// Start recording if --record flag was provided.
		if s.cfg.RecordMode != "" {
			var recMode audio.RecordingMode
			switch s.cfg.RecordMode {
			case "tracks":
				recMode = audio.RecordTracks
			case "both":
				recMode = audio.RecordBoth
			default:
				recMode = audio.RecordMix
			}
			if err := s.conference.StartRecording(recMode, s.cfg.SampleRate); err != nil {
				s.logger.Error("Failed to start recording", "error", err)
			} else {
				s.logger.Info("Recording started", "mode", s.cfg.RecordMode)
			}
		}

		// Handle recording commands from TUI and periodic header flush.
		go s.processRecordingCommands(ctx)
		go s.flushRecordingHeaders(ctx)
		go s.recordMixLoop(ctx)
	}

	// Process server pause toggle.
	if s.serverPauseCh != nil {
		if s.cfg.Conference && !s.cfg.ServerMuted {
			go s.processServerPause(ctx)
		} else if !s.cfg.Conference {
			go s.processServerPauseMulti(ctx)
		}
	}

	return s.acceptClients(ctx, listener)
}

// createListener creates a TCP listener (plain or TLS based on configuration).
func (s *ServerApp) createListener(listenAddr string) (net.Listener, error) {
	if s.tlsConfig != nil {
		return tls.Listen("tcp", listenAddr, s.tlsConfig)
	}
	return net.Listen("tcp", listenAddr) //nolint:noctx
}

// closeListenerOnContextDone closes the listener when context is canceled.
func (s *ServerApp) closeListenerOnContextDone(ctx context.Context, listener net.Listener) {
	<-ctx.Done()
	_ = listener.Close() //nolint:errcheck
}

// acceptClients continuously accepts new client connections.
func (s *ServerApp) acceptClients(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.Error("Accept error", "error", err)
			continue
		}

		clientID := s.generateClientID()
		s.logger.Info("New client connection", "clientID", clientID, "addr", conn.RemoteAddr())
		go s.handleMultiClient(ctx, conn, clientID)
	}
}

// generateClientID creates a unique client identifier.
func (s *ServerApp) generateClientID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextClient++
	return fmt.Sprintf("client-%d", s.nextClient)
}

func (s *ServerApp) handleMultiClient(ctx context.Context, conn net.Conn, clientID string) {
	defer func() { _ = conn.Close() }() //nolint:errcheck
	connStart := time.Now()
	remoteAddr := conn.RemoteAddr().String()

	if !s.checkMultiClientAccess(remoteAddr, clientID) {
		return
	}

	signaler := transport.NewTCPSignalerFromConn(conn)
	sigCtx, sigCancel := context.WithCancel(ctx)
	defer sigCancel()
	signaler.StartWithConn(sigCtx)

	protocol := transport.NewServerSignalingProtocol(s.auth)

	if err := s.authenticateWithProtocol(sigCtx, protocol, signaler, remoteAddr, clientID); err != nil {
		return
	}

	// Receive client metadata (nickname, HWID, session ID) per spec §2.2.
	meta, err := protocol.ReceiveClientMeta(sigCtx, signaler)
	if err != nil {
		s.logger.Warn("Failed to receive client meta", "clientID", clientID, "error", err)
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
		return
	}

	// 3. Check HWID ban.
	if s.cfg.HWIDRequired && meta.HWID != "" && s.banMgr != nil && s.banMgr.IsHWIDBanned(meta.HWID) {
		payload, _ := json.Marshal(auth.AuthResultPayload{Success: false, Message: "device banned", Code: 403}) //nolint:errcheck
		_ = signaler.Send(transport.SignalingMessage{Type: "auth_result", Payload: payload})                    //nolint:errcheck
		return
	}

	// Assign nickname and session ID.
	isCustomNick := meta.Nickname != ""
	nickname := s.assignNickname(meta.Nickname)
	if oldNickname != "" {
		nickname = oldNickname
	}
	sessionID := auth.NewSessionID()

	mc, ok := s.registerMultiClient(conn, clientID)
	if !ok {
		s.logger.Warn("Max clients reached, rejecting connection", "clientID", clientID, "addr", remoteAddr)
		payload, _ := json.Marshal(auth.AuthResultPayload{ //nolint:errcheck
			Success: false,
			Message: fmt.Sprintf("server is at capacity (max %d clients)", s.cfg.MaxClients),
			Code:    503,
		})
		_ = signaler.Send(transport.SignalingMessage{Type: "auth_result", Payload: payload}) //nolint:errcheck
		return
	}
	mc.nickname = nickname
	mc.sessionID = sessionID
	mc.hwid = meta.HWID
	mc.isCustomNick = isCustomNick

	// Track whether this client disconnected gracefully.
	gracefulDisconnect := false
	defer func() {
		s.unregisterMultiClient(mc, clientID, gracefulDisconnect)
	}()

	// Register as conference participant if in conference mode.
	if s.conference != nil {
		s.conference.AddParticipant(clientID)
		defer func() {
			s.conference.RemoveParticipant(clientID)
			s.broadcastParticipantsUpdate(clientID)
		}()
	}

	if err := s.sendAuthResultWithSession(protocol, signaler, sessionID, nickname); err != nil {
		s.logger.Warn("Failed to send auth result", "clientID", clientID, "error", err)
		return
	}

	// Send reconnect system message if applicable.
	if oldNickname != "" && s.chatHub != nil {
		s.chatHub.BroadcastSystem(fmt.Sprintf("%s reconnected", oldNickname))
	}

	peer, direction, ok := s.createMultiClientPeer(mc, clientID)
	if !ok {
		return
	}

	audioDone, ok := s.setupMultiClientAudio(sigCtx, peer, direction, clientID)
	if !ok {
		return
	}

	s.setupConnectionStateCallback(peer, sigCancel)

	if err := peer.CreateControlDataChannel(); err != nil {
		s.logger.Warn("Failed to create control data channel", "clientID", clientID, "error", err)
		return
	}

	if err := peer.CreateChatDataChannel(); err != nil {
		s.logger.Warn("Failed to create chat data channel", "clientID", clientID, "error", err)
	}

	if err := protocol.HandleHandshake(sigCtx, signaler, peer); err != nil {
		s.logger.Warn("WebRTC handshake failed", "clientID", clientID, "error", err)
		return
	}

	s.logger.Info("Client connected", "clientID", clientID, "nickname", nickname, "addr", remoteAddr)
	// Send stats immediately so the TUI transitions to the streaming screen.
	if s.multiStatsCh != nil {
		select {
		case s.multiStatsCh <- s.collectMultiClientStats():
		default:
		}
	}
	// Broadcast participants_update to all other clients on join.
	if s.conference != nil {
		s.broadcastParticipantsUpdate(clientID)
	}
	gracefulDisconnect = s.runMultiClientLoop(sigCtx, signaler, peer, audioDone, clientID, nickname, connStart, protocol)
}

func (s *ServerApp) checkMultiClientAccess(remoteAddr, clientID string) bool {
	return s.validateClientAccess(remoteAddr, clientID) == nil
}

func (s *ServerApp) registerMultiClient(conn net.Conn, clientID string) (*multiClient, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.clients) >= s.cfg.MaxClients {
		s.logger.Warn("Max clients reached, rejecting connection", "clientID", clientID)
		return nil, false
	}

	mc := &multiClient{
		id:       clientID,
		conn:     conn,
		joinedAt: time.Now(),
	}
	s.clients[clientID] = mc
	s.notifyClientCount()
	return mc, true
}

func (s *ServerApp) unregisterMultiClient(mc *multiClient, clientID string, graceful bool) {
	s.mu.Lock()
	delete(s.clients, clientID)
	if graceful {
		// Graceful disconnect: remove session so it cannot be reused.
		delete(s.sessions, mc.sessionID)
	} else {
		// Abnormal disconnect: preserve session for future reconnect.
		s.sessions[mc.sessionID] = &sessionEntry{
			clientID:     clientID,
			nickname:     mc.nickname,
			isCustomNick: mc.isCustomNick,
		}
	}
	s.notifyClientCount()
	s.mu.Unlock()
	if mc.peer != nil {
		_ = mc.peer.Close() //nolint:errcheck
	}
	s.logger.Info("Client disconnected", "clientID", clientID, "graceful", graceful)
}

func (s *ServerApp) createMultiClientPeer(mc *multiClient, clientID string) (transport.PeerManager, transport.MediaDirection, bool) {
	peer, direction, err := s.createWebRTCPeer()
	if err != nil {
		s.logger.Warn("Failed to create peer connection", "clientID", clientID, "error", err)
		return nil, direction, false
	}
	mc.peer = peer
	return peer, direction, true
}

func (s *ServerApp) setupMultiClientAudio(ctx context.Context, peer transport.PeerManager, direction transport.MediaDirection, clientID string) (<-chan error, bool) {
	if s.conference != nil {
		s.mu.RLock()
		mc := s.clients[clientID]
		s.mu.RUnlock()
		var muteIncomingFlag *atomic.Bool
		if mc != nil {
			muteIncomingFlag = &mc.mutedIncoming
		}
		audioDone, err := s.setupConferenceAudioPipeline(ctx, peer, clientID, muteIncomingFlag)
		if err != nil {
			s.logger.Warn("Failed to setup conference audio", "clientID", clientID, "error", err)
			return nil, false
		}
		return audioDone, true
	}

	// Get per-client mute flags.
	s.mu.RLock()
	mc := s.clients[clientID]
	s.mu.RUnlock()
	var muteFlag, muteOutgoingFlag, muteIncomingFlag *atomic.Bool
	if mc != nil {
		muteFlag = &mc.muted
		muteOutgoingFlag = &mc.mutedOutgoing
		muteIncomingFlag = &mc.mutedIncoming
	}

	audioDone, err := s.setupAudioPipelineMulti(ctx, peer, direction, muteFlag, muteOutgoingFlag, muteIncomingFlag)
	if err != nil {
		s.logger.Warn("Failed to setup audio", "clientID", clientID, "error", err)
		return nil, false
	}
	return audioDone, true
}

// setupAudioPipelineMulti is like setupAudioPipeline but uses per-client mute flags
// instead of the shared s.clientMuted. This ensures muting one client doesn't affect others.
// muteFlag is client-initiated mute, muteOutgoingFlag is server-initiated mute.
func (s *ServerApp) setupAudioPipelineMulti(ctx context.Context, peer transport.PeerManager, direction transport.MediaDirection, muteFlag, muteOutgoingFlag, muteIncomingFlag *atomic.Bool) (<-chan error, error) {
	audioDone := make(chan error, 2)

	// Use no-op flags if nil (shouldn't happen, but safe).
	if muteFlag == nil {
		muteFlag = &atomic.Bool{}
	}
	if muteOutgoingFlag == nil {
		muteOutgoingFlag = &atomic.Bool{}
	}
	if muteIncomingFlag == nil {
		muteIncomingFlag = &atomic.Bool{}
	}

	switch direction {
	case transport.DirectionDuplex:
		sendCh, err := peer.AddAudioTrack(s.cfg.SampleRate, s.cfg.Channels)
		if err != nil {
			return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "add audio track")
		}
		filteredSendCh := newCombinedMuteFilterCh(ctx, sendCh, muteFlag, muteOutgoingFlag)
		filteredSendCh = s.newServerPauseFilterCh(ctx, filteredSendCh)
		go func() {
			audioDone <- s.runCapturePipeline(ctx, filteredSendCh)
		}()
		s.setupReverseAudioMuted(ctx, peer, audioDone, "", muteIncomingFlag)
	case transport.DirectionSend:
		sendCh, err := peer.AddAudioTrack(s.cfg.SampleRate, s.cfg.Channels)
		if err != nil {
			return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "add audio track")
		}
		filteredSendCh := newCombinedMuteFilterCh(ctx, sendCh, muteFlag, muteOutgoingFlag)
		filteredSendCh = s.newServerPauseFilterCh(ctx, filteredSendCh)
		go func() {
			audioDone <- s.runCapturePipeline(ctx, filteredSendCh)
		}()
	default:
		s.setupReverseAudioMuted(ctx, peer, audioDone, "", muteIncomingFlag)
	}
	return audioDone, nil
}

// setupConferenceAudioPipeline creates bidirectional audio for conference mode:
// - Receive: decode client audio → submit to conference mixer
// - Send: get personal mix for client → encode → send back
func (s *ServerApp) setupConferenceAudioPipeline(ctx context.Context, peer transport.PeerManager, clientID string, muteIncomingFlag *atomic.Bool) (<-chan error, error) {
	audioDone := make(chan error, 2)

	// Send track: personal mix → encode → client
	sendCh, err := peer.AddAudioTrack(s.cfg.SampleRate, s.cfg.Channels)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "add audio track for conference")
	}

	// Launch personal mix sender goroutine.
	go func() {
		audioDone <- s.runConferenceMixSender(ctx, sendCh, clientID)
	}()

	// Receive track: decode → conference mixer submit (+ AEC reference).
	// No local playback — prevents echo/feedback loop on the server.
	s.setupConferenceReceiver(ctx, peer, audioDone, clientID, muteIncomingFlag)

	return audioDone, nil
}

// runConferenceMixSender periodically gets the personal mix for a client,
// encodes it, and sends it via WebRTC.
func (s *ServerApp) runConferenceMixSender(ctx context.Context, sendCh chan<- []byte, clientID string) error {
	enc, err := audio.NewOpusEncoder(int(s.cfg.SampleRate), int(s.cfg.Channels), s.cfg.OpusApplication)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "create conference opus encoder")
	}
	if err := enc.SetBitrate(s.cfg.OpusBitrate); err != nil {
		s.logger.Warn("Failed to set opus bitrate", "error", err)
	}

	frameSize := int(s.cfg.SampleRate) / 50 * int(s.cfg.Channels) // 20ms
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			mix := s.conference.GetPersonalMix(clientID)
			if mix == nil {
				continue
			}
			// Ensure correct frame size.
			if len(mix) != frameSize {
				if len(mix) > frameSize {
					mix = mix[:frameSize]
				} else {
					padded := make([]float32, frameSize)
					copy(padded, mix)
					s.conference.ReturnMixBuffer(mix)
					mix = padded
				}
			}
			encoded, encErr := enc.Encode(mix)
			s.conference.ReturnMixBuffer(mix)
			if encErr != nil {
				s.logger.Warn("Conference opus encode error", "error", encErr, "clientID", clientID)
				continue
			}
			metrics.AudioBytesSent.Add(float64(len(encoded)))
			select {
			case sendCh <- encoded:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// runMultiClientLoop runs the main event loop for a multi-client connection.
// Returns true if the disconnect was graceful (client sent stop, or server shutdown).
func (s *ServerApp) runMultiClientLoop(ctx context.Context, signaler transport.Signaler, peer transport.PeerManager, audioDone <-chan error, clientID, nickname string, connStart time.Time, protocol *transport.ServerSignalingProtocol) bool {
	var statsWg sync.WaitGroup
	chatRegistered := false
	defer func() {
		if chatRegistered && s.chatHub != nil {
			s.chatHub.Unregister(clientID)
		}
		statsWg.Wait()
		s.sendDisconnected()
	}()

	dcReady := peer.DCReady()
	receiveCh := signaler.Receive()
	var controlCh <-chan []byte
	var chatCh <-chan []byte

	// Start stats reporting immediately (same rationale as single-client flow).
	if s.statsCh != nil {
		statsWg.Add(1)
		go func() {
			defer statsWg.Done()
			s.reportStats(ctx, peer)
		}()
	}

	// Poll for control channel availability.
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

	// stopCh may be nil if not using TUI; use a never-closed channel as fallback.
	stopCh := s.stopCh
	if stopCh == nil {
		stopCh = make(chan struct{})
	}

	for {
		select {
		case ch := <-controlReady:
			controlCh = ch
			controlReady = nil
		case ch := <-chatReady:
			chatCh = ch
			chatReady = nil
			if s.chatHub != nil && !chatRegistered {
				s.chatHub.Register(clientID, nickname, peer.SendChat)
				chatRegistered = true
			}
		case <-dcReady:
			dcReady = nil
			receiveCh = nil
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
			_ = signaler.Close() //nolint:errcheck
		case raw, ok := <-controlCh:
			if !ok || len(raw) == 0 {
				// Channel closed (peer disconnected) — abnormal.
				return false
			}
			if s.handleDCControlMulti(raw, connStart, clientID) {
				_ = peer.Close() //nolint:errcheck
				return true      // Client sent stop — graceful.
			}
		case raw, ok := <-chatCh:
			if !ok || len(raw) == 0 {
				// Channel closed (peer disconnected) — abnormal.
				return false
			}
			if s.chatHub != nil {
				s.chatHub.HandleIncoming(clientID, raw)
			}
		case <-stopCh:
			// Graceful stop requested by TUI: send ActionStop to this client.
			s.logger.Info("Graceful stop requested, notifying client", "clientID", clientID)
			if err := peer.SendControl(transport.ActionStop, nil); err != nil {
				s.logger.Debug("Failed to send stop via DataChannel", "clientID", clientID, "error", err)
			}
			time.Sleep(150 * time.Millisecond)
			_ = peer.Close() //nolint:errcheck
			return true      // Server graceful shutdown.
		case <-ctx.Done():
			// Context may be canceled because stopCh fired (race between stopCh and ctx.Done).
			// Check if this is a graceful stop and notify the client.
			select {
			case <-stopCh:
				s.logger.Info("Graceful stop requested (via ctx), notifying client", "clientID", clientID)
				if err := peer.SendControl(transport.ActionStop, nil); err != nil {
					s.logger.Debug("Failed to send stop via DataChannel", "clientID", clientID, "error", err)
				}
				time.Sleep(150 * time.Millisecond)
				_ = peer.Close() //nolint:errcheck
				return true      // Server graceful shutdown.
			default:
				// Not a graceful stop — context canceled for other reasons.
			}
			return false
		case msg := <-receiveCh:
			if s.handleMultiClientMessage(msg, peer, clientID, connStart, protocol) {
				return true // Signaling control stop — graceful.
			}
		case err := <-audioDone:
			if err != nil && ctx.Err() == nil {
				s.logger.Warn("Audio pipeline error", "clientID", clientID, "error", err)
			}
			metrics.ConnectionDuration.Observe(time.Since(connStart).Seconds())
			metrics.ConnectionsTotal.WithLabelValues(metrics.RoleServer, metrics.StatusSuccess).Inc()
			return false
		}
	}
}

func (s *ServerApp) handleMultiClientMessage(msg transport.SignalingMessage, peer transport.PeerManager, clientID string, connStart time.Time, protocol *transport.ServerSignalingProtocol) bool {
	return s.processSignalingMessage(msg, peer, connStart, clientID, protocol)
}

// runServerConferenceCapture captures audio from the server's microphone
// and submits it to the conference mixer as the "server" participant.
func (s *ServerApp) runServerConferenceCapture(ctx context.Context) {
	capturer, err := audio.NewCapturer(s.cfg.SampleRate, s.cfg.Channels)
	if err != nil {
		s.logger.Error("Failed to create server conference capturer", "error", err)
		return
	}
	defer func() { _ = capturer.Close() }() //nolint:errcheck

	deviceID := uint32(0)
	if s.cfg.DeviceID != nil {
		deviceID = *s.cfg.DeviceID
	} else if devs := s.cfg.CaptureDevices(); len(devs) > 0 {
		deviceID = devs[0].ID
	}

	bufSize := s.cfg.EffectiveAudioBufferFrames()
	if bufSize <= 0 {
		bufSize = 5
	}
	pcmCh := make(chan []float32, bufSize)

	go func() {
		if err := capturer.Start(ctx, deviceID, pcmCh); err != nil && ctx.Err() == nil {
			s.logger.Error("Server conference capture error", "error", err)
		}
	}()

	frameSize := int(s.cfg.SampleRate) / 50 * int(s.cfg.Channels)
	acc := audio.NewFrameAccumulator(frameSize)

	for {
		select {
		case <-ctx.Done():
			return
		case samples := <-pcmCh:
			// Apply AEC to server capture if enabled.
			if s.aec != nil && s.aec.IsEnabled() {
				processed, aecErr := s.aec.Process(ctx, samples)
				if aecErr == nil {
					samples = processed
				}
			}
			if s.spectrum != nil {
				s.spectrum.Feed(samples)
			}
			if s.levelMeter != nil {
				s.levelMeter.Feed(samples)
			}
			frames := acc.Write(samples)
			for _, frame := range frames {
				s.conference.SubmitAudio("server", frame)
			}
		}
	}
}

// assignNickname returns a validated nickname for a client. If the requested nickname
// is empty, a default "Client-N" is assigned. Returns the final nickname.
func (s *ServerApp) assignNickname(requested string) string {
	if requested == "" {
		s.mu.Lock()
		s.nextNickname++
		n := s.nextNickname
		s.mu.Unlock()
		return fmt.Sprintf("Client-%d", n)
	}
	return requested
}

// validateNickname checks if a nickname is valid for use. Returns an error message if invalid, or empty string if ok.
func (s *ServerApp) validateNickname(nickname string) string {
	if nickname == "" {
		return ""
	}

	// Check length.
	if len([]rune(nickname)) > 20 {
		return "invalid nickname"
	}

	// Check allowed characters: alphanumeric, -, _, spaces.
	for _, r := range nickname {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != ' ' {
			return "invalid nickname"
		}
	}

	// Reserved name.
	if strings.EqualFold(nickname, "Server") {
		return "nickname reserved"
	}

	// Check nickname uniqueness among connected clients.
	s.mu.RLock()
	for _, mc := range s.clients {
		if strings.EqualFold(mc.nickname, nickname) {
			s.mu.RUnlock()
			return "nickname already in use. If this is your session, wait ~5 seconds and try again"
		}
	}
	s.mu.RUnlock()

	// Check nickname ban.
	if s.banMgr != nil && s.banMgr.IsNicknameBanned(nickname) {
		return "nickname banned"
	}

	return ""
}

// handleReconnectBySession checks the sessions map for a previous session with the given UUID.
// If found, returns the old nickname (only if it was server-assigned) and removes the session entry.
// Also closes any still-connected client with the same session ID.
func (s *ServerApp) handleReconnectBySession(sessionID string) (oldNickname string, found bool) {
	if sessionID == "" {
		return "", false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check sessions map first (covers abnormal disconnects where session was preserved).
	if entry, ok := s.sessions[sessionID]; ok {
		delete(s.sessions, sessionID)
		if !entry.isCustomNick {
			return entry.nickname, true
		}
		// Custom nickname — don't restore it, but acknowledge the session was found.
		return "", true
	}

	// Fallback: check if a still-connected client has this session (e.g. stale connection).
	for id, mc := range s.clients {
		if mc.sessionID != sessionID {
			continue
		}
		oldNickname = mc.nickname
		// Close old peer and connection.
		if mc.peer != nil {
			_ = mc.peer.Close() //nolint:errcheck
		}
		if mc.conn != nil {
			_ = mc.conn.Close() //nolint:errcheck
		}
		delete(s.clients, id)
		s.notifyClientCount()
		return oldNickname, true
	}
	return "", false
}

// processServerPause listens for server pause toggles from TUI and broadcasts to all clients (conference).
func (s *ServerApp) processServerPause(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case paused, ok := <-s.serverPauseCh:
			if !ok {
				return
			}
			if s.conference != nil {
				s.conference.SetParticipantPaused("server", paused)
				s.broadcastParticipantPause("server", paused)
				s.logger.Info("Server pause state changed", "paused", paused)
			}
		}
	}
}

// processServerPauseMulti listens for server pause toggles in non-conference multi-client mode.
// Sets serverPaused atomic and notifies all connected clients via datachannel.
func (s *ServerApp) processServerPauseMulti(ctx context.Context) {
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
			// Notify all connected clients.
			s.mu.RLock()
			for _, mc := range s.clients {
				_ = mc.peer.SendControl(transport.ActionParticipantPause, map[string]interface{}{
					"participantID": "server",
					"paused":        paused,
				})
			}
			s.mu.RUnlock()
		}
	}
}

// handleDCControlMulti processes a control message in multi-client mode.
// It delegates to handleDCControl and additionally handles conference pause broadcast.
func (s *ServerApp) handleDCControlMulti(raw []byte, connStart time.Time, clientID string) bool {
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

	var ctrl struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(msg.Payload, &ctrl); err != nil {
		return false
	}

	// Resolve client info for per-client state.
	s.mu.RLock()
	mc := s.clients[clientID]
	nickname := clientID
	if mc != nil {
		nickname = mc.nickname
	}
	s.mu.RUnlock()

	// Conference pause broadcast.
	if s.conference != nil {
		switch ctrl.Action {
		case transport.ActionPause, transport.ActionPauseAll:
			s.conference.SetParticipantPaused(clientID, true)
			s.broadcastParticipantPause(clientID, true)
			s.logger.Info(nickname + " paused capture (conference)")
		case transport.ActionResume, transport.ActionResumeAll:
			s.conference.SetParticipantPaused(clientID, false)
			s.broadcastParticipantPause(clientID, false)
			s.logger.Info(nickname + " resumed capture (conference)")
		}
	}

	// Handle per-client mute/pause in multi-client mode (don't delegate to shared state).
	switch ctrl.Action {
	case transport.ActionPeerMute:
		if mc != nil {
			mc.muted.Store(true)
		}
		s.logger.Info(nickname + " muted server audio")
		return false
	case transport.ActionPeerUnmute:
		if mc != nil {
			mc.muted.Store(false)
		}
		s.logger.Info(nickname + " unmuted server audio")
		return false
	case transport.ActionPause, transport.ActionPauseAll:
		if mc != nil {
			mc.paused.Store(true)
		}
		if s.conference == nil {
			s.logger.Info(nickname + " paused capture")
		}
		return false
	case transport.ActionResume, transport.ActionResumeAll:
		if mc != nil {
			mc.paused.Store(false)
		}
		if s.conference == nil {
			s.logger.Info(nickname + " resumed capture")
		}
		return false
	case transport.ActionStop:
		return s.handleDCControl(raw, connStart, nickname)
	}

	return false
}

// broadcastParticipantPause sends a participant_pause control message to all peers
// except the one identified by excludeID.
func (s *ServerApp) broadcastParticipantPause(participantID string, paused bool) {
	payload := map[string]interface{}{
		"participantID": participantID,
		"paused":        paused,
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, mc := range s.clients {
		if id == participantID {
			continue // don't send back to sender
		}
		if mc.peer != nil {
			if err := mc.peer.SendControl(transport.ActionParticipantPause, payload); err != nil {
				s.logger.Debug("Failed to broadcast participant_pause", "target", id, "error", err)
			}
		}
	}
}

// broadcastParticipantsUpdate sends a participants_update control message to all peers
// except excludeID. Contains the full list of participant IDs.
func (s *ServerApp) broadcastParticipantsUpdate(excludeID string) {
	if s.conference == nil {
		return
	}
	ids := s.conference.ParticipantIDs()
	// Build participant info with nicknames and pause state.
	pausedMap := s.conference.GetPausedParticipants()
	type participantInfo struct {
		ID       string `json:"id"`
		Nickname string `json:"nickname"`
		Paused   bool   `json:"paused"`
	}
	participants := make([]participantInfo, 0, len(ids))
	s.mu.RLock()
	for _, pid := range ids {
		nick := pid
		if pid == "server" {
			nick = "Server"
		} else if mc, ok := s.clients[pid]; ok {
			nick = mc.nickname
		}
		participants = append(participants, participantInfo{
			ID:       pid,
			Nickname: nick,
			Paused:   pausedMap[pid],
		})
	}
	clientsCopy := make(map[string]*multiClient, len(s.clients))
	for k, v := range s.clients {
		clientsCopy[k] = v
	}
	s.mu.RUnlock()

	payload := map[string]interface{}{
		"participants": participants,
	}
	for id, mc := range clientsCopy {
		if id == excludeID {
			continue
		}
		if mc.peer != nil {
			if err := mc.peer.SendControl(transport.ActionParticipantsUpdate, payload); err != nil {
				s.logger.Debug("Failed to broadcast participants_update", "target", id, "error", err)
			}
		}
	}
}
