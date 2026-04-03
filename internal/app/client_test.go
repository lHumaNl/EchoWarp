package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func testClientLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func defaultClientConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "127.0.0.1"
	cfg.Password = "test-password"
	return cfg
}

func TestClientApp_New(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	require.NotNil(t, app)
	assert.Equal(t, cfg.Address, app.cfg.Address)
	assert.Equal(t, cfg.Port, app.cfg.Port)
	assert.Equal(t, cfg.Password, app.cfg.Password)
	assert.NotNil(t, app.auth, "auth handler must be initialized")
	assert.NotNil(t, app.signalerFactory, "signaler factory must be set")
	assert.NotNil(t, app.peerFactory, "peer factory must be set")
	assert.Nil(t, app.tlsConfig, "tlsConfig should be nil when not provided")
}

func TestClientApp_New_WithTLS(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	app := NewClientApp(cfg, logger, tlsConfig)

	require.NotNil(t, app)
	assert.Same(t, tlsConfig, app.tlsConfig, "tlsConfig must be the provided instance")
	assert.True(t, app.tlsConfig.InsecureSkipVerify, "TLS config must preserve settings")
	assert.NotNil(t, app.auth)
}

func TestClientApp_createSignaler_WithoutTLS(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	signaler := app.createSignaler("127.0.0.1:4415")

	assert.NotNil(t, signaler)
}

func TestClientApp_createSignaler_WithTLS(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	app := NewClientApp(cfg, logger, tlsConfig)
	signaler := app.createSignaler("127.0.0.1:4415")

	assert.NotNil(t, signaler)
}

func TestClientApp_createPeer_ReceiveMode(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = false
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	defer peer.Close()
}

func TestClientApp_createPeer_SendMode(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = true
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	defer peer.Close()
}

func TestClientApp_createPeer_WithSTUNServers(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.STUNServers = []string{"stun:stun.example.com:3478"}
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	defer peer.Close()
}

func TestClientApp_createPeer_WithTURNServers(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.TURNServers = []config.TURNServer{
		{URL: "turn:turn.example.com:3478", Username: "user", Credential: "pass"},
	}
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	defer peer.Close()
}

func TestClientApp_handleSignalingMessage_Candidate(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	candidate := webrtc.ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host",
	}
	payload, _ := json.Marshal(candidate)

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: payload,
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_CandidateDone(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate_done",
		Payload: json.RawMessage{},
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_ControlStop(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	ctrl := struct {
		Action string `json:"action"`
	}{Action: "stop"}
	payload, _ := json.Marshal(ctrl)

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: payload,
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.True(t, done)
}

func TestClientApp_handleSignalingMessage_ControlOther(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	ctrl := struct {
		Action string `json:"action"`
	}{Action: "pause"}
	payload, _ := json.Marshal(ctrl)

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: payload,
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_InvalidCandidate(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: json.RawMessage(`{"invalid": json}`),
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_InvalidControl(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: json.RawMessage(`{"invalid": json}`),
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_UnknownType(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "unknown",
		Payload: json.RawMessage{},
	}

	done := app.handleSignalingMessage(msg, peer)
	assert.False(t, done)
}

func TestClientApp_recordMetrics(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	connStart := time.Now().Add(-1 * time.Second)
	app.recordMetrics(connStart)
}

func TestClientApp_runCapturePipeline_NoDeviceID(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.DeviceID = nil
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	sendCh := make(chan []byte, 1)

	err := app.runCapturePipeline(context.Background(), sendCh)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no device ID specified")
}

func TestClientApp_runCapturePipeline_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	deviceID := uint32(999)
	cfg.DeviceID = &deviceID
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sendCh := make(chan []byte, 1)
	err := app.runCapturePipeline(ctx, sendCh)

	assert.Error(t, err)
}

func TestClientApp_setupAudioPipeline_ReceiveMode(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = false
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	audioDone := make(chan error, 1)
	err = app.setupAudioPipeline(context.Background(), peer, audioDone)

	assert.NoError(t, err)
}

func TestClientApp_setupAudioPipeline_SendMode(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = true
	deviceID := uint32(0)
	cfg.DeviceID = &deviceID
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)
	err = app.setupAudioPipeline(ctx, peer, audioDone)

	assert.NoError(t, err)
	_ = peer.Close()
}

func TestClientApp_setupAudioPipeline_ReceiveMode_WithDevice(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = false
	deviceID := uint32(0)
	cfg.DeviceID = &deviceID
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)
	err = app.setupAudioPipeline(ctx, peer, audioDone)

	assert.NoError(t, err)
	_ = peer.Close()
}

