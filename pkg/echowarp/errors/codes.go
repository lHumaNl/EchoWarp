// Package errors provides structured error handling with error codes for EchoWarp.
// Error codes are organized by category ranges for easy identification and filtering.
package errors

// Error code ranges:
// - E001-E099: General errors (config, validation)
// - E100-E199: Audio errors (capture, playback, codec)
// - E200-E299: Transport errors (signaling, WebRTC, ICE)
// - E300-E399: Auth errors (authentication, authorization)
// - E400-E499: Network errors (connection, timeout)
// - E500-E599: Internal errors (state, goroutine)

const (
	// General Errors (E001-E099)
	// These errors relate to configuration, validation, and general system issues.

	// ErrConfigNotFound indicates the configuration file could not be found.
	// Check that the config file exists at the expected location.
	ErrConfigNotFound = "E001"

	// ErrConfigInvalid indicates the configuration contains invalid values.
	// Review configuration syntax and value constraints.
	ErrConfigInvalid = "E002"

	// ErrConfigValidation indicates configuration validation failed.
	// Check error details for specific validation failures.
	ErrConfigValidation = "E003"

	// ErrUnsupportedPlatform indicates the current platform is not supported.
	// EchoWarp requires specific platform capabilities.
	ErrUnsupportedPlatform = "E004"
)

const (
	// Audio Errors (E100-E199)
	// These errors relate to audio device access, encoding, and playback.

	// ErrDeviceNotFound indicates the requested audio device does not exist.
	// Verify the device is connected and recognized by the system.
	ErrDeviceNotFound = "E100"

	// ErrDeviceBusy indicates the audio device is in use by another application.
	// Close other applications using the audio device.
	ErrDeviceBusy = "E101"

	// ErrDeviceInUse is an alias for ErrDeviceBusy for backward compatibility.
	ErrDeviceInUse = "E101"

	// ErrOpusEncode indicates Opus audio encoding failed.
	// Check audio format and encoder configuration.
	ErrOpusEncode = "E102"

	// ErrOpusDecode indicates Opus audio decoding failed.
	// The received audio data may be corrupted.
	ErrOpusDecode = "E103"

	// ErrBufferOverflow indicates the audio buffer has overflowed.
	// The system cannot process audio data fast enough.
	ErrBufferOverflow = "E104"

	// ErrFormatUnsupported indicates the requested audio format is not supported.
	// Try a different sample rate or channel configuration.
	ErrFormatUnsupported = "E105"

	// ErrBlackHoleNotInstalled indicates BlackHole virtual audio driver is missing (macOS).
	// Install BlackHole from https://existential.audio/blackhole/
	ErrBlackHoleNotInstalled = "E106"

	// ErrVirtualCableNotInstalled indicates VB-Audio Virtual Cable is missing (Windows).
	// Install VB-Audio Virtual Cable from https://vb-audio.com/Cable/
	ErrVirtualCableNotInstalled = "E107"

	// ErrAggregateDeviceCreate indicates macOS CoreAudio aggregate device creation failed.
	// This is needed for loopback capture via BlackHole.
	ErrAggregateDeviceCreate = "E108"

	// ErrSystemOutputSwitch indicates failure to switch the macOS system audio output device.
	// This is needed for loopback capture to route audio through the aggregate device.
	ErrSystemOutputSwitch = "E109"

	// ErrAudioContextInit indicates the audio system context could not be initialized.
	// The OS audio subsystem may be unavailable or misconfigured.
	ErrAudioContextInit = "E110"

	// ErrDeviceInitFailed indicates the audio device could not be initialized.
	// The device may be disconnected, in use, or incompatible.
	ErrDeviceInitFailed = "E111"

	// ErrDeviceStartFailed indicates the audio device failed to start streaming.
	// The device may have been disconnected after initialization.
	ErrDeviceStartFailed = "E112"

	// ErrDeviceEnumFailed indicates audio device enumeration failed.
	// The OS audio subsystem may be unavailable.
	ErrDeviceEnumFailed = "E113"
)

