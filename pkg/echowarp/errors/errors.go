package errors

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"time"
)

// EchoWarpError represents a structured error with code, context, and suggestions.
// It implements error, json.Marshaler, and supports error wrapping/unwrapping.
type EchoWarpError struct {
	// Code is the error code (E001, E002, etc.)
	Code string `json:"code"`

	// Message is the human-readable error message
	Message string `json:"message"`

	// Cause is the underlying error (for wrapping)
	Cause error `json:"cause,omitempty"`

	// Context contains additional structured information about the error
	Context map[string]interface{} `json:"context,omitempty"`

	// Suggestion contains a recommended fix or action
	Suggestion string `json:"suggestion,omitempty"`

	// Timestamp is when the error occurred
	Timestamp time.Time `json:"timestamp"`
}

// NewError creates a new EchoWarpError with the given code and message.
// The error is initialized with the current timestamp.
// If a suggestion is registered for the code, it is populated automatically.
func NewError(code string, message string) *EchoWarpError {
	return &EchoWarpError{
		Code:       code,
		Message:    message,
		Suggestion: CodeSuggestion(code),
		Timestamp:  time.Now(),
		Context:    make(map[string]interface{}),
	}
}

// Wrap creates a new EchoWarpError that wraps an existing error.
// The original error is preserved for errors.Is() and errors.As() compatibility.
// If a suggestion is registered for the code, it is populated automatically.
func Wrap(err error, code string, message string) *EchoWarpError {
	return &EchoWarpError{
		Code:       code,
		Message:    message,
		Cause:      err,
		Suggestion: CodeSuggestion(code),
		Timestamp:  time.Now(),
		Context:    make(map[string]interface{}),
	}
}

// Error implements the error interface.
// Returns a formatted string with code, message, and optional cause.
func (e *EchoWarpError) Error() string {
	var sb strings.Builder

	// Format: [CODE] message
	fmt.Fprintf(&sb, "[%s] %s", e.Code, e.Message)

	// Add cause if present
	if e.Cause != nil {
		fmt.Fprintf(&sb, ": %v", e.Cause)
	}

	return sb.String()
}

// Unwrap returns the underlying cause for errors.Is() and errors.As().
// This enables proper error chain inspection.
func (e *EchoWarpError) Unwrap() error {
	return e.Cause
}

// WithContext adds a key-value pair to the error context.
// Returns the error for method chaining.
func (e *EchoWarpError) WithContext(key string, value interface{}) *EchoWarpError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggested fix or action for the error.
// Returns the error for method chaining.
func (e *EchoWarpError) WithSuggestion(suggestion string) *EchoWarpError {
	e.Suggestion = suggestion
	return e
}

// WithCause sets the underlying cause of the error.
// Returns the error for method chaining.
func (e *EchoWarpError) WithCause(cause error) *EchoWarpError {
	e.Cause = cause
	return e
}

// Format returns a user-friendly formatted error string.
// Includes suggestion and relevant context if available.
func (e *EchoWarpError) Format() string {
	var sb strings.Builder

	// Error header
	fmt.Fprintf(&sb, "Error [%s]: %s\n", e.Code, e.Message)

	// Add description
	if desc := CodeDescription(e.Code); desc != e.Message {
		fmt.Fprintf(&sb, "  Description: %s\n", desc)
	}

	// Add cause
	if e.Cause != nil {
		fmt.Fprintf(&sb, "  Cause: %v\n", e.Cause)
	}

	// Add context
	if len(e.Context) > 0 {
		sb.WriteString("  Context:\n")
		for key, value := range e.Context {
			fmt.Fprintf(&sb, "    - %s: %v\n", key, value)
		}
	}

	// Add suggestion
	if e.Suggestion != "" {
		fmt.Fprintf(&sb, "  Suggestion: %s\n", e.Suggestion)
	}

	// Add timestamp
	fmt.Fprintf(&sb, "  Time: %s\n", e.Timestamp.Format(time.RFC3339))

	return sb.String()
}

// MarshalJSON implements json.Marshaler for JSON serialization.
// This enables API responses and structured logging.
func (e *EchoWarpError) MarshalJSON() ([]byte, error) {
	type Alias EchoWarpError
	aux := &struct {
		*Alias
		Cause string `json:"cause,omitempty"`
	}{
		Alias: (*Alias)(e),
	}

	if e.Cause != nil {
		aux.Cause = e.Cause.Error()
	}

	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler for JSON deserialization.
func (e *EchoWarpError) UnmarshalJSON(data []byte) error {
	type Alias EchoWarpError
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Ensure context map is initialized
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}

	return nil
}

// Is implements errors.Is interface for exact error comparison.
// Requires both Code and Message to match. For code-only matching
// (ignoring message differences and error wrapping), use [HasCode] or [IsErrorCode].
func (e *EchoWarpError) Is(target error) bool {
	t, ok := target.(*EchoWarpError)
	if !ok {
		return false
	}
	return e.Code == t.Code && e.Message == t.Message
}

// GetCode extracts the error code from an error.
// Returns empty string if the error is not an EchoWarpError.
func GetCode(err error) string {
	var ewErr *EchoWarpError
	if stderrors.As(err, &ewErr) {
		return ewErr.Code
	}
	return ""
}

// GetContext extracts context value from an error.
// Returns nil if the error is not an EchoWarpError or key doesn't exist.
func GetContext(err error, key string) interface{} {
	var ewErr *EchoWarpError
	if stderrors.As(err, &ewErr) {
		return ewErr.Context[key]
	}
	return nil
}

// HasCode checks if an error has the specified error code (via errors.As, no chain traversal).
// For checking the entire error chain, use [IsErrorCode].
func HasCode(err error, code string) bool {
	return GetCode(err) == code
}

// FromCode creates a new EchoWarpError from just an error code.
// Uses the default description for the code.
func FromCode(code string) *EchoWarpError {
	return NewError(code, CodeDescription(code))
}

// WrapWithCode wraps an error with a code, using the default description.
func WrapWithCode(err error, code string) *EchoWarpError {
	return Wrap(err, code, CodeDescription(code))
}

// NewSentinel creates a sentinel error that can be used with errors.Is().
// The sentinel includes the error code for identification.
func NewSentinel(code string, message string) *EchoWarpError {
	return NewError(code, message)
}

// IsErrorCode traverses the full error chain and returns true if any wrapped error
// has the specified code. Unlike [HasCode], this walks the chain manually via Unwrap.
// Use this when an EchoWarpError may be wrapped inside other errors.
func IsErrorCode(err error, code string) bool {
	for {
		if ewErr, ok := err.(*EchoWarpError); ok {
			if ewErr.Code == code {
				return true
			}
		}
		if err = stderrors.Unwrap(err); err == nil {
			return false
		}
	}
}