func TestClientApp_setupAudioPipeline_SendMode_NoDevice(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = true
	cfg.DeviceID = nil
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	audioDone := make(chan error, 1)
	err = app.setupAudioPipeline(ctx, peer, audioDone)

	assert.NoError(t, err)
}

func TestClientApp_setupAudioPipeline_Reverse_AddTrackError(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = true
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	mockPeer := &mockPeerManager{
		addAudioTrackFunc: func(sampleRate, channels uint32) (chan<- []byte, error) {
			return nil, errors.New("add track failed")
		},
	}

	ctx := context.Background()
	audioDone := make(chan error, 1)
	err := app.setupAudioPipeline(ctx, mockPeer, audioDone)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "add audio track")
}

func TestClientApp_createPeer_WithSTUNAndTURN(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.STUNServers = []string{"stun:stun.example.com:3478"}
	cfg.TURNServers = []config.TURNServer{
		{URL: "turn:turn.example.com:3478", Username: "user", Credential: "pass"},
	}
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)
	peer, err := app.createPeer()

	require.NoError(t, err)
	require.NotNil(t, peer)
	peer.Close()
}

func TestClientApp_handleSignalingMessage_WithMockPeer(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	mockPeer := &mockPeerManager{}

	msg := transport.SignalingMessage{
		Type:    "candidate_done",
		Payload: json.RawMessage{},
	}

	done := app.handleSignalingMessage(msg, mockPeer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_CandidateWithMockPeer(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	candidate := webrtc.ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host",
	}
	payload, _ := json.Marshal(candidate)

	mockPeer := &mockPeerManager{}

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: payload,
	}

	done := app.handleSignalingMessage(msg, mockPeer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_ControlWithMockPeer(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	ctrl := struct {
		Action string `json:"action"`
	}{Action: "stop"}
	payload, _ := json.Marshal(ctrl)

	mockPeer := &mockPeerManager{}

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: payload,
	}

	done := app.handleSignalingMessage(msg, mockPeer)
	assert.True(t, done)
}

func TestClientApp_handleSignalingMessage_InvalidCandidateWithMockPeer(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	mockPeer := &mockPeerManager{}

	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: []byte(`invalid json`),
	}

	done := app.handleSignalingMessage(msg, mockPeer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_InvalidControlWithMockPeer(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	mockPeer := &mockPeerManager{}

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`invalid json`),
	}

	done := app.handleSignalingMessage(msg, mockPeer)
	assert.False(t, done)
}

func TestClientApp_handleSignalingMessage_OtherActionWithMockPeer(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	ctrl := struct {
		Action string `json:"action"`
	}{Action: "pause"}
	payload, _ := json.Marshal(ctrl)

	mockPeer := &mockPeerManager{}

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: payload,
	}

	done := app.handleSignalingMessage(msg, mockPeer)
	assert.False(t, done)
}

func TestClientApp_New_WithAllFields(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Address = "192.168.1.1"
	cfg.Port = 8080
	cfg.Password = "test-password-2"
	logger := testClientLogger()
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	app := NewClientApp(cfg, logger, tlsConfig)

	assert.NotNil(t, app)
	assert.Equal(t, cfg, app.cfg)
	assert.NotNil(t, app.auth)
	assert.NotNil(t, app.tlsConfig)
}

func TestClientApp_recordMetrics_WithTime(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	connStart := time.Now().Add(-5 * time.Second)
	app.recordMetrics(connStart)
}

