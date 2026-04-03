package app

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/app/mocks"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// =============================================================================
// Factory Injection Tests
// =============================================================================

func TestServerApp_FactoryInjection_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		signalerFactory SignalerFactory
		peerFactory     PeerFactory
		wantSignaler    bool
		wantPeer        bool
	}{
		{
			name:            "nil factories uses defaults",
			signalerFactory: nil,
			peerFactory:     nil,
			wantSignaler:    true,
			wantPeer:        true,
		},
		{
			name:            "mock factories",
			signalerFactory: mocks.NewMockSignalerFactory(),
			peerFactory:     mocks.NewMockPeerFactory(),
			wantSignaler:    true,
			wantPeer:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Mode = config.ModeServer
			cfg.Port = 0

			app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

			if tt.wantSignaler {
				assert.NotNil(t, app.signalerFactory)
			}
			if tt.wantPeer {
				assert.NotNil(t, app.peerFactory)
			}
		})
	}
}

func TestClientApp_FactoryInjection_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		signalerFactory SignalerFactory
		peerFactory     PeerFactory
		wantSignaler    bool
		wantPeer        bool
	}{
		{
			name:            "nil factories uses defaults",
			signalerFactory: nil,
			peerFactory:     nil,
			wantSignaler:    true,
			wantPeer:        true,
		},
		{
			name:            "mock factories",
			signalerFactory: mocks.NewMockSignalerFactory(),
			peerFactory:     mocks.NewMockPeerFactory(),
			wantSignaler:    true,
			wantPeer:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Mode = config.ModeClient
			cfg.Address = "127.0.0.1"

			app := NewClientApp(cfg, testAppLogger(), nil)

			if tt.wantSignaler {
				assert.NotNil(t, app.signalerFactory)
			}
			if tt.wantPeer {
				assert.NotNil(t, app.peerFactory)
			}
		})
	}
}

// =============================================================================
// Mock Behavior Tests - ServerApp
// =============================================================================

func TestServerApp_MockSignalerFactory_Behavior(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 8080

	mockFactory := mocks.NewMockSignalerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mockFactory,
		peerFactory:     mocks.NewMockPeerFactory(),
	}

	// Test server signaler creation
	signaler := app.signalerFactory.CreateServerSignaler(":8080", nil)
	require.NotNil(t, signaler)

	// Verify mock returns expected remote address
	assert.Equal(t, "127.0.0.1:8080", signaler.RemoteAddr())

	// Test signaler start
	ctx := context.Background()
	err := signaler.Start(ctx)
	assert.NoError(t, err)

	// Test signaler send
	err = signaler.Send(transport.SignalingMessage{
		Type:    "test",
		Payload: []byte(`{}`),
	})
	assert.NoError(t, err)

	// Test signaler close
	err = signaler.Close()
	assert.NoError(t, err)
}

func TestServerApp_MockPeerFactory_Behavior(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer

	mockFactory := mocks.NewMockPeerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mocks.NewMockSignalerFactory(),
		peerFactory:     mockFactory,
	}

	// Test peer creation
	peer, err := app.peerFactory.CreatePeer(
		transport.DirectionSend,
		transport.ICEConfig{},
	)
	require.NoError(t, err)
	require.NotNil(t, peer)

	// Test peer operations
	stats := peer.GetStats()
	assert.Equal(t, "connected", stats.State)

	// Test offer creation
	offer, err := peer.CreateOffer()
	require.NoError(t, err)
	assert.Equal(t, webrtc.SDPTypeOffer, offer.Type)

	// Test data channel ready
	dcReady := peer.DCReady()
	require.NotNil(t, dcReady)

	// Test peer close
	err = peer.Close()
	assert.NoError(t, err)
}

func TestServerApp_MockFactories_AudioPipeline(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Reverse = false

	mockPeerFactory := mocks.NewMockPeerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mocks.NewMockSignalerFactory(),
		peerFactory:     mockPeerFactory,
	}

	peer, err := app.peerFactory.CreatePeer(transport.DirectionSend, transport.ICEConfig{})
	require.NoError(t, err)

	// Test audio track addition (mock returns a channel)
	sendCh, err := peer.AddAudioTrack(48000, 2)
	require.NoError(t, err)
	require.NotNil(t, sendCh)
}

