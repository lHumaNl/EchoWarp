// Package app orchestrates the application modes.
//
// This package contains ServerApp and ClientApp which coordinate
// all components (audio, transport, auth) to implement the complete
// server and client functionality.
//
// ServerApp handles:
//   - Single-client mode (1:1)
//   - Multi-client mode (1:N broadcast, N:1 mix)
//   - Authentication and authorization
//   - Client lifecycle management
//
// ClientApp handles:
//   - Connection establishment
//   - Audio streaming
//   - Reconnection logic
package app
