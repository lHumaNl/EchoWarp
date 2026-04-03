package auth

import (
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Sentinel errors for backward compatibility with errors.Is().
// These are preserved to avoid breaking existing error handling code.
// New code should use the structured error constructors below.
var (
	// ErrAuthFailed is returned when authentication credentials are invalid.
	//
	// Error code: E300
	// Deprecated: Use NewAuthFailedError() for structured errors.
	ErrAuthFailed = ewerrors.NewSentinel(ewerrors.ErrAuthFailed, "authentication failed")

	// ErrAuthTimeout is returned when authentication does not complete within the timeout.
	//
	// Error code: E301
	// Deprecated: Use NewAuthTimeoutError() for structured errors.
	ErrAuthTimeout = ewerrors.NewSentinel(ewerrors.ErrAuthTimeout, "authentication timeout")

	// ErrVersionMismatch is returned when client and server have incompatible protocol versions.
	//
	// Error code: E304
	// Deprecated: Use NewVersionMismatchError() for structured errors.
	ErrVersionMismatch = ewerrors.NewSentinel(ewerrors.ErrVersionMismatch, "incompatible version")

	// ErrKeyExchangeFailed is returned when cryptographic key exchange fails.
	//
	// Error code: E305
	// Deprecated: Use NewKeyExchangeFailedError() for structured errors.
	ErrKeyExchangeFailed = ewerrors.NewSentinel(ewerrors.ErrKeyExchangeFailed, "key exchange failed")

	// ErrDecryptionFailed is returned when message decryption fails due to invalid key or tampering.
	//
	// Error code: E306
	// Deprecated: Use NewDecryptionFailedError() for structured errors.
	ErrDecryptionFailed = ewerrors.NewSentinel(ewerrors.ErrDecryptionFailed, "decryption failed")
)

// NewAuthFailedError creates a structured error for authentication failures.
// Includes context about the authentication method and attempt count.
//
// Example:
//
//	err := auth.NewAuthFailedError("password", 3)
func NewAuthFailedError(method string, attempts int) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrAuthFailed, "authentication failed").
		WithContext("auth_method", method).
		WithContext("attempts", attempts).
		WithSuggestion("Verify the password is correct. Check if the account is locked after too many failed attempts.")
}

// NewAuthTimeoutError creates a structured error for authentication timeouts.
// Includes context about the timeout duration and current state.
//
// Example:
//
//	err := auth.NewAuthTimeoutError(30*time.Second, "waiting_for_response")
func NewAuthTimeoutError(timeoutMs int64, state string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrAuthTimeout, "authentication timeout").
		WithContext("timeout_ms", timeoutMs).
		WithContext("auth_state", state).
		WithSuggestion("Check network latency. Consider increasing the authentication timeout in configuration.")
}

// NewInvalidPasswordError creates a structured error for invalid password.
// Includes context about the attempt count.
//
// Example:
//
//	err := auth.NewInvalidPasswordError(2)
func NewInvalidPasswordError(attempts int) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrInvalidPassword, "invalid password").
		WithContext("attempts", attempts).
		WithSuggestion("Double-check the password. Remember that passwords are case-sensitive.")
}

// NewClientBannedError creates a structured error for banned client connections.
// Includes context about the ban reason if available.
//
// Example:
//
//	err := auth.NewClientBannedError("Too many failed auth attempts", time.Now().Add(24*time.Hour))
func NewClientBannedError(reason string, expiresAt int64) *ewerrors.EchoWarpError {
	err := ewerrors.NewError(ewerrors.ErrClientBanned, "client is banned")
	if reason != "" {
		err = err.WithContext("ban_reason", reason)
	}
	if expiresAt > 0 {
		err = err.WithContext("ban_expires_at", expiresAt)
	}
	return err.WithSuggestion("Contact the server administrator for more information about the ban.")
}

// NewVersionMismatchError creates a structured error for protocol version mismatches.
// Includes context about client and server versions.
//
// Example:
//
//	err := auth.NewVersionMismatchError("1.0.0", "2.0.0")
func NewVersionMismatchError(clientVersion string, serverVersion string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrVersionMismatch, "incompatible version").
		WithContext("client_version", clientVersion).
		WithContext("server_version", serverVersion).
		WithSuggestion("Update EchoWarp to ensure client and server are running compatible versions.")
}

// NewKeyExchangeFailedError creates a structured error for key exchange failures.
// Includes context about the key exchange method and underlying cause.
//
// Example:
//
//	err := auth.NewKeyExchangeFailedError("X25519", underlyingError)
func NewKeyExchangeFailedError(method string, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrKeyExchangeFailed, "key exchange failed").
		WithContext("key_exchange_method", method).
		WithSuggestion("This may indicate a security issue or network problem. Try reconnecting.")
}

// NewDecryptionFailedError creates a structured error for decryption failures.
// Includes context about the message type and underlying cause.
//
// Example:
//
//	err := auth.NewDecryptionFailedError("auth_response", underlyingError)
func NewDecryptionFailedError(messageType string, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrDecryptionFailed, "decryption failed").
		WithContext("message_type", messageType).
		WithSuggestion("The message may have been tampered with or the session key is incorrect. Try reconnecting.")
}