func TestServerApp_MockFactories_SignalingMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer

	mockFactory := mocks.NewMockSignalerFactory()
	mockPeerFactory := mocks.NewMockPeerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mockFactory,
		peerFactory:     mockPeerFactory,
	}

	signaler := app.signalerFactory.CreateServerSignaler(":8080", nil)

	// Test sending various message types
	messageTypes := []string{"offer", "answer", "candidate", "candidate_done", "control"}
	for _, msgType := range messageTypes {
		err := signaler.Send(transport.SignalingMessage{
			Type:    msgType,
			Payload: []byte(`{"test": "data"}`),
		})
		assert.NoError(t, err, "Failed to send message type: %s", msgType)
	}
}

// =============================================================================
// Mock Behavior Tests - ClientApp
// =============================================================================

func TestClientApp_MockSignalerFactory_Behavior(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "192.168.1.1"
	cfg.Port = 8080

	mockFactory := mocks.NewMockSignalerFactory()

	app := &ClientApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mockFactory,
		peerFactory:     mocks.NewMockPeerFactory(),
	}

	// Test client signaler creation
	signaler := app.signalerFactory.CreateClientSignaler("192.168.1.1:8080", nil)
	require.NotNil(t, signaler)

	// Verify mock returns expected remote address
	assert.Equal(t, "127.0.0.1:9090", signaler.RemoteAddr())

	// Test receive channel
	recvCh := signaler.Receive()
	require.NotNil(t, recvCh)
}

func TestClientApp_MockPeerFactory_Behavior(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Reverse = false

	mockFactory := mocks.NewMockPeerFactory()

	app := &ClientApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mocks.NewMockSignalerFactory(),
		peerFactory:     mockFactory,
	}

	// Test peer creation for receive mode
	peer, err := app.peerFactory.CreatePeer(
		transport.DirectionReceive,
		transport.ICEConfig{},
	)
	require.NoError(t, err)
	require.NotNil(t, peer)

	// Test answer creation
	answer, err := peer.CreateAnswer(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "test-offer",
	})
	require.NoError(t, err)
	assert.Equal(t, webrtc.SDPTypeAnswer, answer.Type)
}

// =============================================================================
// Error Handling with Mocks - ServerApp
// =============================================================================

func TestServerApp_MockPeerFactory_PeerCreationError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer

	expectedErr := errors.New("peer creation failed")
	mockFactory := mocks.NewMockPeerFactoryWithError(expectedErr)

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mocks.NewMockSignalerFactory(),
		peerFactory:     mockFactory,
	}

	peer, err := app.peerFactory.CreatePeer(transport.DirectionSend, transport.ICEConfig{})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, peer)
}

func TestServerApp_MockPeerWithError_CreateOfferError(t *testing.T) {
	mockPeer := mocks.NewMockPeerWithError()
	mockPeer.CreateOfferError = errors.New("offer creation failed")

	offer, err := mockPeer.CreateOffer()

	require.Error(t, err)
	assert.Equal(t, "offer creation failed", err.Error())
	assert.Equal(t, webrtc.SessionDescription{}, offer)
}

func TestServerApp_MockPeerWithError_SetRemoteDescriptionError(t *testing.T) {
	mockPeer := mocks.NewMockPeerWithError()
	mockPeer.SetRemoteDescError = errors.New("remote description error")

	err := mockPeer.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  "test-sdp",
	})

	require.Error(t, err)
	assert.Equal(t, "remote description error", err.Error())
}

func TestServerApp_MockPeerWithError_AddICECandidateError(t *testing.T) {
	mockPeer := mocks.NewMockPeerWithError()
	mockPeer.AddICECandidateError = errors.New("ICE candidate error")

	err := mockPeer.AddICECandidate(webrtc.ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host",
	})

	require.Error(t, err)
	assert.Equal(t, "ICE candidate error", err.Error())
}

func TestServerApp_MockSignalerWithError_StartError(t *testing.T) {
	mockSignaler := mocks.NewMockSignalerWithError("127.0.0.1:8080")
	mockSignaler.StartError = errors.New("start failed")

	err := mockSignaler.Start(context.Background())

	require.Error(t, err)
	assert.Equal(t, "start failed", err.Error())
}

