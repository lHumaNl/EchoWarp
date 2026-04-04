// Package mocks provides mock implementations for testing the app layer.
// These mocks demonstrate how interfaces enable easy testing without
// depending on concrete implementations from pkg/echowarp/transport.
package mocks

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/pion/webrtc/v4"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// MockSignaler is a mock implementation of transport.Signaler for testing.
// Thread-safe: All methods are safe for concurrent use.
type MockSignaler struct {
	mu         sync.Mutex
	started    bool
	remoteAddr string
	sendCh     chan transport.SignalingMessage
	recvCh     chan transport.SignalingMessage
	closed     bool
}

// NewMockSignaler creates a new mock signaler.
func NewMockSignaler(remoteAddr string) *MockSignaler {
	return &MockSignaler{
		remoteAddr: remoteAddr,
		sendCh:     make(chan transport.SignalingMessage, 10),
		recvCh:     make(chan transport.SignalingMessage, 10),
	}
}

// Start simulates starting the signaler.
func (m *MockSignaler) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return nil
}

// Send sends a message through the mock signaler.
func (m *MockSignaler) Send(msg transport.SignalingMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.sendCh <- msg
	return nil
}

// Receive returns the receive channel for mock messages.
func (m *MockSignaler) Receive() <-chan transport.SignalingMessage {
	return m.recvCh
}

// RemoteAddr returns the mock remote address.
func (m *MockSignaler) RemoteAddr() string {
	return m.remoteAddr
}

// Close closes the mock signaler.
func (m *MockSignaler) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.recvCh)
	return nil
}

// Ready returns a closed channel indicating the signaler is immediately ready.
func (m *MockSignaler) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// SimulateReceive allows test code to inject messages into the receive channel.
func (m *MockSignaler) SimulateReceive(msg transport.SignalingMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.recvCh <- msg
	}
}

// MockPeer is a mock implementation of transport.PeerManager for testing.
// Thread-safe: All methods are safe for concurrent use.
type MockPeer struct {
	mu              sync.Mutex
	closed          bool
	dcReady         chan struct{}
	audioTrackCB    func(inCh <-chan []byte)
	iceCandidateCB  func(candidate *webrtc.ICECandidate)
	connectionState func(state webrtc.PeerConnectionState)
}

// NewMockPeer creates a new mock peer.
func NewMockPeer() *MockPeer {
	return &MockPeer{
		dcReady: make(chan struct{}),
	}
}

// CreatePeerConnection simulates creating a peer connection.
func (m *MockPeer) CreatePeerConnection(_ transport.ICEConfig) error {
	return nil
}

// Close closes the mock peer.
func (m *MockPeer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	close(m.dcReady)
	return nil
}

// IsClosed returns whether the peer has been closed.
func (m *MockPeer) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// OnConnectionStateChange sets the connection state callback.
func (m *MockPeer) OnConnectionStateChange(handler func(state webrtc.PeerConnectionState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionState = handler
}

// GetStats returns mock connection statistics.
func (m *MockPeer) GetStats() transport.ConnectionStats {
	return transport.ConnectionStats{State: "connected"}
}

// AddAudioTrack simulates adding an audio track and returns a send channel.
func (m *MockPeer) AddAudioTrack(_, _ uint32) (chan<- []byte, error) {
	return make(chan []byte, 10), nil
}

// OnAudioTrack sets the audio track callback.
func (m *MockPeer) OnAudioTrack(handler func(inCh <-chan []byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audioTrackCB = handler
}

// CreateOffer creates a mock SDP offer.
func (m *MockPeer) CreateOffer() (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "mock-offer-sdp",
	}, nil
}

// CreateAnswer creates a mock SDP answer.
func (m *MockPeer) CreateAnswer(_ webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  "mock-answer-sdp",
	}, nil
}

// SetRemoteDescription sets the remote SDP (mock implementation).
func (m *MockPeer) SetRemoteDescription(_ webrtc.SessionDescription) error {
	return nil
}

// AddICECandidate adds an ICE candidate (mock implementation).
func (m *MockPeer) AddICECandidate(_ webrtc.ICECandidateInit) error {
	return nil
}

// OnICECandidate sets the ICE candidate callback.
func (m *MockPeer) OnICECandidate(handler func(candidate *webrtc.ICECandidate)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.iceCandidateCB = handler
}

// CreateDataChannel creates a mock data channel.
func (m *MockPeer) CreateDataChannel(_ string) error {
	return nil
}

// OnDataChannel sets the data channel callback (not implemented in mock).
func (m *MockPeer) OnDataChannel(_ func(label string, msgCh <-chan []byte, sendFn func([]byte) error)) {
	// Not implemented in mock
}

// CreateControlDataChannel creates the control data channel.
func (m *MockPeer) CreateControlDataChannel() error {
	return nil
}

// DCReady returns a channel that closes when data channel is ready.
func (m *MockPeer) DCReady() <-chan struct{} {
	return m.dcReady
}

// SendControl sends a control message (mock implementation).
func (m *MockPeer) SendControl(_ string, _ interface{}) error {
	return nil
}
func (m *MockPeer) ControlMessages() <-chan []byte { return nil }
func (m *MockPeer) CreateChatDataChannel() error   { return nil }
func (m *MockPeer) SendChat([]byte) error          { return nil }
func (m *MockPeer) ChatMessages() <-chan []byte    { return nil }

