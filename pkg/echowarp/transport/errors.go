package transport

import (
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Sentinel errors for backward compatibility with errors.Is().
// These are preserved to avoid breaking existing error handling code.
// New code should use the structured error constructors below.
var (
	// ErrConnectionFailed is returned when WebRTC connection cannot be established.
	//
	// Error code: E205
	// Deprecated: Use NewConnectionFailedError() for structured errors.
	ErrConnectionFailed = ewerrors.NewSentinel(ewerrors.ErrConnectionFailed, "connection failed")

	// ErrSignalingClosed is returned when the signaling connection is closed unexpectedly.
	//
	// Error code: E201
	// Deprecated: Use NewSignalingClosedError() for structured errors.
	ErrSignalingClosed = ewerrors.NewSentinel(ewerrors.ErrSignalingClosed, "signaling connection closed")

	// ErrICEFailed is returned when ICE connection negotiation fails.
	//
	// Error code: E202
	// Deprecated: Use NewICEFailedError() for structured errors.
	ErrICEFailed = ewerrors.NewSentinel(ewerrors.ErrICEFailed, "ICE connection failed")

	// ErrPeerClosed is returned when the peer connection is closed by the remote party.
	//
	// Error code: E204
	// Deprecated: Use NewPeerDisconnectedError() for structured errors.
	ErrPeerClosed = ewerrors.NewSentinel(ewerrors.ErrPeerDisconnected, "peer connection closed")
)

// NewConnectionFailedError creates a structured error for WebRTC connection failures.
// Includes context about the connection state and underlying cause.
//
// Example:
//
//	err := transport.NewConnectionFailedError("connecting", underlyingError)
func NewConnectionFailedError(state string, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrConnectionFailed, "WebRTC connection failed").
		WithContext("connection_state", state).
		WithSuggestion("Check network configuration, NAT traversal settings, and peer availability.")
}

// NewSignalingClosedError creates a structured error for signaling connection closure.
// Includes context about the closure reason if available.
//
// Example:
//
//	err := transport.NewSignalingClosedError("server shutdown")
func NewSignalingClosedError(reason string) *ewerrors.EchoWarpError {
	err := ewerrors.NewError(ewerrors.ErrSignalingClosed, "signaling connection closed")
	if reason != "" {
		err = err.WithContext("reason", reason)
	}
	return err.WithSuggestion("The signaling server may have terminated the connection. Try reconnecting.")
}

// NewICEFailedError creates a structured error for ICE connection failures.
// Includes context about ICE candidates and connection attempts.
//
// Example:
//
//	err := transport.NewICEFailedError(5, 0, underlyingError)
func NewICEFailedError(candidatesReceived int, candidatesSent int, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrICEFailed, "ICE connection failed").
		WithContext("candidates_received", candidatesReceived).
		WithContext("candidates_sent", candidatesSent).
		WithSuggestion("Check NAT/firewall configuration. Ensure STUN/TURN servers are accessible.")
}

// NewPeerDisconnectedError creates a structured error for peer disconnection.
// Includes context about the disconnection reason.
//
// Example:
//
//	err := transport.NewPeerDisconnectedError("user closed application")
func NewPeerDisconnectedError(reason string) *ewerrors.EchoWarpError {
	err := ewerrors.NewError(ewerrors.ErrPeerDisconnected, "peer disconnected")
	if reason != "" {
		err = err.WithContext("reason", reason)
	}
	return err.WithSuggestion("The remote peer has disconnected. Reconnection may be required.")
}

// NewDTLSFailedError creates a structured error for DTLS handshake failures.
// Includes context about the handshake state and underlying cause.
//
// Example:
//
//	err := transport.NewDTLSFailedError("handshake timeout", underlyingError)
func NewDTLSFailedError(state string, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrDTLSFailed, "DTLS handshake failed").
		WithContext("handshake_state", state).
		WithSuggestion("This may indicate a security issue or protocol mismatch. Check TLS configuration.")
}

// NewDataChannelFailedError creates a structured error for data channel errors.
// Includes context about the data channel state and underlying cause.
//
// Example:
//
//	err := transport.NewDataChannelFailedError("audio", "closed", underlyingError)
func NewDataChannelFailedError(channelLabel string, state string, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrDataChannelFailed, "data channel error").
		WithContext("channel_label", channelLabel).
		WithContext("channel_state", state).
		WithSuggestion("The WebRTC data channel encountered an error. Reconnection may resolve the issue.")
}
