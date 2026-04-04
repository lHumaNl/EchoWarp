package auth

import (
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// NewAuthenticator creates an Authenticator based on configuration.
// Auto-detection logic:
// - If TLS is enabled (certs provided) → ChallengeAuth (HMAC only, TLS encrypts)
// - If TLS is disabled → ECDHAuth (ECDH encryption + optional HMAC)
func NewAuthenticator(isTLSEnabled bool, password string) Authenticator {
	if isTLSEnabled {
		return NewChallengeAuth()
	}
	return NewECDHAuth(password)
}

// NewAuthHandler creates a transport.AuthHandler based on configuration.
// This is an alias for compatibility with protocol.go.
func NewAuthHandler(isTLSEnabled bool, password string) transport.AuthHandler {
	if isTLSEnabled {
		return NewChallengeAuth()
	}
	return NewECDHAuth(password)
}
