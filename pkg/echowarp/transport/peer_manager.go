package transport

import (
	"errors"
	"sort"
	"sync"

	"github.com/pion/webrtc/v4"

	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

var (
	ErrPeerExists   = errors.New("peer with this client ID already exists")
	ErrMaxPeers     = errors.New("maximum number of peers reached")
	ErrPeerNotFound = errors.New("peer not found")
)

// WebRTCMultiPeerManager manages multiple WebRTC peer connections for multi-client scenarios.
// It provides thread-safe operations for adding, removing, and tracking peers.
type WebRTCMultiPeerManager struct {
	mu          sync.RWMutex
	peers       map[string]*WebRTCPeer
	direction   MediaDirection
	maxPeers    int
	activeCount int
}

var _ MultiPeerManager = (*WebRTCMultiPeerManager)(nil)

// NewMultiPeerManager creates a new multi-peer manager.
// maxPeers of 0 means unlimited peers.
func NewMultiPeerManager(direction MediaDirection, maxPeers int) *WebRTCMultiPeerManager {
	return &WebRTCMultiPeerManager{
		peers:     make(map[string]*WebRTCPeer),
		direction: direction,
		maxPeers:  maxPeers,
	}
}

// AddPeer creates and adds a new peer connection for the given client ID.
// Returns ErrPeerExists if the client ID already exists.
// Returns ErrMaxPeers if the maximum number of peers has been reached.
func (m *WebRTCMultiPeerManager) AddPeer(clientID string, iceConfig ICEConfig) (PeerManager, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.maxPeers > 0 && len(m.peers) >= m.maxPeers {
		return nil, ErrMaxPeers
	}

	if _, exists := m.peers[clientID]; exists {
		return nil, ErrPeerExists
	}

	peer := NewWebRTCPeer(m.direction)
	if err := peer.CreatePeerConnection(iceConfig); err != nil {
		return nil, err
	}

	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.mu.Lock()
		switch state {
		case webrtc.PeerConnectionStateConnected:
			m.activeCount++
			metrics.ConnectionPoolActive.Inc()
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			if m.activeCount > 0 {
				m.activeCount--
				metrics.ConnectionPoolActive.Dec()
			}
		}
		m.mu.Unlock()
	})

	m.peers[clientID] = peer
	metrics.ConnectionPoolSize.Inc()
	return peer, nil
}

// RemovePeer closes and removes the peer connection for the given client ID.
// Returns ErrPeerNotFound if the client ID doesn't exist.
func (m *WebRTCMultiPeerManager) RemovePeer(clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peer, exists := m.peers[clientID]
	if !exists {
		return ErrPeerNotFound
	}

	stats := peer.GetStats()
	if stats.State == webrtc.PeerConnectionStateConnected.String() {
		m.activeCount--
		metrics.ConnectionPoolActive.Dec()
	}

	err := peer.Close()
	delete(m.peers, clientID)
	metrics.ConnectionPoolSize.Dec()
	return err
}

// Peers returns a sorted list of all client IDs.
func (m *WebRTCMultiPeerManager) Peers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.peers))
	for id := range m.peers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// PeerCount returns the current number of active peers.
func (m *WebRTCMultiPeerManager) PeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

// Close terminates all peer connections and releases resources.
// Returns the last error encountered during cleanup.
func (m *WebRTCMultiPeerManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	peerCount := len(m.peers)
	for id, peer := range m.peers {
		if err := peer.Close(); err != nil {
			lastErr = err
		}
		delete(m.peers, id)
	}

	metrics.ConnectionPoolSize.Sub(float64(peerCount))
	metrics.ConnectionPoolActive.Sub(float64(m.activeCount))
	m.activeCount = 0
	return lastErr
}
