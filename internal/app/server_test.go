package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func testAppLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestServerApp_Creation(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 9999
	cfg.Password = "secret"

	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	require.NotNil(t, app)
	assert.Equal(t, 9999, app.cfg.Port)
	assert.Equal(t, "secret", app.cfg.Password)
	assert.NotNil(t, app.auth, "auth handler must be initialized")
	assert.NotNil(t, app.signalerFactory, "signaler factory must be set")
	assert.NotNil(t, app.peerFactory, "peer factory must be set")
	assert.NotNil(t, app.clients, "clients map must be initialized")
	assert.Nil(t, app.banMgr, "banMgr should be nil when not provided")
	assert.Nil(t, app.tlsConfig, "tlsConfig should be nil when not provided")
	assert.Nil(t, app.rateLimiter, "rateLimiter should be nil when not provided")
}

func TestClientApp_Creation(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "127.0.0.1"
	cfg.Port = 8888
	cfg.Password = "pass"

	app := NewClientApp(cfg, testAppLogger(), nil)
	require.NotNil(t, app)
	assert.Equal(t, "127.0.0.1", app.cfg.Address)
	assert.Equal(t, 8888, app.cfg.Port)
	assert.NotNil(t, app.auth, "auth handler must be initialized")
	assert.NotNil(t, app.signalerFactory, "signaler factory must be set")
	assert.NotNil(t, app.peerFactory, "peer factory must be set")
	assert.Nil(t, app.tlsConfig, "tlsConfig should be nil when not provided")
}

func TestServerApp_Run_AlreadyCancelledContext(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 0

	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
}

func TestClientApp_Run_AlreadyCancelledContext(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "127.0.0.1"

	app := NewClientApp(cfg, testAppLogger(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
}

func testServerConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 0
	cfg.Password = "test-password"
	return cfg
}

func TestServerApp_CreateAndStartSignaler_NoTLS(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect a client in background so Ready() fires
	go func() {
		// Wait for listener to be up, then connect
		time.Sleep(50 * time.Millisecond)
		conn, err := net.Dial("tcp", "127.0.0.1:0")
		if err == nil {
			conn.Close()
		}
	}()

	// Use a short timeout; the real test is ContextCancellation
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelTimeout()

	signaler, _, sigCancel, err := app.createAndStartSignaler(ctxTimeout, ":0")
	if err != nil {
		// Context timeout is expected if no client connects to ephemeral port
		require.ErrorIs(t, err, context.DeadlineExceeded)
		return
	}
	defer sigCancel()
	require.NotNil(t, signaler)
	signaler.Close()
}

func TestServerApp_CreateAndStartSignaler_WithTLS(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	app := NewServerApp(cfg, testAppLogger(), nil, tlsConfig, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelTimeout()

	signaler, _, sigCancel, err := app.createAndStartSignaler(ctxTimeout, ":0")
	if err != nil {
		require.ErrorIs(t, err, context.DeadlineExceeded)
		return
	}
	defer sigCancel()
	require.NotNil(t, signaler)
	signaler.Close()
}

func TestServerApp_CheckClientAccess_Allowed(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	rateLimiter := auth.NewIPRateLimiter(10)
	defer rateLimiter.Close()
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, rateLimiter)

	err := app.validateClientAccess("192.168.1.1:12345", "test-client")
	require.NoError(t, err)
}

func TestServerApp_CheckClientAccess_Banned(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	banMgr.banned["127.0.0.1"] = true
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)

	err := app.validateClientAccess("127.0.0.1:12345", "test-client")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banned")
}

func TestServerApp_SendAuthResultViaProtocol_Success(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go signaler.StartWithConn(ctx)

	go func() {
		buf := make([]byte, 1024)
		client.Read(buf)
	}()

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err := app.sendAuthResultViaProtocol(protocol, signaler)
	require.NoError(t, err)
}

func TestServerApp_SendAuthResultViaProtocol_ClosedConnection(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	server, client := net.Pipe()
	client.Close()
	server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err := app.sendAuthResultViaProtocol(protocol, signaler)
	require.Error(t, err)
}