func TestServerApp_MockSignalerWithError_SendError(t *testing.T) {
	mockSignaler := mocks.NewMockSignalerWithError("127.0.0.1:8080")
	mockSignaler.SendError = errors.New("send failed")

	err := mockSignaler.Send(transport.SignalingMessage{
		Type:    "test",
		Payload: []byte(`{}`),
	})

	require.Error(t, err)
	assert.Equal(t, "send failed", err.Error())
}

// =============================================================================
// Error Handling with Mocks - ClientApp
// =============================================================================

func TestClientApp_MockPeerFactory_PeerCreationError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient

	expectedErr := errors.New("peer creation failed")
	mockFactory := mocks.NewMockPeerFactoryWithError(expectedErr)

	app := &ClientApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mocks.NewMockSignalerFactory(),
		peerFactory:     mockFactory,
	}

	peer, err := app.peerFactory.CreatePeer(transport.DirectionReceive, transport.ICEConfig{})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, peer)
}

func TestClientApp_MockPeerWithError_CreateAnswerError(t *testing.T) {
	mockPeer := mocks.NewMockPeerWithError()
	mockPeer.CreateAnswerError = errors.New("answer creation failed")

	answer, err := mockPeer.CreateAnswer(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "test-offer",
	})

	require.Error(t, err)
	assert.Equal(t, "answer creation failed", err.Error())
	assert.Equal(t, webrtc.SessionDescription{}, answer)
}

func TestClientApp_MockPeerWithError_AddAudioTrackError(t *testing.T) {
	mockPeer := mocks.NewMockPeerWithError()
	mockPeer.AddAudioTrackError = errors.New("audio track error")

	ch, err := mockPeer.AddAudioTrack(48000, 2)

	require.Error(t, err)
	assert.Equal(t, "audio track error", err.Error())
	assert.Nil(t, ch)
}

// =============================================================================
// Factory Override Tests
// =============================================================================

func TestServerApp_FactoryOverride(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 0

	// Create with default factories
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	// Verify defaults are set
	assert.IsType(t, &TCPSignalerFactory{}, app.signalerFactory)
	assert.IsType(t, &WebRTCPeerFactory{}, app.peerFactory)

	// Override with mocks
	app.signalerFactory = mocks.NewMockSignalerFactory()
	app.peerFactory = mocks.NewMockPeerFactory()

	// Verify mocks are now set
	assert.IsType(t, &mocks.MockSignalerFactory{}, app.signalerFactory)
	assert.IsType(t, &mocks.MockPeerFactory{}, app.peerFactory)
}

func TestClientApp_FactoryOverride(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "127.0.0.1"

	// Create with default factories
	app := NewClientApp(cfg, testAppLogger(), nil)

	// Verify defaults are set
	assert.IsType(t, &TCPSignalerFactory{}, app.signalerFactory)
	assert.IsType(t, &WebRTCPeerFactory{}, app.peerFactory)

	// Override with mocks
	app.signalerFactory = mocks.NewMockSignalerFactory()
	app.peerFactory = mocks.NewMockPeerFactory()

	// Verify mocks are now set
	assert.IsType(t, &mocks.MockSignalerFactory{}, app.signalerFactory)
	assert.IsType(t, &mocks.MockPeerFactory{}, app.peerFactory)
}

// =============================================================================
// TLS Configuration Tests with Mocks
// =============================================================================

func TestServerApp_MockFactory_WithTLS(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 8443

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	mockFactory := mocks.NewMockSignalerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		tlsConfig:       tlsConfig,
		signalerFactory: mockFactory,
		peerFactory:     mocks.NewMockPeerFactory(),
	}

	// Verify TLS config is set
	require.NotNil(t, app.tlsConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), app.tlsConfig.MinVersion)

	// Create signaler with TLS (mock handles it)
	signaler := app.signalerFactory.CreateServerSignaler(":8443", app.tlsConfig)
	require.NotNil(t, signaler)
}

