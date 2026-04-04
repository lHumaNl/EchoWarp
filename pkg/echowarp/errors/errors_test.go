package errors

import (
	stderrors "errors"
	"testing"
)

func TestNewError(t *testing.T) {
	err := NewError(ErrDeviceNotFound, "audio device not found")

	if err.Code != ErrDeviceNotFound {
		t.Errorf("expected code %s, got %s", ErrDeviceNotFound, err.Code)
	}

	if err.Message != "audio device not found" {
		t.Errorf("expected message 'audio device not found', got %s", err.Message)
	}

	if err.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestWrap(t *testing.T) {
	cause := stderrors.New("underlying error")
	err := Wrap(cause, ErrOpusEncode, "Opus encoding failed")

	if err.Code != ErrOpusEncode {
		t.Errorf("expected code %s, got %s", ErrOpusEncode, err.Code)
	}

	if err.Cause != cause {
		t.Error("cause should be preserved")
	}

	if !stderrors.Is(err, cause) {
		t.Error("errors.Is should find underlying cause")
	}
}

func TestWithContext(t *testing.T) {
	err := NewError(ErrDeviceNotFound, "device not found").
		WithContext("device_name", "Microphone").
		WithContext("device_id", 1)

	if err.Context["device_name"] != "Microphone" {
		t.Error("device_name context should be set")
	}

	if err.Context["device_id"] != 1 {
		t.Error("device_id context should be set")
	}
}

func TestWithSuggestion(t *testing.T) {
	suggestion := "Try reconnecting the device"
	err := NewError(ErrDeviceNotFound, "device not found").
		WithSuggestion(suggestion)

	if err.Suggestion != suggestion {
		t.Errorf("expected suggestion %s, got %s", suggestion, err.Suggestion)
	}
}

func TestErrorFormatting(t *testing.T) {
	cause := stderrors.New("underlying cause")
	err := Wrap(cause, ErrDeviceNotFound, "device not found").
		WithContext("device_name", "Mic").
		WithSuggestion("Reconnect device")

	// Test Error() method
	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() should return non-empty string")
	}

	// Test Format() method
	formatted := err.Format()
	if formatted == "" {
		t.Error("Format() should return non-empty string")
	}

	// Verify formatted output contains expected elements
	tests := []string{
		ErrDeviceNotFound,
		"device not found",
		"underlying cause",
		"device_name",
		"Mic",
		"Suggestion",
	}

	for _, test := range tests {
		if !contains(formatted, test) {
			t.Errorf("formatted output should contain %s", test)
		}
	}
}

func TestJSONSerialization(t *testing.T) {
	err := NewError(ErrDeviceNotFound, "device not found").
		WithContext("device_id", 123).
		WithSuggestion("Reconnect device")

	// Serialize to JSON
	data, serErr := err.MarshalJSON()
	if serErr != nil {
		t.Fatalf("failed to marshal: %v", serErr)
	}

	// Deserialize back
	var decoded EchoWarpError
	deErr := decoded.UnmarshalJSON(data)
	if deErr != nil {
		t.Fatalf("failed to unmarshal: %v", deErr)
	}

	if decoded.Code != err.Code {
		t.Errorf("code mismatch: expected %s, got %s", err.Code, decoded.Code)
	}

	if decoded.Message != err.Message {
		t.Errorf("message mismatch: expected %s, got %s", err.Message, decoded.Message)
	}
}

func TestUnwrap(t *testing.T) {
	cause := stderrors.New("underlying error")
	err := Wrap(cause, ErrDeviceNotFound, "device not found")

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("Unwrap should return the cause")
	}
}

func TestIsErrorCode(t *testing.T) {
	cause := stderrors.New("underlying")
	err := Wrap(cause, ErrDeviceNotFound, "device not found").
		WithContext("device", "mic")

	if !IsErrorCode(err, ErrDeviceNotFound) {
		t.Error("IsErrorCode should return true for matching code")
	}

	if IsErrorCode(err, ErrDeviceBusy) {
		t.Error("IsErrorCode should return false for non-matching code")
	}

	// Test with non-EchoWarpError
	regularErr := stderrors.New("regular error")
	if IsErrorCode(regularErr, ErrDeviceNotFound) {
		t.Error("IsErrorCode should return false for non-EchoWarpError")
	}
}

