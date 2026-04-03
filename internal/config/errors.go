package config

import (
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Sentinel errors for backward compatibility with errors.Is().
// These are preserved to avoid breaking existing error handling code.
// New code should use the structured error constructors below.
var (
	// ErrInvalidConfig indicates a configuration validation failure.
	// The error message contains details about which field is invalid.
	//
	// Error code: E002
	// Deprecated: Use NewInvalidConfigError() for structured errors.
	ErrInvalidConfig = ewerrors.NewSentinel(ewerrors.ErrConfigInvalid, "invalid configuration")
)

// NewConfigNotFoundError creates a structured error for missing config file.
// Includes context about the expected config file location.
//
// Example:
//
//	err := config.NewConfigNotFoundError("/etc/echowarp/config.yaml")
func NewConfigNotFoundError(configPath string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrConfigNotFound, "configuration file not found").
		WithContext("config_path", configPath).
		WithSuggestion("Create a configuration file at the expected location, or specify a custom path with --config flag.")
}

// NewInvalidConfigError creates a structured error for invalid configuration.
// Includes context about which field failed validation and why.
//
// Example:
//
//	err := config.NewInvalidConfigError("sample_rate", "must be positive", 0)
func NewInvalidConfigError(field string, reason string, value interface{}) *ewerrors.EchoWarpError {
	msg := field + ": " + reason
	return ewerrors.NewError(ewerrors.ErrConfigInvalid, msg).
		WithContext("field", field).
		WithContext("reason", reason).
		WithContext("value", value).
		WithSuggestion("Check the configuration documentation for valid values and syntax.")
}

// NewConfigValidationError creates a structured error for configuration validation failures.
// Includes context about all validation errors found.
//
// Example:
//
//	err := config.NewConfigValidationError([]string{"sample_rate: must be positive", "channels: must be 1 or 2"})
func NewConfigValidationError(validationErrors []string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrConfigValidation, "configuration validation failed").
		WithContext("validation_errors", validationErrors).
		WithContext("error_count", len(validationErrors)).
		WithSuggestion("Fix all validation errors listed in the context. See documentation for valid configuration options.")
}

// NewUnsupportedPlatformError creates a structured error for unsupported platform.
// Includes context about the current platform.
//
// Example:
//
//	err := config.NewUnsupportedPlatformError("linux/arm")
func NewUnsupportedPlatformError(platform string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrUnsupportedPlatform, "unsupported platform").
		WithContext("platform", platform).
		WithSuggestion("EchoWarp supports Windows (amd64) and macOS (amd64, arm64). Check if your platform is supported.")
}