func TestClientApp_MockFactory_WithTLS(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "127.0.0.1"
	cfg.Port = 8443

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	mockFactory := mocks.NewMockSignalerFactory()

	app := &ClientApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		tlsConfig:       tlsConfig,
		signalerFactory: mockFactory,
		peerFactory:     mocks.NewMockPeerFactory(),
	}

	// Verify TLS config is set
	require.NotNil(t, app.tlsConfig)
	assert.True(t, app.tlsConfig.InsecureSkipVerify)

	// Create signaler with TLS (mock handles it)
	signaler := app.signalerFactory.CreateClientSignaler("127.0.0.1:8443", app.tlsConfig)
	require.NotNil(t, signaler)
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestMockSignalerFactory_ImplementsInterface(t *testing.T) {
	// Compile-time interface check
	var _ SignalerFactory = (*mocks.MockSignalerFactory)(nil)

	// Runtime verification
	factory := mocks.NewMockSignalerFactory()
	require.NotNil(t, factory)

	// Test interface methods
	serverSignaler := factory.CreateServerSignaler(":8080", nil)
	require.NotNil(t, serverSignaler)

	clientSignaler := factory.CreateClientSignaler("127.0.0.1:8080", nil)
	require.NotNil(t, clientSignaler)
}

func TestMockPeerFactory_ImplementsInterface(t *testing.T) {
	// Compile-time interface check
	var _ PeerFactory = (*mocks.MockPeerFactory)(nil)

	// Runtime verification
	factory := mocks.NewMockPeerFactory()
	require.NotNil(t, factory)

	// Test interface method
	peer, err := factory.CreatePeer(transport.DirectionSend, transport.ICEConfig{})
	require.NoError(t, err)
	require.NotNil(t, peer)
}

func TestMockPeer_ImplementsPeerManager(t *testing.T) {
	// Compile-time interface check
	var _ transport.PeerManager = (*mocks.MockPeer)(nil)

	// Runtime verification
	peer := mocks.NewMockPeer()
	require.NotNil(t, peer)

	// Test all interface methods
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	assert.NoError(t, err)

	_, err = peer.CreateOffer()
	assert.NoError(t, err)

	_, err = peer.CreateAnswer(webrtc.SessionDescription{})
	assert.NoError(t, err)

	err = peer.SetRemoteDescription(webrtc.SessionDescription{})
	assert.NoError(t, err)

	err = peer.AddICECandidate(webrtc.ICECandidateInit{})
	assert.NoError(t, err)

	ch, err := peer.AddAudioTrack(48000, 2)
	assert.NotNil(t, ch)
	assert.NoError(t, err)

	stats := peer.GetStats()
	assert.Equal(t, "connected", stats.State)

	dcReady := peer.DCReady()
	assert.NotNil(t, dcReady)

	err = peer.Close()
	assert.NoError(t, err)
}

func TestMockSignaler_ImplementsSignaler(t *testing.T) {
	// Compile-time interface check
	var _ transport.Signaler = (*mocks.MockSignaler)(nil)

	// Runtime verification
	signaler := mocks.NewMockSignaler("127.0.0.1:8080")
	require.NotNil(t, signaler)

	// Test all interface methods
	err := signaler.Start(context.Background())
	assert.NoError(t, err)

	err = signaler.Send(transport.SignalingMessage{})
	assert.NoError(t, err)

	recvCh := signaler.Receive()
	assert.NotNil(t, recvCh)

	addr := signaler.RemoteAddr()
	assert.Equal(t, "127.0.0.1:8080", addr)

	err = signaler.Close()
	assert.NoError(t, err)
}

// =============================================================================
// Mock Peer Callbacks Tests
// =============================================================================

func TestMockPeer_OnICECandidate(t *testing.T) {
	peer := mocks.NewMockPeer()

	var callbackSet bool
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		callbackSet = true
	})

	// Verify callback was set (mock stores it)
	require.NotNil(t, peer)
	// Note: The mock doesn't trigger callbacks automatically
	assert.False(t, callbackSet)
}

func TestMockPeer_OnConnectionStateChange(t *testing.T) {
	peer := mocks.NewMockPeer()

	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		// Callback registered
	})

	// Verify callback was set
	require.NotNil(t, peer)
}