const (
	// Transport Errors (E200-E299)
	// These errors relate to WebRTC, signaling, and peer connections.

	// ErrSignalingFailed indicates the signaling connection failed.
	// Check network connectivity and signaling server availability.
	ErrSignalingFailed = "E200"

	// ErrSignalingClosed indicates the signaling connection was closed unexpectedly.
	// The signaling server may have terminated the connection.
	ErrSignalingClosed = "E201"

	// ErrICEFailed indicates ICE connection negotiation failed.
	// Check NAT/firewall configuration and STUN/TURN server settings.
	ErrICEFailed = "E202"

	// ErrDTLSFailed indicates DTLS handshake failed.
	// This may indicate a security issue or protocol mismatch.
	ErrDTLSFailed = "E203"

	// ErrPeerDisconnected indicates the remote peer has disconnected.
	// The connection was terminated by the remote party.
	ErrPeerDisconnected = "E204"

	// ErrPeerClosed indicates the peer connection was closed.
	// Alias for ErrPeerDisconnected for backward compatibility.
	ErrPeerClosed = "E204"

	// ErrConnectionFailed indicates WebRTC connection could not be established.
	// Check network configuration and peer availability.
	ErrConnectionFailed = "E205"

	// ErrDataChannelFailed indicates WebRTC data channel error.
	// The data channel could not be opened or was closed.
	ErrDataChannelFailed = "E206"
)

const (
	// Auth Errors (E300-E399)
	// These errors relate to authentication and authorization.

	// ErrAuthFailed indicates authentication credentials were rejected.
	// Verify the password is correct.
	ErrAuthFailed = "E300"

	// ErrAuthTimeout indicates authentication did not complete in time.
	// Check network latency and timeout configuration.
	ErrAuthTimeout = "E301"

	// ErrInvalidPassword indicates the provided password is incorrect.
	// Double-check the password and try again.
	ErrInvalidPassword = "E302"

	// ErrClientBanned indicates the client is banned from connecting.
	// Contact the server administrator for more information.
	ErrClientBanned = "E303"

	// ErrClientKicked indicates the client was kicked by the server administrator.
	// The client should not auto-reconnect; the user can manually reconnect.
	ErrClientKicked = "E308"

	// ErrClientBannedByAdmin indicates the client was banned by the server administrator
	// during an active session (via ban_notify). Unlike ErrClientBanned (auth-time ban),
	// this is received mid-session. The client must not auto-reconnect.
	ErrClientBannedByAdmin = "E309"

	// ErrVersionMismatch indicates incompatible protocol versions.
	// Ensure client and server are running compatible versions.
	ErrVersionMismatch = "E304"

	// ErrKeyExchangeFailed indicates cryptographic key exchange failed.
	// This may indicate a security issue or network problem.
	ErrKeyExchangeFailed = "E305"

	// ErrDecryptionFailed indicates message decryption failed.
	// The message may have been tampered with or key is incorrect.
	ErrDecryptionFailed = "E306"

	// ErrRateLimited indicates too many authentication attempts.
	// Wait for the cooldown period before retrying.
	ErrRateLimited = "E307"
)

const (
	// Network Errors (E400-E499)
	// These errors relate to network connectivity and communication.

	// ErrNetworkConnection indicates a general network connection failure.
	// Check network connectivity and firewall settings.
	ErrNetworkConnection = "E400"

	// ErrConnectionTimeout indicates a connection timed out.
	// The remote host may be unreachable or slow to respond.
	ErrConnectionTimeout = "E401"

	// ErrNetworkUnreachable indicates the network is unreachable.
	// Check network configuration and routing.
	ErrNetworkUnreachable = "E402"

	// ErrDNSFailed indicates DNS resolution failed.
	// Check DNS configuration and hostname validity.
	ErrDNSFailed = "E403"
)

const (
	// Internal Errors (E500-E599)
	// These errors indicate internal state issues or unexpected conditions.

	// ErrInternalState indicates an invalid internal state.
	// This is typically a bug that should be reported.
	ErrInternalState = "E500"

	// ErrGoroutinePanic indicates a goroutine panicked unexpectedly.
	// Check logs for panic details and stack trace.
	ErrGoroutinePanic = "E501"

	// ErrNotInitialized indicates a component was used before initialization.
	// Ensure proper initialization sequence is followed.
	ErrNotInitialized = "E502"

	// ErrAlreadyRunning indicates an operation was attempted on an already running component.
	// Stop the component before starting it again.
	ErrAlreadyRunning = "E503"

	// ErrNotRunning indicates an operation was attempted on a non-running component.
	// Start the component before performing the operation.
	ErrNotRunning = "E504"
)