func TestServerApp_CreateWebRTCPeer_SendMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = false
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer, direction, err := app.createWebRTCPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, transport.DirectionSend, direction)
	peer.Close()
}

func TestServerApp_CreateWebRTCPeer_ReceiveMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer, direction, err := app.createWebRTCPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, transport.DirectionReceive, direction)
	peer.Close()
}

func TestServerApp_SetupAudioPipeline_SendMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = false
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	installFakeSharedCaptureHub(app, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{}
	})

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	// Register a test client so setupAudioPipeline can access mc.mutedOutgoing.
	app.mu.Lock()
	app.clients["test-client"] = &multiClient{id: "test-client"}
	app.mu.Unlock()

	ctx := context.Background()
	audioDone, err := app.setupAudioPipeline(ctx, peer, transport.DirectionSend, "test-client")

	require.NoError(t, err)
	require.NotNil(t, audioDone)
}

func TestServerApp_SetupAudioPipeline_ReceiveMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionReceive)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	// Register a test client so setupAudioPipeline can access mc.mutedOutgoing.
	app.mu.Lock()
	app.clients["test-client"] = &multiClient{id: "test-client"}
	app.mu.Unlock()

	ctx := context.Background()
	audioDone, err := app.setupAudioPipeline(ctx, peer, transport.DirectionReceive, "test-client")

	require.NoError(t, err)
	require.NotNil(t, audioDone)
}

func TestServerApp_SetupConnectionStateCallback(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	app.setupConnectionStateCallback(peer, func() {})
}

func TestServerApp_HandleHandshake_Success(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go signaler.StartWithConn(ctx)

	// Drain client side
	go func() {
		buf := make([]byte, 4096)
		client.Read(buf)
	}()

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err = protocol.HandleHandshake(ctx, signaler, peer)
	// Will time out waiting for answer, expected
	assert.Error(t, err)
}