func TestGetCode(t *testing.T) {
	err := NewError(ErrDeviceNotFound, "device not found")

	code := GetCode(err)
	if code != ErrDeviceNotFound {
		t.Errorf("expected code %s, got %s", ErrDeviceNotFound, code)
	}

	// Test with non-EchoWarpError
	regularErr := stderrors.New("regular error")
	code = GetCode(regularErr)
	if code != "" {
		t.Errorf("expected empty code for non-EchoWarpError, got %s", code)
	}
}

func TestGetContext(t *testing.T) {
	err := NewError(ErrDeviceNotFound, "device not found").
		WithContext("device_name", "Microphone")

	value := GetContext(err, "device_name")
	if value != "Microphone" {
		t.Errorf("expected context value 'Microphone', got %v", value)
	}

	// Test non-existent key
	value = GetContext(err, "nonexistent")
	if value != nil {
		t.Errorf("expected nil for non-existent key, got %v", value)
	}
}

func TestHasCode(t *testing.T) {
	err := NewError(ErrDeviceNotFound, "device not found")

	if !HasCode(err, ErrDeviceNotFound) {
		t.Error("HasCode should return true for matching code")
	}

	if HasCode(err, ErrDeviceBusy) {
		t.Error("HasCode should return false for non-matching code")
	}
}

func TestFromCode(t *testing.T) {
	err := FromCode(ErrDeviceNotFound)

	if err.Code != ErrDeviceNotFound {
		t.Errorf("expected code %s, got %s", ErrDeviceNotFound, err.Code)
	}

	// Should use default description
	if err.Message == "" {
		t.Error("message should not be empty")
	}
}

func TestWrapWithCode(t *testing.T) {
	cause := stderrors.New("underlying")
	err := WrapWithCode(cause, ErrDeviceNotFound)

	if err.Code != ErrDeviceNotFound {
		t.Errorf("expected code %s, got %s", ErrDeviceNotFound, err.Code)
	}

	if err.Cause != cause {
		t.Error("cause should be preserved")
	}
}

func TestCodeDescription(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrDeviceNotFound, "Audio device not found"},
		{ErrAuthFailed, "Authentication failed"},
		{ErrICEFailed, "ICE connection failed"},
		{"E999", "Unknown error"},
	}

	for _, test := range tests {
		desc := CodeDescription(test.code)
		if desc != test.expected {
			t.Errorf("CodeDescription(%s) = %s, expected %s", test.code, desc, test.expected)
		}
	}
}

func TestChaining(t *testing.T) {
	cause := stderrors.New("underlying")

	err := Wrap(cause, ErrDeviceNotFound, "device not found").
		WithContext("device_name", "Mic").
		WithContext("device_type", "input").
		WithSuggestion("Reconnect the device")

	if err.Code != ErrDeviceNotFound {
		t.Error("chaining should preserve code")
	}

	if err.Cause != cause {
		t.Error("chaining should preserve cause")
	}

	if len(err.Context) != 2 {
		t.Errorf("expected 2 context entries, got %d", len(err.Context))
	}

	if err.Suggestion == "" {
		t.Error("chaining should preserve suggestion")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that sentinel errors work with errors.Is
	sentinel := NewSentinel(ErrDeviceNotFound, "audio device not found")

	// Should work with errors.Is
	if !stderrors.Is(sentinel, sentinel) {
		t.Error("sentinel should work with errors.Is")
	}

	// Create a wrapped version
	wrapped := Wrap(sentinel, ErrDeviceNotFound, "wrapped error")

	// Should be able to unwrap to original
	if !stderrors.Is(wrapped, sentinel) {
		t.Error("wrapped error should match original sentinel")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	// No sleep needed - time.Now() provides precise timestamps
	m.Run()
}
