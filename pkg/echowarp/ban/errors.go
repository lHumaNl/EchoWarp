package ban

import (
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Sentinel errors for backward compatibility with errors.Is().
// These are preserved to avoid breaking existing error handling code.
// New code should use the structured error constructors below.
var (
	// ErrClientBanned is returned when a banned client attempts to connect.
	//
	// Error code: E303
	// Deprecated: Use NewClientBannedError() from auth package for structured errors.
	ErrClientBanned = ewerrors.NewSentinel(ewerrors.ErrClientBanned, "client is banned")
)

// NewClientBannedError creates a structured error for banned client connections.
// Includes context about the ban reason, duration, and expiration.
//
// Example:
//
//	err := ban.NewClientBannedError("192.168.1.100", "Too many failed auth attempts", time.Now().Add(24*time.Hour))
func NewClientBannedError(clientID string, reason string, expiresAt int64) *ewerrors.EchoWarpError {
	err := ewerrors.NewError(ewerrors.ErrClientBanned, "client is banned").
		WithContext("client_id", clientID)

	if reason != "" {
		err = err.WithContext("ban_reason", reason)
	}
	if expiresAt > 0 {
		err = err.WithContext("ban_expires_at", expiresAt)
	}

	return err.WithSuggestion("Contact the server administrator for more information or wait for the ban to expire.")
}