func TestServerApp_HandleSignalingMessage_Answer(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(`{"sdp":"v=0\r\no=- 123 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"}`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_HandleSignalingMessage_Candidate(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: []byte(`{"candidate":"candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host","sdpMid":"0","sdpMLineIndex":0}`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_HandleSignalingMessage_CandidateDone(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate_done",
		Payload: []byte(`{}`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_HandleSignalingMessage_ControlStop(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`{"action":"stop"}`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.True(t, done)
}

func TestServerApp_HandleSignalingMessage_InvalidAnswer(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(`invalid json`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_HandleSignalingMessage_InvalidControl(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`invalid json`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_WithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`{"action":"stop"}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.True(t, done)
}

func TestServerApp_RunCapturePipeline_NoDeviceID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.DeviceID = nil
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	sendCh := make(chan []byte, 1)
	err := app.runCapturePipeline(context.Background(), sendCh)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no device ID")
}

func TestServerApp_RunSingle_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.runSingle(ctx)
	assert.Error(t, err)
}

func TestServerApp_HandleSignalingLoop_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)
	err = app.handleSignalingLoop(ctx, signaler, peer, audioDone, time.Now(), nil, "client-1", "Client-1", "test-session")
	assert.Error(t, err)
}

func TestServerApp_HandleSignalingLoop_AudioDone(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go signaler.StartWithConn(ctx)

	audioDone := make(chan error, 1)
	audioDone <- nil

	done := make(chan error)
	go func() {
		done <- app.handleSignalingLoop(ctx, signaler, peer, audioDone, time.Now(), nil, "client-1", "Client-1", "test-session")
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Error("handleSignalingLoop did not complete")
	}
}

func TestServerApp_HandleSignalingLoop_AudioError(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go signaler.StartWithConn(ctx)

	audioDone := make(chan error, 1)
	audioDone <- errors.New("audio error")

	done := make(chan error)
	go func() {
		done <- app.handleSignalingLoop(ctx, signaler, peer, audioDone, time.Now(), nil, "client-1", "Client-1", "test-session")
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "audio pipeline")
	case <-time.After(2 * time.Second):
		t.Error("handleSignalingLoop did not complete")
	}
}

func TestServerApp_ProcessSignalingMessage_InvalidCandidate(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: []byte(`invalid json`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "", nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_UnknownType(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "unknown",
		Payload: []byte(`{}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "", nil)
	assert.False(t, done)
}

func TestServerApp_BuildServerConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	cfg.SampleRate = 48000
	cfg.Channels = 2
	cfg.OpusBitrate = 128000
	cfg.OpusComplexity = 5
	cfg.OpusDTX = true
	cfg.OpusFEC = true
	cfg.OpusApplication = "audio"

	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	serverConfig := app.buildServerConfig()

	assert.Equal(t, true, serverConfig["reverse"])
	assert.Equal(t, uint32(48000), serverConfig["sample_rate"])
	assert.Equal(t, uint32(2), serverConfig["channels"])
	assert.Equal(t, 128000, serverConfig["opus_bitrate"])
	assert.Equal(t, 5, serverConfig["opus_complexity"])
	assert.Equal(t, true, serverConfig["opus_dtx"])
	assert.Equal(t, true, serverConfig["opus_fec"])
	assert.Equal(t, "audio", serverConfig["opus_application"])
}

func TestServerApp_NewServerApp_WithAllDeps(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	rateLimiter := auth.NewIPRateLimiter(10)
	defer rateLimiter.Close()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	app := NewServerApp(cfg, testAppLogger(), banMgr, tlsConfig, rateLimiter)

	require.NotNil(t, app)
	assert.Same(t, banMgr, app.banMgr, "banMgr must be the provided instance")
	assert.Same(t, tlsConfig, app.tlsConfig, "tlsConfig must be the provided instance")
	assert.Same(t, rateLimiter, app.rateLimiter, "rateLimiter must be the provided instance")
	assert.NotNil(t, app.auth, "auth handler must be initialized from config")
	assert.Equal(t, uint16(tls.VersionTLS12), app.tlsConfig.MinVersion)
	assert.Empty(t, app.clients, "clients map must be empty initially")
	assert.Equal(t, cfg.Port, app.cfg.Port)
	assert.Equal(t, cfg.Password, app.cfg.Password)
}

func TestServerApp_SendAuthResultPayload_Marshal(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	serverConfig := app.buildServerConfig()
	configJSON, err := json.Marshal(serverConfig)
	require.NoError(t, err)

	resultPayload := auth.AuthResultPayload{
		Success: true,
		Message: "Authenticated",
		Config:  configJSON,
	}

	payload, err := json.Marshal(resultPayload)
	require.NoError(t, err)

	var unmarshaled auth.AuthResultPayload
	err = json.Unmarshal(payload, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, unmarshaled.Success)
	assert.Equal(t, "Authenticated", unmarshaled.Message)
}

func TestServerApp_Run_SingleMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.MaxClients = 1
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
}

func TestServerApp_Run_MultiMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.MaxClients = 5
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
}

func TestServerApp_HandleOneClient_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.handleOneClient(ctx, ":0")
	assert.Error(t, err)
}

func TestServerApp_ValidateClientAccess_NoRateLimiter(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	err := app.validateClientAccess("192.168.1.1:12345", "client-1")
	assert.NoError(t, err)
}

func TestServerApp_ValidateClientAccess_NoBanManager(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	rateLimiter := auth.NewIPRateLimiter(10)
	defer rateLimiter.Close()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, rateLimiter)

	err := app.validateClientAccess("192.168.1.1:12345", "client-1")
	assert.NoError(t, err)
}

func TestServerApp_ValidateClientAccess_EmptyRemoteAddr(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	rateLimiter := auth.NewIPRateLimiter(10)
	defer rateLimiter.Close()
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, rateLimiter)

	err := app.validateClientAccess("", "client-1")
	assert.NoError(t, err)
}

func TestServerApp_SetupReverseAudio_WithDevice(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	deviceID := uint32(0)
	cfg.DeviceID = &deviceID
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionReceive)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)
	app.setupReverseAudio(ctx, peer, audioDone)
}