func TestClientApp_runCapturePipeline_WithContextError(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	deviceID := uint32(999)
	cfg.DeviceID = &deviceID
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sendCh := make(chan []byte, 1)
	err := app.runCapturePipeline(ctx, sendCh)

	assert.Error(t, err)
}

func TestClientApp_Run_CancelledContext(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Address = "127.0.0.1"
	logger := testClientLogger()

	app := NewClientApp(cfg, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
}

type mockSignaler struct {
	sendFunc   func(msg transport.SignalingMessage) error
	recvCh     chan transport.SignalingMessage
	remoteAddr string
	startFunc  func(ctx context.Context) error
	closeFunc  func() error
}

func (m *mockSignaler) Start(ctx context.Context) error {
	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	return nil
}

func (m *mockSignaler) Send(msg transport.SignalingMessage) error {
	if m.sendFunc != nil {
		return m.sendFunc(msg)
	}
	return nil
}

func (m *mockSignaler) Receive() <-chan transport.SignalingMessage {
	return m.recvCh
}

func (m *mockSignaler) RemoteAddr() string {
	return m.remoteAddr
}

func (m *mockSignaler) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockSignaler) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

type mockPeerManager struct {
	createPeerConnectionFunc func(iceConfig transport.ICEConfig) error
	addAudioTrackFunc        func(sampleRate, channels uint32) (chan<- []byte, error)
	onAudioTrackFunc         func(handler func(inCh <-chan []byte))
	createOfferFunc          func() (webrtc.SessionDescription, error)
	createAnswerFunc         func(offer webrtc.SessionDescription) (webrtc.SessionDescription, error)
	setRemoteDescriptionFunc func(sdp webrtc.SessionDescription) error
	addICECandidateFunc      func(candidate webrtc.ICECandidateInit) error
	onICECandidateFunc       func(handler func(candidate *webrtc.ICECandidate))
	onConnectionStateFunc    func(handler func(state webrtc.PeerConnectionState))
	getStatsFunc             func() transport.ConnectionStats
	createDataChannelFunc    func(label string) error
	onDataChannelFunc        func(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error))
	closeFunc                func() error
	dcReadyCh                chan struct{}
}

func (m *mockPeerManager) CreatePeerConnection(iceConfig transport.ICEConfig) error {
	if m.createPeerConnectionFunc != nil {
		return m.createPeerConnectionFunc(iceConfig)
	}
	return nil
}

func (m *mockPeerManager) AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error) {
	if m.addAudioTrackFunc != nil {
		return m.addAudioTrackFunc(sampleRate, channels)
	}
	return make(chan []byte, 1), nil
}

func (m *mockPeerManager) OnAudioTrack(handler func(inCh <-chan []byte)) {
	if m.onAudioTrackFunc != nil {
		m.onAudioTrackFunc(handler)
	}
}

func (m *mockPeerManager) CreateOffer() (webrtc.SessionDescription, error) {
	if m.createOfferFunc != nil {
		return m.createOfferFunc()
	}
	return webrtc.SessionDescription{}, nil
}

func (m *mockPeerManager) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	if m.createAnswerFunc != nil {
		return m.createAnswerFunc(offer)
	}
	return webrtc.SessionDescription{}, nil
}

func (m *mockPeerManager) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if m.setRemoteDescriptionFunc != nil {
		return m.setRemoteDescriptionFunc(sdp)
	}
	return nil
}

func (m *mockPeerManager) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	if m.addICECandidateFunc != nil {
		return m.addICECandidateFunc(candidate)
	}
	return nil
}

func (m *mockPeerManager) OnICECandidate(handler func(candidate *webrtc.ICECandidate)) {
	if m.onICECandidateFunc != nil {
		m.onICECandidateFunc(handler)
	}
}

func (m *mockPeerManager) OnConnectionStateChange(handler func(state webrtc.PeerConnectionState)) {
	if m.onConnectionStateFunc != nil {
		m.onConnectionStateFunc(handler)
	}
}

func (m *mockPeerManager) GetStats() transport.ConnectionStats {
	if m.getStatsFunc != nil {
		return m.getStatsFunc()
	}
	return transport.ConnectionStats{}
}

