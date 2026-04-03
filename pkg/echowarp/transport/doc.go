// Package transport provides WebRTC-based network transport.
//
// This package implements:
//   - TCP signaling for SDP/ICE exchange
//   - WebRTC peer connection management
//   - ICE/STUN/TURN NAT traversal
//   - DTLS/SRTP encryption
//   - Multi-peer connection support
//
// Signaling uses JSON Lines protocol over TCP for reliability.
// Media transport uses WebRTC with mandatory encryption.
package transport
