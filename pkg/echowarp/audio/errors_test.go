package audio

import (
	"errors"
	"strings"
	"testing"
)

func TestAudioErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"ErrDeviceNotFound", ErrDeviceNotFound, "audio device not found"},
		{"ErrDeviceInUse", ErrDeviceInUse, "audio device is in use"},
		{"ErrFormatUnsupported", ErrFormatUnsupported, "audio format not supported by device"},
		{"ErrBlackHoleNotInstalled", ErrBlackHoleNotInstalled, "BlackHole virtual audio device not installed"},
		{"ErrVirtualCableNotInstalled", ErrVirtualCableNotInstalled, "VB-Audio Virtual Cable not installed"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test errors.Is compatibility
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("error should match itself with errors.Is")
			}

			// Test error message contains expected text
			errStr := tt.err.Error()
			if !strings.Contains(errStr, tt.contains) {
				t.Errorf("expected error to contain %q, got %q", tt.contains, errStr)
			}

			// Test error code is included in the error message
			if !strings.Contains(errStr, "[E") {
				t.Errorf("error message should include error code prefix, got %q", errStr)
			}
		})
	}
}

func TestStructuredErrors(t *testing.T) {
	t.Run("NewDeviceNotFoundError", func(t *testing.T) {
		err := NewDeviceNotFoundError("Test Device")
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Check error message
		errStr := err.Error()
		if !strings.Contains(errStr, "E100") {
			t.Errorf("error should contain code E100, got %s", errStr)
		}
		if !strings.Contains(errStr, "device not found") {
			t.Errorf("error should contain 'device not found', got %s", errStr)
		}

		// Check context
		ctx := err.Context
		if ctx == nil {
			t.Fatal("context should not be nil")
		}
		if ctx["device_name"] != "Test Device" {
			t.Errorf("expected device_name 'Test Device', got %v", ctx["device_name"])
		}

		// Check suggestion
		if err.Suggestion == "" {
			t.Error("suggestion should not be empty")
		}
	})

	t.Run("NewDeviceBusyError", func(t *testing.T) {
		err := NewDeviceBusyError("Test Device", "Zoom")
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Check error code
		if !strings.Contains(err.Error(), "E101") {
			t.Errorf("error should contain code E101, got %s", err.Error())
		}

		// Check context
		if err.Context["device_name"] != "Test Device" {
			t.Errorf("expected device_name 'Test Device', got %v", err.Context["device_name"])
		}
		if err.Context["used_by"] != "Zoom" {
			t.Errorf("expected used_by 'Zoom', got %v", err.Context["used_by"])
		}
	})

	t.Run("NewDeviceBusyError_EmptyUsedBy", func(t *testing.T) {
		err := NewDeviceBusyError("Test Device", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Should not have used_by context when empty
		if _, exists := err.Context["used_by"]; exists {
			t.Error("should not include used_by when empty")
		}
	})

	t.Run("NewFormatUnsupportedError", func(t *testing.T) {
		err := NewFormatUnsupportedError(96000, 2, "Test Device")
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Check error code
		if !strings.Contains(err.Error(), "E105") {
			t.Errorf("error should contain code E105, got %s", err.Error())
		}

		// Check context
		if err.Context["requested_sample_rate"] != 96000 {
			t.Errorf("expected sample_rate 96000, got %v", err.Context["requested_sample_rate"])
		}
		if err.Context["requested_channels"] != 2 {
			t.Errorf("expected channels 2, got %v", err.Context["requested_channels"])
		}
	})

	t.Run("NewOpusEncodeError", func(t *testing.T) {
		cause := errors.New("encoder error")
		err := NewOpusEncodeError(48000, 2, cause)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Check wrapping
		if !errors.Is(err, cause) {
			t.Error("should wrap underlying cause")
		}

		// Check error code
		if !strings.Contains(err.Error(), "E102") {
			t.Errorf("error should contain code E102, got %s", err.Error())
		}
	})

	t.Run("NewBufferOverflowError", func(t *testing.T) {
		err := NewBufferOverflowError(4096, 0.95)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Check error code
		if !strings.Contains(err.Error(), "E104") {
			t.Errorf("error should contain code E104, got %s", err.Error())
		}

		// Check context
		if err.Context["buffer_size"] != 4096 {
			t.Errorf("expected buffer_size 4096, got %v", err.Context["buffer_size"])
		}
		if err.Context["fill_ratio"] != 0.95 {
			t.Errorf("expected fill_ratio 0.95, got %v", err.Context["fill_ratio"])
		}
	})

	t.Run("Platform-specific errors", func(t *testing.T) {
		t.Run("BlackHole", func(t *testing.T) {
			err := NewBlackHoleNotInstalledError()
			if !strings.Contains(err.Error(), "E106") {
				t.Errorf("error should contain code E106, got %s", err.Error())
			}
			if !strings.Contains(err.Suggestion, "existential.audio") {
				t.Error("suggestion should mention BlackHole download URL")
			}
		})

		t.Run("VirtualCable", func(t *testing.T) {
			err := NewVirtualCableNotInstalledError()
			if !strings.Contains(err.Error(), "E107") {
				t.Errorf("error should contain code E107, got %s", err.Error())
			}
			if !strings.Contains(err.Suggestion, "vb-audio.com") {
				t.Error("suggestion should mention VB-Audio download URL")
			}
		})
	})
}

func TestErrorFormatting(t *testing.T) {
	err := NewDeviceNotFoundError("Test Device")
	formatted := err.Format()

	// Check that formatted output includes all important information
	requiredElements := []string{
		"E100",
		"device not found",
		"device_name",
		"Test Device",
		"Suggestion",
		"Time",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(formatted, elem) {
			t.Errorf("formatted output should contain %q, got:\n%s", elem, formatted)
		}
	}
}
