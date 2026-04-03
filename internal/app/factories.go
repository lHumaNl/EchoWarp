package app

import (
	"crypto/tls"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// TCPSignalerFactory implements SignalerFactory using TCP signaling.
type TCPSignalerFactory struct{}

// Ensure TCPSignalerFactory implements SignalerFactory interface.
var _ SignalerFactory = (*TCPSignalerFactory)(nil)

// NewTCPSignalerFactory creates a new TCP signaler factory.
func NewTCPSignalerFactory() *TCPSignalerFactory {
	return &TCPSignalerFactory{}
}

// CreateServerSignaler creates a TCP signaler for server mode.
func (f *TCPSignalerFactory) CreateServerSignaler(address string, tlsConfig *tls.Config) transport.Signaler {
	if tlsConfig != nil {
		return transport.NewTCPSignaler(transport.RoleServer, address, transport.WithTLS(tlsConfig))
	}
	return transport.NewTCPSignaler(transport.RoleServer, address)
}

// CreateClientSignaler creates a TCP signaler for client mode.
func (f *TCPSignalerFactory) CreateClientSignaler(address string, tlsConfig *tls.Config) transport.Signaler {
	if tlsConfig != nil {
		return transport.NewTCPSignaler(transport.RoleClient, address, transport.WithTLS(tlsConfig))
	}
	return transport.NewTCPSignaler(transport.RoleClient, address)
}

// WebRTCPeerFactory implements PeerFactory using WebRTC.
type WebRTCPeerFactory struct{}

// Ensure WebRTCPeerFactory implements PeerFactory interface.
var _ PeerFactory = (*WebRTCPeerFactory)(nil)

// NewWebRTCPeerFactory creates a new WebRTC peer factory.
func NewWebRTCPeerFactory() *WebRTCPeerFactory {
	return &WebRTCPeerFactory{}
}

// CreatePeer creates a new WebRTC peer connection.
// For receive or duplex directions, a recvonly audio transceiver is added automatically
// so the SDP offer/answer includes audio media for incoming tracks.
func (f *WebRTCPeerFactory) CreatePeer(direction transport.MediaDirection, iceConfig transport.ICEConfig) (transport.PeerManager, error) {
	peer := transport.NewWebRTCPeer(direction)
	if err := peer.CreatePeerConnection(iceConfig); err != nil {
		return nil, err
	}
	if direction == transport.DirectionReceive || direction == transport.DirectionDuplex {
		if err := peer.AddAudioTransceiver(); err != nil {
			return nil, err
		}
	}
	return peer, nil
}