// codeSuggestions maps error codes to actionable suggestions for users.
var codeSuggestions = map[string]string{
	// General
	ErrConfigNotFound:      "Run 'echowarp config init' to create a default config, or check that the config file exists at the expected path.",
	ErrConfigInvalid:       "Review config file syntax. Run 'echowarp config init' to regenerate a valid default config.",
	ErrConfigValidation:    "Check error details for the specific field that failed validation. Run 'echowarp config init' for reference defaults.",
	ErrUnsupportedPlatform: "EchoWarp supports Linux, macOS, and Windows. Ensure you are running a supported OS and architecture.",

	// Audio
	ErrDeviceNotFound:           "Run 'echowarp devices' to list available devices. Verify the device is connected and not disabled by the OS.",
	ErrDeviceBusy:               "Close other applications that may be using the audio device (e.g. video conferencing apps, DAWs). Then retry.",
	ErrOpusEncode:               "Check that sample rate and channel count match the Opus encoder configuration (--sample-rate, --channels).",
	ErrOpusDecode:               "The received audio packet may be corrupted. Check network stability and packet loss metrics.",
	ErrBufferOverflow:           "The system cannot process audio fast enough. Try increasing --audio-buffer-frames or reducing --opus-bitrate.",
	ErrFormatUnsupported:        "Opus supports sample rates: 8000, 12000, 16000, 24000, 48000 Hz and 1–2 channels. Adjust --sample-rate and --channels.",
	ErrBlackHoleNotInstalled:    "Install BlackHole virtual audio driver from https://existential.audio/blackhole/ and restart EchoWarp.",
	ErrVirtualCableNotInstalled: "Install VB-Audio Virtual Cable from https://vb-audio.com/Cable/ and restart EchoWarp.",
	ErrAggregateDeviceCreate:    "Failed to create aggregate device. Try creating Multi-Output Device manually in Audio MIDI Setup.",
	ErrSystemOutputSwitch:       "Failed to switch system audio output. Check System Preferences > Sound and set the output manually.",
	ErrAudioContextInit:         "Audio system initialization failed. Check that audio drivers are installed and the audio service is running.",
	ErrDeviceInitFailed:         "Could not open audio device. Verify it is connected, not in use by another app, and supports the configured format.",
	ErrDeviceStartFailed:        "Audio device failed to start. Try closing other apps using the device, or select a different device.",
	ErrDeviceEnumFailed:         "Could not list audio devices. Check that audio drivers are installed and the audio service is running.",

	// Transport
	ErrSignalingFailed:   "Check that the server address and port are correct and reachable. Verify no firewall blocks TCP on the configured port.",
	ErrSignalingClosed:   "The server closed the connection. Check server logs for errors. The client will attempt to reconnect automatically.",
	ErrICEFailed:         "ICE negotiation failed. If behind NAT, add a TURN server via --turn-server. Check that UDP ports are not blocked by firewall.",
	ErrDTLSFailed:        "DTLS handshake failed. This may indicate a MITM or protocol mismatch. Ensure client and server versions are compatible.",
	ErrPeerDisconnected:  "The remote peer disconnected. The client will attempt to reconnect automatically if --max-reconnect-attempts > 0.",
	ErrConnectionFailed:  "WebRTC connection could not be established. Check STUN/TURN server accessibility and firewall UDP rules.",
	ErrDataChannelFailed: "WebRTC data channel error. Try reconnecting. If the issue persists, check for packet loss or NAT traversal issues.",

	// Auth
	ErrAuthFailed:          "Authentication rejected by the server. Verify the --password matches the server configuration.",
	ErrAuthTimeout:         "Authentication timed out. Check network latency. The server may be overloaded or unreachable.",
	ErrInvalidPassword:     "The password is incorrect. Double-check the --password flag or ECHOWARP_PASSWORD environment variable.",
	ErrClientBanned:        "This client IP has been banned after too many failed attempts. Contact the server administrator to unban, or wait for the ban to expire.",
	ErrVersionMismatch:     "Client and server are running incompatible versions. Run 'echowarp update' to update to the latest version.",
	ErrKeyExchangeFailed:   "ECDH key exchange failed. This may indicate a network issue or a MITM attack. Check connection security.",
	ErrDecryptionFailed:    "Message decryption failed. The message may have been tampered with. Check for MITM or replay attacks.",
	ErrRateLimited:         "Too many authentication attempts from this IP. Wait for the cooldown period (default: 60s) before retrying.",
	ErrClientKicked:        "You were kicked by the server administrator. You can try reconnecting manually.",
	ErrClientBannedByAdmin: "You were banned by the server administrator. Reconnecting will not help — contact the administrator.",

	// Network
	ErrNetworkConnection:  "Network connection failed. Check internet connectivity, firewall rules, and that the server is running.",
	ErrConnectionTimeout:  "Connection timed out. The remote host may be unreachable or slow. Check --address and network route.",
	ErrNetworkUnreachable: "Network is unreachable. Check network interface configuration, routing table, and DNS settings.",
	ErrDNSFailed:          "DNS resolution failed for the server address. Verify the hostname is correct and DNS is reachable.",

	// Internal
	ErrInternalState:  "Invalid internal state detected. This is likely a bug. Please report it at https://github.com/s0up4200/EchoWarp/issues.",
	ErrGoroutinePanic: "An internal goroutine panicked. Check logs for the stack trace and report the issue at https://github.com/s0up4200/EchoWarp/issues.",
	ErrNotInitialized: "A component was used before initialization. This is likely a bug. Please report it at https://github.com/s0up4200/EchoWarp/issues.",
	ErrAlreadyRunning: "The component is already running. Stop it first before starting again.",
	ErrNotRunning:     "The component is not running. Start it first before performing this operation.",
}