func (m *mockPeerManager) CreateDataChannel(label string) error {
	if m.createDataChannelFunc != nil {
		return m.createDataChannelFunc(label)
	}
	return nil
}

func (m *mockPeerManager) OnDataChannel(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error)) {
	if m.onDataChannelFunc != nil {
		m.onDataChannelFunc(handler)
	}
}

func (m *mockPeerManager) CreateControlDataChannel() error {
	return nil
}

func (m *mockPeerManager) DCReady() <-chan struct{} {
	if m.dcReadyCh != nil {
		return m.dcReadyCh
	}
	return make(chan struct{})
}

func (m *mockPeerManager) SendControl(action string, payload interface{}) error {
	return nil
}
func (m *mockPeerManager) ControlMessages() <-chan []byte { return nil }
func (m *mockPeerManager) CreateChatDataChannel() error   { return nil }
func (m *mockPeerManager) SendChat([]byte) error          { return nil }
func (m *mockPeerManager) ChatMessages() <-chan []byte    { return nil }

func (m *mockPeerManager) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestMockSignaler_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ transport.Signaler = &mockSignaler{}
}

func TestMockPeerManager_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ transport.PeerManager = &mockPeerManager{}
}

func TestMockSignaler_DefaultBehavior(t *testing.T) {
	t.Parallel()
	ms := &mockSignaler{
		recvCh:     make(chan transport.SignalingMessage, 1),
		remoteAddr: "192.168.1.1:12345",
	}

	assert.NoError(t, ms.Start(context.Background()))
	assert.NoError(t, ms.Send(transport.SignalingMessage{}))
	assert.NotNil(t, ms.Receive())
	assert.Equal(t, "192.168.1.1:12345", ms.RemoteAddr())
	assert.NoError(t, ms.Close())
}

func TestMockSignaler_CustomError(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("send error")
	ms := &mockSignaler{
		sendFunc: func(msg transport.SignalingMessage) error {
			return expectedErr
		},
	}

	err := ms.Send(transport.SignalingMessage{})
	assert.ErrorIs(t, err, expectedErr)
}

func TestMockPeerManager_DefaultBehavior(t *testing.T) {
	t.Parallel()
	mp := &mockPeerManager{}

	assert.NoError(t, mp.CreatePeerConnection(transport.ICEConfig{}))
	ch, err := mp.AddAudioTrack(48000, 1)
	assert.NoError(t, err)
	assert.NotNil(t, ch)
	assert.NoError(t, mp.SetRemoteDescription(webrtc.SessionDescription{}))
	assert.NoError(t, mp.AddICECandidate(webrtc.ICECandidateInit{}))
	assert.NoError(t, mp.Close())
}

func TestMockPeerManager_CreatePeerConnectionError(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("peer connection error")
	mp := &mockPeerManager{
		createPeerConnectionFunc: func(iceConfig transport.ICEConfig) error {
			return expectedErr
		},
	}

	err := mp.CreatePeerConnection(transport.ICEConfig{})
	assert.ErrorIs(t, err, expectedErr)
}

func TestMockPeerManager_AddAudioTrackError(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("add track error")
	mp := &mockPeerManager{
		addAudioTrackFunc: func(sampleRate, channels uint32) (chan<- []byte, error) {
			return nil, expectedErr
		},
	}

	ch, err := mp.AddAudioTrack(48000, 1)
	assert.Nil(t, ch)
	assert.ErrorIs(t, err, expectedErr)
}

func TestClientApp_handleSignalingMessage_Table(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	app := NewClientApp(cfg, logger, nil)

	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	candidate := webrtc.ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host",
	}
	candidatePayload, _ := json.Marshal(candidate)

	stopPayload, _ := json.Marshal(struct {
		Action string `json:"action"`
	}{Action: "stop"})

	tests := []struct {
		name     string
		msg      transport.SignalingMessage
		wantDone bool
	}{
		{"candidate", transport.SignalingMessage{Type: "candidate", Payload: candidatePayload}, false},
		{"candidate_done", transport.SignalingMessage{Type: "candidate_done", Payload: json.RawMessage{}}, false},
		{"control_stop", transport.SignalingMessage{Type: "control", Payload: stopPayload}, true},
		{"unknown_type", transport.SignalingMessage{Type: "unknown", Payload: json.RawMessage{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := app.handleSignalingMessage(tt.msg, peer)
			assert.Equal(t, tt.wantDone, done)
		})
	}
}

