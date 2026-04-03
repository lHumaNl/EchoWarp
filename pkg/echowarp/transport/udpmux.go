package transport

import (
	"fmt"
	"net"
	"sync"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v4"
)

// UDPMuxManager manages a shared ICE UDP multiplexer that allows all WebRTC
// PeerConnections to share a single UDP port. This simplifies firewall rules:
// only one UDP port needs to be opened regardless of the number of clients.
type UDPMuxManager struct {
	mu     sync.Mutex
	conn   *net.UDPConn
	mux    ice.UDPMux
	api    *webrtc.API
	port   int
	closed bool
}

// NewUDPMuxManager creates a UDPMuxManager that listens on the specified UDP port.
// All PeerConnections created via WebRTCAPI() will share this single port.
func NewUDPMuxManager(port int) (*UDPMuxManager, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		return nil, fmt.Errorf("failed to listen UDP on port %d: %w", port, err)
	}

	logFactory := logging.NewDefaultLoggerFactory()
	logFactory.DefaultLogLevel = logging.LogLevelDisabled
	logger := logFactory.NewLogger("ice-udp-mux")

	mux := webrtc.NewICEUDPMux(logger, conn)

	se := webrtc.SettingEngine{}
	se.SetICEUDPMux(mux)

	api := webrtc.NewAPI(webrtc.WithSettingEngine(se))

	// Resolve actual port (may differ from requested when port=0).
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("udpmux: unexpected local address type %T", conn.LocalAddr())
	}
	actualPort := udpAddr.Port

	return &UDPMuxManager{
		conn: conn,
		mux:  mux,
		api:  api,
		port: actualPort,
	}, nil
}

// WebRTCAPI returns the webrtc.API configured with the shared UDPMux.
// Use api.NewPeerConnection(config) instead of webrtc.NewPeerConnection(config).
func (m *UDPMuxManager) WebRTCAPI() *webrtc.API {
	return m.api
}

// Port returns the UDP port the mux is listening on.
func (m *UDPMuxManager) Port() int {
	return m.port
}

// Close releases the UDP socket and mux resources.
func (m *UDPMuxManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	// Close mux first (it uses the conn internally).
	muxErr := m.mux.Close()
	// conn.Close may return "use of closed" if mux already closed it — ignore.
	_ = m.conn.Close()
	return muxErr
}