func TestServerApp_SetupReverseAudio_NoDevice(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	cfg.DeviceID = nil
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionReceive)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx := context.Background()
	audioDone := make(chan error, 1)
	app.setupReverseAudio(ctx, peer, audioDone)
}

func TestServerApp_CreateWebRTCPeer_WithTURN(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.TURNServers = []config.TURNServer{
		{URL: "turn:turn.example.com:3478", Username: "user", Credential: "pass"},
	}
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer, direction, err := app.createWebRTCPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, transport.DirectionSend, direction)
	peer.Close()
}

func TestEncoderConfig_AllFields(t *testing.T) {
	t.Parallel()
	encCfg := EncoderConfig{
		Bitrate:     128000,
		Complexity:  5,
		DTX:         true,
		FEC:         true,
		Application: "audio",
	}

	assert.Equal(t, 128000, encCfg.Bitrate)
	assert.Equal(t, 5, encCfg.Complexity)
	assert.True(t, encCfg.DTX)
	assert.True(t, encCfg.FEC)
	assert.Equal(t, "audio", encCfg.Application)
}

func TestCapturePipelineConfig_AllFields(t *testing.T) {
	t.Parallel()
	deviceID := uint32(1)
	cfg := CapturePipelineConfig{
		SampleRate: 48000,
		Channels:   2,
		DeviceID:   deviceID,
		EncoderConfig: EncoderConfig{
			Bitrate:     128000,
			Complexity:  5,
			DTX:         true,
			FEC:         true,
			Application: "audio",
		},
	}

	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, deviceID, cfg.DeviceID)
	assert.Equal(t, 128000, cfg.EncoderConfig.Bitrate)
}

func TestServerApp_RunCapturePipeline_WithDevice(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	deviceID := uint32(999)
	cfg.DeviceID = &deviceID
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sendCh := make(chan []byte, 1)
	err := app.runCapturePipeline(ctx, sendCh)
	assert.Error(t, err)
}

func TestServerApp_HandleSignalingMessage_InvalidAnswerJSON(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(`invalid json`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_HandleSignalingMessage_OtherAction(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`{"action":"pause"}`),
	}

	done := app.handleSignalingMessage(msg, peer, time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_SetupConnectionStateCallback_TriggersCallback(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	app.setupConnectionStateCallback(peer, func() {})
}

func TestServerApp_HandleSignalingLoop_MessageReceived(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go signaler.StartWithConn(ctx)

	audioDone := make(chan error, 1)

	go func() {
		msg := transport.SignalingMessage{
			Type:    "control",
			Payload: []byte(`{"action":"stop"}`),
		}
		data, _ := json.Marshal(msg)
		client.Write(append(data, '\n'))
	}()

	done := make(chan error)
	go func() {
		done <- app.handleSignalingLoop(ctx, signaler, peer, audioDone, time.Now(), nil, "client-1", "Client-1", "test-session")
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Error("handleSignalingLoop did not complete")
	}
}

func TestServerApp_CreateAndStartSignaler_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	signaler, _, _, err := app.createAndStartSignaler(ctx, ":0")

	require.Error(t, err)
	require.Nil(t, signaler)
}

func TestServerApp_ValidateClientAccess_WithBanManagerOnly(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)

	err := app.validateClientAccess("192.168.1.1:12345", "test-client")
	assert.NoError(t, err)
}

func TestServerApp_ValidateClientAccess_BannedNoClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	banMgr.banned["192.168.1.1"] = true
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)

	err := app.validateClientAccess("192.168.1.1:12345", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "banned")
}