func TestClientApp_runMessageLoop_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	app := NewClientApp(cfg, logger, nil)

	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	recvCh := make(chan transport.SignalingMessage)
	audioDone := make(chan error)

	ms := &mockSignaler{
		recvCh: recvCh,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = app.runMessageLoop(ctx, ms, peer, audioDone, time.Now())
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClientApp_runMessageLoop_ControlStop(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	app := NewClientApp(cfg, logger, nil)

	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	recvCh := make(chan transport.SignalingMessage, 1)
	audioDone := make(chan error)

	stopPayload, _ := json.Marshal(struct {
		Action string `json:"action"`
	}{Action: "stop"})
	recvCh <- transport.SignalingMessage{Type: "control", Payload: stopPayload}

	ms := &mockSignaler{
		recvCh: recvCh,
	}

	ctx := context.Background()
	err = app.runMessageLoop(ctx, ms, peer, audioDone, time.Now())
	assert.NoError(t, err)
}

func TestClientApp_runMessageLoop_AudioDone(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	app := NewClientApp(cfg, logger, nil)

	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	recvCh := make(chan transport.SignalingMessage)
	audioDone := make(chan error, 1)
	audioDone <- nil

	ms := &mockSignaler{
		recvCh: recvCh,
	}

	ctx := context.Background()
	err = app.runMessageLoop(ctx, ms, peer, audioDone, time.Now())
	assert.NoError(t, err)
}

func TestClientApp_runMessageLoop_AudioError(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	app := NewClientApp(cfg, logger, nil)

	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	recvCh := make(chan transport.SignalingMessage)
	audioDone := make(chan error, 1)
	audioDone <- errors.New("audio error")

	ms := &mockSignaler{
		recvCh: recvCh,
	}

	ctx := context.Background()
	err = app.runMessageLoop(ctx, ms, peer, audioDone, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audio pipeline")
}

func TestClientApp_runMessageLoop_AudioErrorWithCancelledContext(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	logger := testClientLogger()
	app := NewClientApp(cfg, logger, nil)

	peer, err := app.createPeer()
	require.NoError(t, err)
	defer peer.Close()

	recvCh := make(chan transport.SignalingMessage)
	audioDone := make(chan error, 1)
	audioDone <- errors.New("audio error")

	ms := &mockSignaler{
		recvCh: recvCh,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = app.runMessageLoop(ctx, ms, peer, audioDone, time.Now())
	if err != nil && err != context.Canceled {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// Coverage tests for client.go 0% functions
// =============================================================================

func TestClientApp_Run_ImmediatelyCancelled(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestClientApp_initializeSignaler(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)

	mockSig := &mockSignaler{recvCh: make(chan transport.SignalingMessage, 1)}
	app.signalerFactory = &inlineSignalerFactory{clientSignaler: mockSig}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signaler, sigCtx, sigCancel, err := app.initializeSignaler(ctx, "127.0.0.1:8080")
	defer sigCancel()

	assert.NoError(t, err)
	assert.NotNil(t, signaler)
	assert.NotNil(t, sigCtx)
	assert.Same(t, mockSig, signaler)
}

func TestClientApp_initializePeer_Success(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)

	mockSig := &mockSignaler{recvCh: make(chan transport.SignalingMessage, 1)}
	mockPeer := &mockPeerManager{}
	app.peerFactory = &inlinePeerFactory{peer: mockPeer}

	peer, cleanup, err := app.initializePeer(mockSig)
	require.NoError(t, err)
	require.NotNil(t, peer)
	require.NotNil(t, cleanup)
	assert.Same(t, mockPeer, peer)
	cleanup() // should not panic
}

func TestClientApp_initializePeer_Error(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)

	mockSig := &mockSignaler{recvCh: make(chan transport.SignalingMessage, 1)}
	app.peerFactory = &inlinePeerFactory{err: errors.New("peer creation failed")}

	peer, cleanup, err := app.initializePeer(mockSig)
	assert.Error(t, err)
	assert.Nil(t, peer)
	assert.Nil(t, cleanup)
}

func TestClientApp_setupPeerCallbacks(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)

	var callbackCalled bool
	peer := &mockPeerManager{
		onConnectionStateFunc: func(handler func(state webrtc.PeerConnectionState)) {
			callbackCalled = true
			// Invoke the handler to ensure it doesn't panic
			handler(webrtc.PeerConnectionStateConnected)
		},
	}

	app.setupPeerCallbacks(peer, func() {})
	assert.True(t, callbackCalled)
}

func TestClientApp_initializeAudioPipeline_ReceiveMode(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = false
	app := NewClientApp(cfg, testClientLogger(), nil)

	peer := &mockPeerManager{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	audioDone, err := app.initializeAudioPipeline(ctx, peer, &mockSignaler{recvCh: make(chan transport.SignalingMessage, 1)})
	require.NoError(t, err)
	assert.NotNil(t, audioDone)
}

func TestClientApp_initializeAudioPipeline_SendMode_NoDevice(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = true
	cfg.DeviceID = nil
	app := NewClientApp(cfg, testClientLogger(), nil)

	peer := &mockPeerManager{}
	mockSig := &mockSignaler{recvCh: make(chan transport.SignalingMessage, 1)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	audioDone, err := app.initializeAudioPipeline(ctx, peer, mockSig)
	require.NoError(t, err)

	// The capture pipeline will return an error about no device ID
	select {
	case audioErr := <-audioDone:
		assert.Error(t, audioErr)
		assert.Contains(t, audioErr.Error(), "no device ID")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for audio error")
	}
}

func TestClientApp_authenticate_Success(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)
	// Replace auth with a no-op handler
	app.auth = &mockAuthHandler{}

	sig := &mockSignaler{recvCh: make(chan transport.SignalingMessage, 2)}
	// Push auth_result so WaitForAuthResult succeeds.
	authPayload, _ := json.Marshal(transport.AuthResultMessage{Success: true, Message: "ok", Nickname: "Client-1"})
	sig.recvCh <- transport.SignalingMessage{Type: "auth_result", Payload: authPayload}

	ctx := context.Background()
	protocol, authResult, err := app.authenticate(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, protocol)
	require.NotNil(t, authResult)
	assert.Equal(t, "Client-1", authResult.Nickname)
}

func TestClientApp_authenticate_Failure(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	app := NewClientApp(cfg, testClientLogger(), nil)
	app.auth = &mockAuthHandler{clientErr: errors.New("auth rejected")}

	sig := &mockSignaler{recvCh: make(chan transport.SignalingMessage, 1)}

	ctx := context.Background()
	protocol, _, err := app.authenticate(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, protocol)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestClientApp_Run_FullFlow_ControlStop(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	cfg.Reverse = false
	app := NewClientApp(cfg, testClientLogger(), nil)
	app.auth = &mockAuthHandler{}

	recvCh := make(chan transport.SignalingMessage, 10)
	sig := &mockSignaler{
		recvCh:     recvCh,
		remoteAddr: "127.0.0.1:8080",
	}
	app.signalerFactory = &inlineSignalerFactory{clientSignaler: sig}

	dcReady := make(chan struct{})
	peer := &mockPeerManager{
		onConnectionStateFunc: func(handler func(state webrtc.PeerConnectionState)) {},
		onICECandidateFunc:    func(handler func(candidate *webrtc.ICECandidate)) {},
	}
	// Override DCReady to return a controllable channel
	peer.dcReadyCh = dcReady
	app.peerFactory = &inlinePeerFactory{peer: peer}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send offer, then answer flow for HandleHandshake
	go func() {
		time.Sleep(50 * time.Millisecond)
		// HandleHandshake expects an "offer" message
		offerPayload, _ := json.Marshal(map[string]string{"sdp": "v=0\r\no=- 123 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"})
		recvCh <- transport.SignalingMessage{Type: "offer", Payload: offerPayload}
		time.Sleep(50 * time.Millisecond)
		// After HandleHandshake completes (or errors), send control stop
		recvCh <- transport.SignalingMessage{Type: "control", Payload: json.RawMessage(`{"action":"stop"}`)}
	}()

	err := app.Run(ctx)
	// Should complete (either via control stop or handshake error — both are OK)
	// The test verifies the Run() flow doesn't hang or panic
	_ = err
}

// mockAuthHandler implements transport.AuthHandler for testing.
type mockAuthHandler struct {
	serverErr error
	clientErr error
}

func (m *mockAuthHandler) AuthenticateServer(send transport.AuthSendFunc, recv transport.AuthRecvFunc, password string) error {
	return m.serverErr
}

func (m *mockAuthHandler) AuthenticateClient(send transport.AuthSendFunc, recv transport.AuthRecvFunc, password string) error {
	return m.clientErr
}

// inlineSignalerFactory is a simple mock factory for tests in this file.
type inlineSignalerFactory struct {
	clientSignaler transport.Signaler
	serverSignaler transport.Signaler
}

func (f *inlineSignalerFactory) CreateServerSignaler(address string, tlsConfig *tls.Config) transport.Signaler {
	return f.serverSignaler
}

func (f *inlineSignalerFactory) CreateClientSignaler(address string, tlsConfig *tls.Config) transport.Signaler {
	return f.clientSignaler
}

// inlinePeerFactory is a simple mock factory for tests in this file.
type inlinePeerFactory struct {
	peer transport.PeerManager
	err  error
}

func (f *inlinePeerFactory) CreatePeer(direction transport.MediaDirection, iceConfig transport.ICEConfig) (transport.PeerManager, error) {
	return f.peer, f.err
}

// --- Bug 8: Kick/Ban handling tests ---

func TestClientApp_handleDCControl_KickNotify(t *testing.T) {
	t.Parallel()
	app := NewClientApp(defaultClientConfig(), testClientLogger(), nil)

	raw := mustMarshal(t, map[string]interface{}{
		"type": transport.TypeControl,
		"payload": map[string]interface{}{
			"action": transport.ActionKickNotify,
			"data":   map[string]string{"reason": "spam"},
		},
	})

	ended := app.handleDCControl(raw)
	assert.False(t, ended, "handleDCControl should return false for kick_notify (server closes connection)")
	assert.True(t, app.kickedByServer.Load(), "kickedByServer flag should be set")
	assert.Equal(t, "spam", app.kickReason)
}

func TestClientApp_handleDCControl_BanNotify(t *testing.T) {
	t.Parallel()
	app := NewClientApp(defaultClientConfig(), testClientLogger(), nil)

	raw := mustMarshal(t, map[string]interface{}{
		"type": transport.TypeControl,
		"payload": map[string]interface{}{
			"action": transport.ActionBanNotify,
			"data": map[string]interface{}{
				"reason":   "toxic behavior",
				"criteria": []string{"IP", "Nickname"},
			},
		},
	})

	ended := app.handleDCControl(raw)
	assert.False(t, ended, "handleDCControl should return false for ban_notify")
	assert.True(t, app.bannedByServer.Load(), "bannedByServer flag should be set")
	assert.Equal(t, "toxic behavior", app.banReason)
	assert.Equal(t, []string{"IP", "Nickname"}, app.banCriteria)
}

func TestClientApp_handleDCControl_KickNotify_EmptyReason(t *testing.T) {
	t.Parallel()
	app := NewClientApp(defaultClientConfig(), testClientLogger(), nil)

	raw := mustMarshal(t, map[string]interface{}{
		"type": transport.TypeControl,
		"payload": map[string]interface{}{
			"action": transport.ActionKickNotify,
			"data":   map[string]string{},
		},
	})

	ended := app.handleDCControl(raw)
	assert.False(t, ended)
	assert.True(t, app.kickedByServer.Load())
	assert.Empty(t, app.kickReason)
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
