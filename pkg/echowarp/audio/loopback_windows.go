//go:build windows

package audio

import (
	"fmt"
	"strings"
)

// DetectBlackHole returns nil on Windows (BlackHole is macOS-only).
func DetectBlackHole(dm DeviceEnumerator) []AudioDevice { return nil }

// CleanupOrphanedAggregateDevices is a no-op on Windows.
func CleanupOrphanedAggregateDevices() {}

// ListLoopbackDevices returns playback devices available for WASAPI loopback capture.
// On Windows, any playback device can be loopback-captured natively via WASAPI
// without requiring third-party virtual audio drivers.
func ListLoopbackDevices(dm DeviceEnumerator) ([]LoopbackDevice, error) {
	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, err
	}

	var result []LoopbackDevice
	for _, out := range outputs {
		// Skip virtual cable devices — they have their own capture side.
		if isVirtualCableDevice(out.Name) {
			continue
		}
		result = append(result, LoopbackDevice{
			OutputDevice: out,
			BlackHole:    out, // No separate capture device needed on Windows.
			Channels:     out.Channels,
		})
	}
	return result, nil
}

func isVirtualCableDevice(name string) bool {
	lower := strings.ToLower(name)
	for _, term := range []string{"cable", "vb-audio", "virtual"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

// windowsLoopbackSession holds the playback device index for WASAPI loopback capture.
type windowsLoopbackSession struct {
	playbackDeviceID uint32
}

// NewLoopbackSession creates a WASAPI loopback capture session for the given
// output device. The blackholeInputDeviceName parameter is ignored on Windows.
func NewLoopbackSession(outputDeviceName, _ string) (LoopbackSession, error) {
	dm, err := NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("init device manager for loopback: %w", err)
	}
	defer dm.Close()

	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, fmt.Errorf("list output devices: %w", err)
	}

	for _, out := range outputs {
		if strings.EqualFold(out.Name, outputDeviceName) {
			return &windowsLoopbackSession{playbackDeviceID: out.ID}, nil
		}
	}

	// Fallback: substring match.
	lower := strings.ToLower(outputDeviceName)
	for _, out := range outputs {
		if strings.Contains(strings.ToLower(out.Name), lower) {
			return &windowsLoopbackSession{playbackDeviceID: out.ID}, nil
		}
	}

	return nil, fmt.Errorf("output device %q not found for loopback capture", outputDeviceName)
}

func (s *windowsLoopbackSession) CaptureDeviceID() uint32 {
	return s.playbackDeviceID
}

func (s *windowsLoopbackSession) IsNativeLoopback() bool {
	return true
}

func (s *windowsLoopbackSession) Close() error {
	return nil
}