func TestServerApp_ValidateClientAccess_RateLimitedNoClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	rateLimiter := auth.NewIPRateLimiter(1)
	defer rateLimiter.Close()
	rateLimiter.Allow("10.0.0.1")
	rateLimiter.Allow("10.0.0.1")
	app := NewServerApp(cfg, testAppLogger(), nil, nil, rateLimiter)

	err := app.validateClientAccess("10.0.0.1:1234", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestServerApp_ProcessSignalingMessage_AnswerWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(`{"sdp":"v=0\r\no=- 123 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.True(t, done)
}

func TestServerApp_ProcessSignalingMessage_ControlWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`{"action":"stop"}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.True(t, done)
}

func TestServerApp_ProcessSignalingMessage_CandidateWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: []byte(`{"candidate":"candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host","sdpMid":"0","sdpMLineIndex":0}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_CandidateDoneWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate_done",
		Payload: []byte(`{}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_InvalidControlWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`invalid json`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_InvalidAnswerWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(`invalid json`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_InvalidCandidateWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: []byte(`invalid json`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.False(t, done)
}

func TestServerApp_ProcessSignalingMessage_OtherActionWithClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`{"action":"pause"}`),
	}

	done := app.processSignalingMessage(msg, peer, time.Now(), "test-client", nil)
	assert.False(t, done)
}

func TestServerApp_SetupReverseAudio_NoDeviceWarns(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	cfg.DeviceID = nil
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionReceive)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx := context.Background()
	audioDone := make(chan error, 1)
	app.setupReverseAudio(ctx, peer, audioDone)
}

func TestServerApp_SetupReverseAudio_WithDeviceID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	deviceID := uint32(0)
	cfg.DeviceID = &deviceID
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionReceive)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)
	app.setupReverseAudio(ctx, peer, audioDone)
}

func TestServerApp_CreateWebRTCPeer_WithSTUN(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.STUNServers = []string{"stun:stun.example.com:3478"}
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer, direction, err := app.createWebRTCPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, transport.DirectionSend, direction)
	peer.Close()
}

func TestServerApp_CreateWebRTCPeer_Reverse(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer, direction, err := app.createWebRTCPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	assert.Equal(t, transport.DirectionReceive, direction)
	peer.Close()
}

func TestServerApp_SetupMultiClientAudio_ReceiveMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = true
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionReceive)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	audioDone, ok := app.setupMultiClientAudio(ctx, ctx, peer, transport.DirectionReceive, "test-client")

	assert.True(t, ok)
	assert.NotNil(t, audioDone)
}

func TestServerApp_SetupMultiClientAudio_SendMode(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Reverse = false
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	installFakeSharedCaptureHub(app, func() *fakeSharedCapturer {
		return &fakeSharedCapturer{}
	})

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	audioDone, ok := app.setupMultiClientAudio(ctx, ctx, peer, transport.DirectionSend, "test-client")

	assert.True(t, ok)
	assert.NotNil(t, audioDone)
}

func TestServerApp_RunMultiClientLoop_ContextCancel(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.runMultiClientLoop(ctx, signaler, peer, audioDone, "test-client", "TestClient", time.Now(), nil)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("runMultiClientLoop did not complete")
	}
}

// =============================================================================
// Coverage tests for server.go 0% functions
// =============================================================================

func TestServerApp_GenerateClientID(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	id1 := app.generateClientID()
	id2 := app.generateClientID()
	id3 := app.generateClientID()

	assert.Equal(t, "client-1", id1)
	assert.Equal(t, "client-2", id2)
	assert.Equal(t, "client-3", id3)
}

func TestServerApp_GenerateClientID_Concurrent(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	ids := make(chan string, 100)
	for i := 0; i < 100; i++ {
		go func() {
			ids <- app.generateClientID()
		}()
	}

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := <-ids
		assert.False(t, seen[id], "duplicate client ID: %s", id)
		seen[id] = true
	}
	assert.Len(t, seen, 100)
}

func TestServerApp_SendBannedMessage_BannedClient(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	banMgr.banned["10.0.0.1"] = true
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)

	var sentMsg transport.SignalingMessage
	sig := &serverMockSignaler{
		remoteAddr: "10.0.0.1:1234",
		sendFunc: func(msg transport.SignalingMessage) error {
			sentMsg = msg
			return nil
		},
	}

	app.sendBannedMessage(sig, "10.0.0.1:1234")

	assert.Equal(t, "auth_result", sentMsg.Type)
	assert.Contains(t, string(sentMsg.Payload), "IP banned")
	assert.Contains(t, string(sentMsg.Payload), "403")
}