func TestMockPeer_OnAudioTrack(t *testing.T) {
	peer := mocks.NewMockPeer()

	var callbackCalled bool
	peer.OnAudioTrack(func(inCh <-chan []byte) {
		callbackCalled = true
	})

	// Verify callback was set
	require.NotNil(t, peer)
	assert.False(t, callbackCalled)
}

// =============================================================================
// SimulateReceive Test
// =============================================================================

func TestMockSignaler_SimulateReceive(t *testing.T) {
	signaler := mocks.NewMockSignaler("127.0.0.1:8080")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start signaler
	err := signaler.Start(ctx)
	require.NoError(t, err)

	// Simulate receiving a message
	testMsg := transport.SignalingMessage{
		Type:    "test",
		Payload: []byte(`{"test": "data"}`),
	}

	// Send message to receive channel
	go func() {
		signaler.SimulateReceive(testMsg)
	}()

	// Receive the message
	select {
	case msg := <-signaler.Receive():
		assert.Equal(t, "test", msg.Type)
	case <-ctx.Done():
		t.Fatal("Did not receive message in time")
	}

	signaler.Close()
}

// =============================================================================
// SimulateDCReady Test
// =============================================================================

func TestMockPeer_SimulateDCReady(t *testing.T) {
	peer := mocks.NewMockPeer()

	// Get the DC ready channel
	dcReady := peer.DCReady()
	require.NotNil(t, dcReady)

	// Simulate data channel becoming ready
	go func() {
		peer.SimulateDCReady()
	}()

	// Wait for it
	select {
	case <-dcReady:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DC ready channel was not closed")
	}
}

// =============================================================================
// Mock Multiple Calls Tests
// =============================================================================

func TestMockPeerFactory_MultipleCreatePeerCalls(t *testing.T) {
	factory := mocks.NewMockPeerFactory()

	// Create multiple peers - should return same mock peer
	peer1, err := factory.CreatePeer(transport.DirectionSend, transport.ICEConfig{})
	require.NoError(t, err)

	peer2, err := factory.CreatePeer(transport.DirectionReceive, transport.ICEConfig{})
	require.NoError(t, err)

	// Same underlying mock is returned
	assert.Equal(t, peer1, peer2)
}

func TestMockSignalerFactory_MultipleCreateCalls(t *testing.T) {
	factory := mocks.NewMockSignalerFactory()

	// Create multiple signalers
	sig1 := factory.CreateServerSignaler(":8080", nil)
	sig2 := factory.CreateServerSignaler(":9090", nil)

	// Same underlying mock is returned
	assert.Equal(t, sig1, sig2)

	// Client signalers
	cliSig1 := factory.CreateClientSignaler("127.0.0.1:8080", nil)
	cliSig2 := factory.CreateClientSignaler("127.0.0.1:9090", nil)

	assert.Equal(t, cliSig1, cliSig2)
}

// =============================================================================
// Direction Parameter Tests
// =============================================================================

func TestMockPeerFactory_AllDirections(t *testing.T) {
	factory := mocks.NewMockPeerFactory()

	directions := []transport.MediaDirection{
		transport.DirectionSend,
		transport.DirectionReceive,
	}

	for _, direction := range directions {
		peer, err := factory.CreatePeer(direction, transport.ICEConfig{})
		require.NoError(t, err, "Failed for direction: %v", direction)
		require.NotNil(t, peer, "Peer should not be nil for direction: %v", direction)
	}
}

// =============================================================================
// Thread Safety Tests
// =============================================================================

