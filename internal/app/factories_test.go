package app

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/app/mocks"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// TestServerApp_WithMockFactories demonstrates how the new interfaces
// enable testing without depending on concrete transport implementations.
func TestServerApp_WithMockFactories(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 0

	// Create app with default factories (real implementations)
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	require.NotNil(t, app)

	// Verify default factories are concrete types
	assert.IsType(t, &TCPSignalerFactory{}, app.signalerFactory, "default signaler factory must be TCP")
	assert.IsType(t, &WebRTCPeerFactory{}, app.peerFactory, "default peer factory must be WebRTC")
}

// TestClientApp_WithMockFactories demonstrates mock-based testing.
func TestClientApp_WithMockFactories(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "127.0.0.1"

	// Create app with default factories
	app := NewClientApp(cfg, testAppLogger(), nil)
	require.NotNil(t, app)

	// Verify default factories are concrete types
	assert.IsType(t, &TCPSignalerFactory{}, app.signalerFactory, "default signaler factory must be TCP")
	assert.IsType(t, &WebRTCPeerFactory{}, app.peerFactory, "default peer factory must be WebRTC")
}

// TestServerApp_UsingMocksForTesting demonstrates how to inject mocks
// for isolated unit testing without network dependencies.
func TestServerApp_UsingMocksForTesting(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 8080

	// Create app with mock factories for testing
	mockSignalerFactory := mocks.NewMockSignalerFactory()
	mockPeerFactory := mocks.NewMockPeerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mockSignalerFactory,
		peerFactory:     mockPeerFactory,
	}

	// Test signaler factory
	signaler := app.signalerFactory.CreateServerSignaler(":8080", nil)
	require.NotNil(t, signaler)

	// Verify mock returns expected remote address
	assert.Equal(t, "127.0.0.1:8080", signaler.RemoteAddr())

	// Test peer factory
	peer, err := mockPeerFactory.CreatePeer(
		transport.DirectionSend,
		transport.ICEConfig{},
	)
	require.NoError(t, err)
	require.NotNil(t, peer)

	// Test peer operations
	stats := peer.GetStats()
	assert.Equal(t, "connected", stats.State)
}

// TestClientApp_UsingMocksForTesting demonstrates mock injection for client testing.
func TestClientApp_UsingMocksForTesting(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "192.168.1.1"
	cfg.Port = 8080

	// Create app with mock factories
	mockSignalerFactory := mocks.NewMockSignalerFactory()
	mockPeerFactory := mocks.NewMockPeerFactory()

	app := &ClientApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mockSignalerFactory,
		peerFactory:     mockPeerFactory,
	}

	// Test client signaler creation
	signaler := app.signalerFactory.CreateClientSignaler("192.168.1.1:8080", nil)
	require.NotNil(t, signaler)

	// Test peer creation
	peer, err := app.peerFactory.CreatePeer(
		transport.DirectionReceive,
		transport.ICEConfig{},
	)
	require.NoError(t, err)
	require.NotNil(t, peer)

	// Verify mock peer is ready to use
	dcReady := peer.DCReady()
	require.NotNil(t, dcReady)
}

// TestServerApp_FactoryInjectionWithTLS demonstrates factory injection with TLS.
func TestServerApp_FactoryInjectionWithTLS(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 8443

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	mockSignalerFactory := mocks.NewMockSignalerFactory()
	mockPeerFactory := mocks.NewMockPeerFactory()

	app := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		tlsConfig:       tlsConfig,
		signalerFactory: mockSignalerFactory,
		peerFactory:     mockPeerFactory,
	}

	// Verify TLS config is properly set
	require.NotNil(t, app.tlsConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), app.tlsConfig.MinVersion)

	// Create signaler with TLS (mock handles it)
	signaler := app.signalerFactory.CreateServerSignaler(":8443", app.tlsConfig)
	require.NotNil(t, signaler)
}

// TestInterfaceSegregation verifies that our interfaces follow ISP.
func TestInterfaceSegregation(t *testing.T) {
	t.Parallel()
	// SignalerFactory should have only 2 methods (small interface)
	var _ SignalerFactory = (*mocks.MockSignalerFactory)(nil)

	// PeerFactory should have only 1 method (minimal interface)
	var _ PeerFactory = (*mocks.MockPeerFactory)(nil)

	// Mock peer implements full PeerManager interface
	var _ transport.PeerManager = (*mocks.MockPeer)(nil)

	// Mock signaler implements Signaler interface
	var _ transport.Signaler = (*mocks.MockSignaler)(nil)
}

// TestFactoriesEnableTesting demonstrates that factories enable
// replacing real implementations with mocks for testing.
func TestFactoriesEnableTesting(t *testing.T) {
	t.Parallel()
	// This test demonstrates the key benefit: we can test app logic
	// without requiring actual network connections or WebRTC setup.

	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer

	// Production code uses real factories
	prodApp := NewServerApp(cfg, testAppLogger(), nil, nil, nil)
	assert.IsType(t, &TCPSignalerFactory{}, prodApp.signalerFactory)
	assert.IsType(t, &WebRTCPeerFactory{}, prodApp.peerFactory)

	// Test code uses mocks
	testApp := &ServerApp{
		cfg:             cfg,
		logger:          testAppLogger(),
		signalerFactory: mocks.NewMockSignalerFactory(),
		peerFactory:     mocks.NewMockPeerFactory(),
	}
	assert.IsType(t, &mocks.MockSignalerFactory{}, testApp.signalerFactory)
	assert.IsType(t, &mocks.MockPeerFactory{}, testApp.peerFactory)
}