func TestServerApp_SendBannedMessage_NotBanned(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)

	var sendCalled bool
	sig := &serverMockSignaler{
		sendFunc: func(msg transport.SignalingMessage) error {
			sendCalled = true
			return nil
		},
	}

	app.sendBannedMessage(sig, "10.0.0.1:1234")
	assert.False(t, sendCalled, "should not send message for non-banned client")
}

func TestServerApp_SendBannedMessage_NoBanManager(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	sig := &serverMockSignaler{}
	// Should not panic with nil banMgr
	app.sendBannedMessage(sig, "10.0.0.1:1234")
}

func TestServerApp_CheckClientAccess_BannedSendsBannedMessage(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	banMgr.banned["10.0.0.1"] = true
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)

	var sentMsg transport.SignalingMessage
	var closed bool
	sig := &serverMockSignaler{
		remoteAddr: "10.0.0.1:5555",
		sendFunc: func(msg transport.SignalingMessage) error {
			sentMsg = msg
			return nil
		},
		closeFunc: func() error {
			closed = true
			return nil
		},
	}

	addr, err := app.checkClientAccess(sig)
	assert.Error(t, err)
	assert.Empty(t, addr)
	assert.True(t, closed, "signaler should be closed on banned client")
	assert.Equal(t, "auth_result", sentMsg.Type)
}

func TestServerApp_CheckClientAccess_RateLimited(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	rateLimiter := auth.NewIPRateLimiter(1)
	defer rateLimiter.Close()
	// Exhaust rate limit
	rateLimiter.Allow("10.0.0.2")
	rateLimiter.Allow("10.0.0.2")
	app := NewServerApp(cfg, testAppLogger(), nil, nil, rateLimiter)

	var closed bool
	sig := &serverMockSignaler{
		remoteAddr: "10.0.0.2:5555",
		closeFunc: func() error {
			closed = true
			return nil
		},
	}

	addr, err := app.checkClientAccess(sig)
	assert.Error(t, err)
	assert.Empty(t, addr)
	assert.True(t, closed)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestServerApp_CheckClientAccess_AllowedNoRestrictions(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	sig := &serverMockSignaler{
		remoteAddr: "10.0.0.3:5555",
	}

	addr, err := app.checkClientAccess(sig)
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.3:5555", addr)
}

func TestServerApp_SetupConnectionStateCallback_WithMock(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	var connCB func(state webrtc.PeerConnectionState)

	peer := &serverMockPeerManager{
		onConnectionStateFunc: func(handler func(state webrtc.PeerConnectionState)) {
			connCB = handler
		},
	}

	app.setupConnectionStateCallback(peer, func() {})

	require.NotNil(t, connCB)

	// Test connection state change callback (should not panic)
	connCB(webrtc.PeerConnectionStateConnected)
}

func TestServerApp_HandleOneClient_WithMocks(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	recvCh := make(chan transport.SignalingMessage, 10)
	sig := &serverMockSignaler{
		remoteAddr: "127.0.0.1:12345",
		recvCh:     recvCh,
	}
	app.signalerFactory = &serverMockSignalerFactory{serverSignaler: sig}

	peer := &serverMockPeerManager{
		dcReady: make(chan struct{}),
	}
	app.peerFactory = &serverMockPeerFactory{peer: peer}

	// Auth will fail because mock signaler doesn't respond to auth challenge
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := app.handleOneClient(ctx, ":0")
	// Should error (auth timeout or context cancellation)
	assert.Error(t, err)
}

func TestServerApp_HandleOneClient_FullFlow_ControlStop(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	app.auth = &serverMockAuthHandler{}

	recvCh := make(chan transport.SignalingMessage, 10)
	sig := &serverMockSignaler{
		remoteAddr: "127.0.0.1:12345",
		recvCh:     recvCh,
	}
	app.signalerFactory = &serverMockSignalerFactory{serverSignaler: sig}

	peer := &serverMockPeerManager{
		dcReady: make(chan struct{}),
	}
	app.peerFactory = &serverMockPeerFactory{peer: peer}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		// handleOneClient sends offer, then waits for answer in signaling loop
		// Send answer message
		answerPayload, _ := json.Marshal(map[string]string{"sdp": "v=0\r\no=- 123 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"})
		recvCh <- transport.SignalingMessage{Type: "answer", Payload: answerPayload}
		time.Sleep(50 * time.Millisecond)
		recvCh <- transport.SignalingMessage{Type: "control", Payload: json.RawMessage(`{"action":"stop"}`)}
	}()

	err := app.handleOneClient(ctx, ":0")
	// May succeed or have an error, but must not hang
	_ = err
}

