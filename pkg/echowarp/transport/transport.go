package transport

import (
	"context"
	"encoding/json"

	"github.com/pion/webrtc/v4"
)

// ConnectionStats contains WebRTC peer connection statistics.
type ConnectionStats struct {
	State       string
	LocalAddr   string
	RemoteAddr  string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsLost uint32
	Jitter      float64
	RoundTrip   float64
}

// ClientInfo holds per-client statistics for multi-client mode.
type ClientInfo struct {
	ClientID      string
	Nickname      string // display name (may differ from ClientID)
	RemoteAddr    string
	JoinedAt      string // formatted time
	Duration      string // formatted duration
	BytesSent     uint64
	BytesRecv     uint64
	Jitter        float64
	RoundTrip     float64 // RTT in milliseconds
	PacketsLost   uint32
	State         string // WebRTC connection state
	Muted         bool   // client has muted the server's audio
	MutedOutgoing bool   // server-initiated: server stopped sending audio to this client
	MutedIncoming bool   // server-initiated: server stopped receiving audio from this client
	Paused        bool   // client has paused its capture
	HWID          string // hardware identifier (empty if not collected)
}

// MultiClientStats aggregates statistics across all connected clients.
type MultiClientStats struct {
	Clients    []ClientInfo
	MaxClients int
}

// TURNServer represents a TURN server configuration for NAT traversal.
type TURNServer struct {
	URL        string `yaml:"url"        json:"url"`        // TURN server URL (e.g., "turn:server.com:3478").
	Username   string `yaml:"username"   json:"username"`   // TURN username.
	Credential string `yaml:"credential" json:"credential"` // TURN password (redacted in SafeConfig).
}

// SignalingRole indicates whether the endpoint acts as server or client.
type SignalingRole int

const (
	RoleServer SignalingRole = iota
	RoleClient
)

// MediaDirection specifies the direction of media flow (send or receive).
type MediaDirection int

const (
	DirectionSend MediaDirection = iota
	DirectionReceive
	DirectionDuplex
)

// SignalingMessage represents a JSON-encoded message exchanged over the signaling channel.
type SignalingMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

const (
	TypeControl              = "control"
	TypeDCReady              = "dc_ready"
	ActionDCReady            = "dc_ready"
	ActionStop               = "stop"
	ActionPeerMute           = "peer_mute"
	ActionPeerUnmute         = "peer_unmute"
	ActionPause              = "pause"
	ActionResume             = "resume"
	ActionPauseAll           = "pause_all"
	ActionResumeAll          = "resume_all"
	ActionParticipantPause   = "participant_pause"
	ActionParticipantsUpdate = "participants_update"
	ActionMuteOutgoing       = "mute_outgoing"
	ActionUnmuteOutgoing     = "unmute_outgoing"
	ActionMuteIncoming       = "mute_incoming"
	ActionUnmuteIncoming     = "unmute_incoming"
	ActionKickNotify         = "kick_notify"
	ActionBanNotify          = "ban_notify"
	LabelControl             = "control"
	LabelChat                = "chat"
)

// ICEConfig holds STUN and TURN server configuration for NAT traversal.
type ICEConfig struct {
	STUNServers []string
	TURNServers []TURNServer

	// UDPMux is an optional shared UDP multiplexer that constrains all ICE
	// traffic to a single UDP port. When set, CreatePeerConnection uses this
	// API instead of the default webrtc.NewPeerConnection.
	UDPMux *UDPMuxManager
}

// Signaler defines the interface for the signaling channel used to exchange
// SDP offers/answers and ICE candidates before WebRTC connection establishment.
type Signaler interface {
	Start(ctx context.Context) error
	Send(msg SignalingMessage) error
	Receive() <-chan SignalingMessage
	RemoteAddr() string
	Ready() <-chan struct{}
	Close() error
}

// PeerConnection manages the WebRTC peer connection lifecycle.
type PeerConnection interface {
	// CreatePeerConnection initializes a new WebRTC PeerConnection with the given ICE configuration.
	CreatePeerConnection(iceConfig ICEConfig) error

	// Close terminates the peer connection and releases resources.
	Close() error

	// OnConnectionStateChange registers a callback for connection state changes.
	OnConnectionStateChange(handler func(state webrtc.PeerConnectionState))

	// GetStats returns current connection statistics.
	GetStats() ConnectionStats
}

// MediaManager handles audio track creation and handling.
type MediaManager interface {
	// AddAudioTrack creates an audio track for sending audio data.
	// Returns a channel for writing Opus-encoded audio frames.
	AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error)

	// OnAudioTrack registers a callback for receiving incoming audio tracks.
	OnAudioTrack(handler func(inCh <-chan []byte))
}

// SignalingHandler manages SDP/ICE signaling exchange.
type SignalingHandler interface {
	// CreateOffer creates an SDP offer for the peer connection.
	CreateOffer() (webrtc.SessionDescription, error)

	// CreateAnswer creates an SDP answer in response to an offer.
	CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error)

	// SetRemoteDescription sets the remote peer's SDP.
	SetRemoteDescription(sdp webrtc.SessionDescription) error

	// AddICECandidate adds a remote ICE candidate.
	AddICECandidate(candidate webrtc.ICECandidateInit) error

	// OnICECandidate registers a callback for local ICE candidates.
	OnICECandidate(handler func(candidate *webrtc.ICECandidate))
}

// DataChannelManager handles WebRTC data channel operations.
type DataChannelManager interface {
	// CreateDataChannel creates a new data channel with the specified label.
	CreateDataChannel(label string) error

	// OnDataChannel registers a callback for incoming data channels.
	OnDataChannel(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error))
}

// DataChannelControl manages the control data channel used for post-handshake communication.
type DataChannelControl interface {
	// CreateControlDataChannel creates the control data channel.
	CreateControlDataChannel() error

	// DCReady returns a channel that closes when the dc_ready message is received.
	DCReady() <-chan struct{}

	// SendControl sends a control message through the data channel.
	SendControl(action string, payload interface{}) error

	// ControlMessages returns a channel that receives raw control messages from the peer.
	ControlMessages() <-chan []byte
}

// DataChannelChat manages the chat data channel used for text messaging between peers.
type DataChannelChat interface {
	// CreateChatDataChannel creates the chat data channel.
	CreateChatDataChannel() error

	// SendChat sends raw bytes through the chat data channel.
	SendChat(data []byte) error

	// ChatMessages returns a channel that receives raw chat messages from the peer.
	ChatMessages() <-chan []byte
}

// PeerManager combines all specialized interfaces for complete peer connection management.
// This interface follows the Interface Segregation Principle by composing smaller interfaces.
type PeerManager interface {
	PeerConnection
	MediaManager
	SignalingHandler
	DataChannelManager
	DataChannelControl
	DataChannelChat
}

// MultiPeerManager manages multiple peer connections for multi-client scenarios.
// Thread-safe: All methods are safe for concurrent use.
type MultiPeerManager interface {
	AddPeer(clientID string, iceConfig ICEConfig) (PeerManager, error)
	RemovePeer(clientID string) error
	Peers() []string
	PeerCount() int
	Close() error
}