// SimulateDCReady simulates the data channel becoming ready.
func (m *MockPeer) SimulateDCReady() {
	close(m.dcReady)
}

// MockSignalerFactory is a mock implementation of SignalerFactory for testing.
type MockSignalerFactory struct {
	ServerSignaler *MockSignaler
	ClientSignaler *MockSignaler
}

// NewMockSignalerFactory creates a new mock signaler factory.
func NewMockSignalerFactory() *MockSignalerFactory {
	return &MockSignalerFactory{
		ServerSignaler: NewMockSignaler("127.0.0.1:8080"),
		ClientSignaler: NewMockSignaler("127.0.0.1:9090"),
	}
}

// CreateServerSignaler creates a mock server signaler.
func (f *MockSignalerFactory) CreateServerSignaler(_ string, _ *tls.Config) transport.Signaler {
	return f.ServerSignaler
}

// CreateClientSignaler creates a mock client signaler.
func (f *MockSignalerFactory) CreateClientSignaler(_ string, _ *tls.Config) transport.Signaler {
	return f.ClientSignaler
}

// MockPeerFactory is a mock implementation of PeerFactory for testing.
type MockPeerFactory struct {
	Peer *MockPeer
	// CreatePeerError when set, CreatePeer returns this error
	CreatePeerError error
}

// NewMockPeerFactory creates a new mock peer factory.
func NewMockPeerFactory() *MockPeerFactory {
	return &MockPeerFactory{
		Peer: NewMockPeer(),
	}
}

// NewMockPeerFactoryWithError creates a mock peer factory that returns errors.
func NewMockPeerFactoryWithError(err error) *MockPeerFactory {
	return &MockPeerFactory{
		Peer:            NewMockPeer(),
		CreatePeerError: err,
	}
}

// CreatePeer creates a mock peer.
func (f *MockPeerFactory) CreatePeer(_ transport.MediaDirection, _ transport.ICEConfig) (transport.PeerManager, error) {
	if f.CreatePeerError != nil {
		return nil, f.CreatePeerError
	}
	return f.Peer, nil
}

// MockPeerWithError is a mock peer that can be configured to return errors.
type MockPeerWithError struct {
	*MockPeer
	// AddAudioTrackError when set, AddAudioTrack returns this error
	AddAudioTrackError error
	// CreateOfferError when set, CreateOffer returns this error
	CreateOfferError error
	// CreateAnswerError when set, CreateAnswer returns this error
	CreateAnswerError error
	// SetRemoteDescError when set, SetRemoteDescription returns this error
	SetRemoteDescError error
	// AddICECandidateError when set, AddICECandidate returns this error
	AddICECandidateError error
}

// NewMockPeerWithError creates a mock peer with error injection capability.
func NewMockPeerWithError() *MockPeerWithError {
	return &MockPeerWithError{
		MockPeer: NewMockPeer(),
	}
}

// AddAudioTrack overrides to return error if configured.
func (m *MockPeerWithError) AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error) {
	if m.AddAudioTrackError != nil {
		return nil, m.AddAudioTrackError
	}
	return m.MockPeer.AddAudioTrack(sampleRate, channels)
}

// CreateOffer overrides to return error if configured.
func (m *MockPeerWithError) CreateOffer() (webrtc.SessionDescription, error) {
	if m.CreateOfferError != nil {
		return webrtc.SessionDescription{}, m.CreateOfferError
	}
	return m.MockPeer.CreateOffer()
}

// CreateAnswer overrides to return error if configured.
func (m *MockPeerWithError) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	if m.CreateAnswerError != nil {
		return webrtc.SessionDescription{}, m.CreateAnswerError
	}
	return m.MockPeer.CreateAnswer(offer)
}

// SetRemoteDescription overrides to return error if configured.
func (m *MockPeerWithError) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if m.SetRemoteDescError != nil {
		return m.SetRemoteDescError
	}
	return m.MockPeer.SetRemoteDescription(sdp)
}

// AddICECandidate overrides to return error if configured.
func (m *MockPeerWithError) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	if m.AddICECandidateError != nil {
		return m.AddICECandidateError
	}
	return m.MockPeer.AddICECandidate(candidate)
}

// MockSignalerWithError is a mock signaler that can be configured to return errors.
type MockSignalerWithError struct {
	*MockSignaler
	// StartError when set, Start returns this error
	StartError error
	// SendError when set, Send returns this error
	SendError error
}

// NewMockSignalerWithError creates a mock signaler with error injection capability.
func NewMockSignalerWithError(remoteAddr string) *MockSignalerWithError {
	return &MockSignalerWithError{
		MockSignaler: NewMockSignaler(remoteAddr),
	}
}

// Start overrides to return error if configured.
func (m *MockSignalerWithError) Start(ctx context.Context) error {
	if m.StartError != nil {
		return m.StartError
	}
	return m.MockSignaler.Start(ctx)
}

// Send overrides to return error if configured.
func (m *MockSignalerWithError) Send(msg transport.SignalingMessage) error {
	if m.SendError != nil {
		return m.SendError
	}
	return m.MockSignaler.Send(msg)
}