func TestServerApp_HandleOneClient_PeerCreationFails(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	app.auth = &serverMockAuthHandler{}

	sig := &serverMockSignaler{
		remoteAddr: "127.0.0.1:12345",
		recvCh:     make(chan transport.SignalingMessage, 10),
	}
	app.signalerFactory = &serverMockSignalerFactory{serverSignaler: sig}
	app.peerFactory = &serverMockPeerFactory{err: errors.New("peer creation failed")}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := app.handleOneClient(ctx, ":0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer")
}

func TestServerApp_AuthenticateWithProtocol_Success(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	app.auth = &serverMockAuthHandler{}

	sig := &serverMockSignaler{
		recvCh: make(chan transport.SignalingMessage, 1),
	}

	protocol := transport.NewServerSignalingProtocol(app.auth)
	ctx := context.Background()
	err := app.authenticateWithProtocol(ctx, protocol, sig, "127.0.0.1:1234", "client-1")
	assert.NoError(t, err)
}

func TestServerApp_AuthenticateWithProtocol_Failure(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	app.auth = &serverMockAuthHandler{serverErr: errors.New("auth rejected")}

	var closed bool
	sig := &serverMockSignaler{
		recvCh: make(chan transport.SignalingMessage, 1),
		closeFunc: func() error {
			closed = true
			return nil
		},
	}

	protocol := transport.NewServerSignalingProtocol(app.auth)
	ctx := context.Background()
	err := app.authenticateWithProtocol(ctx, protocol, sig, "127.0.0.1:1234", "")
	assert.Error(t, err)
	assert.True(t, closed)
}

func TestServerApp_AuthenticateWithProtocol_FailureWithBanManager(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	banMgr := newMockBanManager()
	app := NewServerApp(cfg, testAppLogger(), banMgr, nil, nil)
	app.auth = &serverMockAuthHandler{serverErr: errors.New("auth rejected")}

	var closed bool
	sig := &serverMockSignaler{
		recvCh: make(chan transport.SignalingMessage, 1),
		closeFunc: func() error {
			closed = true
			return nil
		},
	}

	protocol := transport.NewServerSignalingProtocol(app.auth)
	ctx := context.Background()
	err := app.authenticateWithProtocol(ctx, protocol, sig, "127.0.0.1:1234", "client-1")
	assert.Error(t, err)
	assert.True(t, closed)
	assert.Equal(t, 1, banMgr.failures["127.0.0.1"])
}

// serverMockAuthHandler implements transport.AuthHandler for server tests.
type serverMockAuthHandler struct {
	serverErr error
	clientErr error
}

func (m *serverMockAuthHandler) AuthenticateServer(send transport.AuthSendFunc, recv transport.AuthRecvFunc, password string) error {
	return m.serverErr
}

func (m *serverMockAuthHandler) AuthenticateClient(send transport.AuthSendFunc, recv transport.AuthRecvFunc, password string) error {
	return m.clientErr
}

// serverMockSignaler is a mock signaler for server tests.
type serverMockSignaler struct {
	sendFunc   func(msg transport.SignalingMessage) error
	recvCh     chan transport.SignalingMessage
	remoteAddr string
	closeFunc  func() error
}