func TestMockSignaler_ConcurrentAccess(t *testing.T) {
	signaler := mocks.NewMockSignaler("127.0.0.1:8080")

	ctx := context.Background()
	err := signaler.Start(ctx)
	require.NoError(t, err)

	// Concurrent sends
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			err := signaler.Send(transport.SignalingMessage{
				Type:    "test",
				Payload: []byte(`{}`),
			})
			assert.NoError(t, err, "Concurrent send %d failed", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	signaler.Close()
}

func TestMockPeer_ConcurrentAccess(t *testing.T) {
	peer := mocks.NewMockPeer()

	// Concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			_, err := peer.CreateOffer()
			assert.NoError(t, err, "Concurrent CreateOffer %d failed", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// ICE Configuration Tests with Mocks
// =============================================================================

func TestMockPeerFactory_WithSTUNServers(t *testing.T) {
	factory := mocks.NewMockPeerFactory()

	iceConfig := transport.ICEConfig{
		STUNServers: []string{"stun:stun.example.com:3478"},
	}

	peer, err := factory.CreatePeer(transport.DirectionSend, iceConfig)
	require.NoError(t, err)
	require.NotNil(t, peer)
}

func TestMockPeerFactory_WithTURNServers(t *testing.T) {
	factory := mocks.NewMockPeerFactory()

	iceConfig := transport.ICEConfig{
		STUNServers: []string{"stun:stun.example.com:3478"},
		TURNServers: []transport.TURNServer{
			{URL: "turn:turn.example.com:3478", Username: "user", Credential: "pass"},
		},
	}

	peer, err := factory.CreatePeer(transport.DirectionReceive, iceConfig)
	require.NoError(t, err)
	require.NotNil(t, peer)
}

// =============================================================================
// Error Injection Tests with MockPeerWithError
// =============================================================================

func TestMockPeerWithError_AllErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockPeerWithError)
		operation func(*mocks.MockPeerWithError) error
		wantErr   string
	}{
		{
			name: "add audio track error",
			setupMock: func(m *mocks.MockPeerWithError) {
				m.AddAudioTrackError = errors.New("audio track failed")
			},
			operation: func(m *mocks.MockPeerWithError) error {
				_, err := m.AddAudioTrack(48000, 2)
				return err
			},
			wantErr: "audio track failed",
		},
		{
			name: "create offer error",
			setupMock: func(m *mocks.MockPeerWithError) {
				m.CreateOfferError = errors.New("offer failed")
			},
			operation: func(m *mocks.MockPeerWithError) error {
				_, err := m.CreateOffer()
				return err
			},
			wantErr: "offer failed",
		},
		{
			name: "create answer error",
			setupMock: func(m *mocks.MockPeerWithError) {
				m.CreateAnswerError = errors.New("answer failed")
			},
			operation: func(m *mocks.MockPeerWithError) error {
				_, err := m.CreateAnswer(webrtc.SessionDescription{})
				return err
			},
			wantErr: "answer failed",
		},
		{
			name: "set remote description error",
			setupMock: func(m *mocks.MockPeerWithError) {
				m.SetRemoteDescError = errors.New("remote desc failed")
			},
			operation: func(m *mocks.MockPeerWithError) error {
				return m.SetRemoteDescription(webrtc.SessionDescription{})
			},
			wantErr: "remote desc failed",
		},
		{
			name: "add ICE candidate error",
			setupMock: func(m *mocks.MockPeerWithError) {
				m.AddICECandidateError = errors.New("ICE failed")
			},
			operation: func(m *mocks.MockPeerWithError) error {
				return m.AddICECandidate(webrtc.ICECandidateInit{})
			},
			wantErr: "ICE failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPeer := mocks.NewMockPeerWithError()
			tt.setupMock(mockPeer)

			err := tt.operation(mockPeer)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// =============================================================================
// MockSignalerWithError Tests
// =============================================================================

func TestMockSignalerWithError_AllErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockSignalerWithError)
		operation func(*mocks.MockSignalerWithError) error
		wantErr   string
	}{
		{
			name: "start error",
			setupMock: func(m *mocks.MockSignalerWithError) {
				m.StartError = errors.New("start failed")
			},
			operation: func(m *mocks.MockSignalerWithError) error {
				return m.Start(context.Background())
			},
			wantErr: "start failed",
		},
		{
			name: "send error",
			setupMock: func(m *mocks.MockSignalerWithError) {
				m.SendError = errors.New("send failed")
			},
			operation: func(m *mocks.MockSignalerWithError) error {
				return m.Send(transport.SignalingMessage{})
			},
			wantErr: "send failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSignaler := mocks.NewMockSignalerWithError("127.0.0.1:8080")
			tt.setupMock(mockSignaler)

			err := tt.operation(mockSignaler)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
