// Package app provides application-level orchestration for EchoWarp.
// This file defines minimal interfaces to reduce coupling between the app layer
// and concrete implementations in pkg/echowarp/transport, following the
// Interface Segregation Principle.
package app

import (
	"crypto/tls"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// SignalerFactory creates signaling channel instances for server or client modes.
// This interface abstracts the creation of TCP signalers, allowing the app layer
// to work with any signaling implementation.
//
// Purpose: Decouple app layer from concrete transport.TCPSignaler type,
// enabling easier testing and future alternative signaling implementations.
type SignalerFactory interface {
	// CreateServerSignaler creates a signaler for server mode (listens on address).
	// The address should be in format ":port" or "host:port".
	// TLS config is optional; if nil, plain TCP is used.
	CreateServerSignaler(address string, tlsConfig *tls.Config) transport.Signaler

	// CreateClientSignaler creates a signaler for client mode (connects to address).
	// The address should be in format "host:port".
	// TLS config is optional; if nil, plain TCP is used.
	CreateClientSignaler(address string, tlsConfig *tls.Config) transport.Signaler
}

// PeerFactory creates WebRTC peer connection instances.
// This interface abstracts the creation of peer connections, allowing the app layer
// to work with any peer implementation.
//
// Purpose: Decouple app layer from concrete transport.WebRTCPeer type,
// enabling easier testing and supporting alternative WebRTC implementations.
type PeerFactory interface {
	// CreatePeer creates a new WebRTC peer connection with the given media direction
	// and ICE configuration.
	//
	// direction: Send (capture and stream) or Receive (playback)
	// iceConfig: STUN/TURN server configuration for NAT traversal
	//
	// Returns a PeerManager interface that provides all peer operations.
	CreatePeer(direction transport.MediaDirection, iceConfig transport.ICEConfig) (transport.PeerManager, error)
}