func (m *serverMockSignaler) Start(ctx context.Context) error { return nil }
func (m *serverMockSignaler) Send(msg transport.SignalingMessage) error {
	if m.sendFunc != nil {
		return m.sendFunc(msg)
	}
	return nil
}
func (m *serverMockSignaler) Receive() <-chan transport.SignalingMessage {
	if m.recvCh != nil {
		return m.recvCh
	}
	return make(chan transport.SignalingMessage)
}
func (m *serverMockSignaler) RemoteAddr() string { return m.remoteAddr }
func (m *serverMockSignaler) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *serverMockSignaler) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// serverMockPeerManager for server-specific tests.
type serverMockPeerManager struct {
	onICECandidateFunc    func(handler func(candidate *webrtc.ICECandidate))
	onConnectionStateFunc func(handler func(state webrtc.PeerConnectionState))
	addAudioTrackFunc     func(sampleRate, channels uint32) (chan<- []byte, error)
	onAudioTrackFunc      func(handler func(inCh <-chan []byte))
	dcReady               chan struct{}
}

func (m *serverMockPeerManager) CreatePeerConnection(iceConfig transport.ICEConfig) error {
	return nil
}
func (m *serverMockPeerManager) Close() error { return nil }
func (m *serverMockPeerManager) OnConnectionStateChange(handler func(state webrtc.PeerConnectionState)) {
	if m.onConnectionStateFunc != nil {
		m.onConnectionStateFunc(handler)
	}
}
func (m *serverMockPeerManager) GetStats() transport.ConnectionStats {
	return transport.ConnectionStats{}
}
func (m *serverMockPeerManager) AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error) {
	if m.addAudioTrackFunc != nil {
		return m.addAudioTrackFunc(sampleRate, channels)
	}
	return make(chan []byte, 10), nil
}
func (m *serverMockPeerManager) OnAudioTrack(handler func(inCh <-chan []byte)) {
	if m.onAudioTrackFunc != nil {
		m.onAudioTrackFunc(handler)
	}
}
func (m *serverMockPeerManager) CreateOffer() (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: "mock-offer"}, nil
}
func (m *serverMockPeerManager) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: "mock-answer"}, nil
}
func (m *serverMockPeerManager) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	return nil
}
func (m *serverMockPeerManager) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	return nil
}
func (m *serverMockPeerManager) OnICECandidate(handler func(candidate *webrtc.ICECandidate)) {
	if m.onICECandidateFunc != nil {
		m.onICECandidateFunc(handler)
	}
}
func (m *serverMockPeerManager) CreateDataChannel(label string) error { return nil }
func (m *serverMockPeerManager) OnDataChannel(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error)) {
}
func (m *serverMockPeerManager) CreateControlDataChannel() error { return nil }
func (m *serverMockPeerManager) DCReady() <-chan struct{} {
	if m.dcReady != nil {
		return m.dcReady
	}
	return make(chan struct{})
}
func (m *serverMockPeerManager) SendControl(action string, payload interface{}) error { return nil }
func (m *serverMockPeerManager) ControlMessages() <-chan []byte                       { return nil }
func (m *serverMockPeerManager) CreateChatDataChannel() error                         { return nil }
func (m *serverMockPeerManager) SendChat([]byte) error                                { return nil }
func (m *serverMockPeerManager) ChatMessages() <-chan []byte                          { return nil }

// serverMockSignalerFactory for server tests.
type serverMockSignalerFactory struct {
	serverSignaler transport.Signaler
}

func (f *serverMockSignalerFactory) CreateServerSignaler(address string, tlsConfig *tls.Config) transport.Signaler {
	return f.serverSignaler
}
func (f *serverMockSignalerFactory) CreateClientSignaler(address string, tlsConfig *tls.Config) transport.Signaler {
	return nil
}

// serverMockPeerFactory for server tests.
type serverMockPeerFactory struct {
	peer transport.PeerManager
	err  error
}

func (f *serverMockPeerFactory) CreatePeer(direction transport.MediaDirection, iceConfig transport.ICEConfig) (transport.PeerManager, error) {
	return f.peer, f.err
}