// CodeSuggestion returns an actionable suggestion for an error code.
func CodeSuggestion(code string) string {
	return codeSuggestions[code]
}

// codeDescriptions maps error codes to human-readable descriptions.
var codeDescriptions = map[string]string{
	// General
	ErrConfigNotFound:      "Configuration file not found",
	ErrConfigInvalid:       "Invalid configuration",
	ErrConfigValidation:    "Configuration validation failed",
	ErrUnsupportedPlatform: "Unsupported platform",

	// Audio
	ErrDeviceNotFound:           "Audio device not found",
	ErrDeviceBusy:               "Audio device busy",
	ErrOpusEncode:               "Opus encoding failed",
	ErrOpusDecode:               "Opus decoding failed",
	ErrBufferOverflow:           "Audio buffer overflow",
	ErrFormatUnsupported:        "Audio format not supported",
	ErrBlackHoleNotInstalled:    "BlackHole virtual audio device not installed",
	ErrVirtualCableNotInstalled: "VB-Audio Virtual Cable not installed",
	ErrAggregateDeviceCreate:    "CoreAudio aggregate device creation failed",
	ErrSystemOutputSwitch:       "System audio output switch failed",
	ErrAudioContextInit:         "Audio system initialization failed",
	ErrDeviceInitFailed:         "Audio device initialization failed",
	ErrDeviceStartFailed:        "Audio device failed to start",
	ErrDeviceEnumFailed:         "Audio device enumeration failed",

	// Transport
	ErrSignalingFailed:   "Signaling connection failed",
	ErrSignalingClosed:   "Signaling connection closed",
	ErrICEFailed:         "ICE connection failed",
	ErrDTLSFailed:        "DTLS handshake failed",
	ErrPeerDisconnected:  "Peer disconnected",
	ErrConnectionFailed:  "WebRTC connection failed",
	ErrDataChannelFailed: "Data channel error",

	// Auth
	ErrAuthFailed:          "Authentication failed",
	ErrAuthTimeout:         "Authentication timeout",
	ErrInvalidPassword:     "Invalid password",
	ErrClientBanned:        "Client is banned",
	ErrVersionMismatch:     "Protocol version mismatch",
	ErrKeyExchangeFailed:   "Key exchange failed",
	ErrDecryptionFailed:    "Decryption failed",
	ErrRateLimited:         "Rate limit exceeded",
	ErrClientKicked:        "Kicked by server",
	ErrClientBannedByAdmin: "Banned by server administrator",

	// Network
	ErrNetworkConnection:  "Network connection failed",
	ErrConnectionTimeout:  "Connection timeout",
	ErrNetworkUnreachable: "Network unreachable",
	ErrDNSFailed:          "DNS resolution failed",

	// Internal
	ErrInternalState:  "Invalid internal state",
	ErrGoroutinePanic: "Goroutine panic",
	ErrNotInitialized: "Component not initialized",
	ErrAlreadyRunning: "Component already running",
	ErrNotRunning:     "Component not running",
}

// CodeDescription returns a human-readable description for an error code.
func CodeDescription(code string) string {
	if desc, ok := codeDescriptions[code]; ok {
		return desc
	}
	return "Unknown error"
}
