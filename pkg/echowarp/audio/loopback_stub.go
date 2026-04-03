//go:build !darwin && !windows && !linux

package audio

import "fmt"

// DetectBlackHole returns nil on platforms without loopback support.
func DetectBlackHole(dm DeviceEnumerator) []AudioDevice { return nil }

// ListLoopbackDevices returns nil on platforms without loopback support.
func ListLoopbackDevices(dm DeviceEnumerator) ([]LoopbackDevice, error) { return nil, nil }

// CleanupOrphanedAggregateDevices is a no-op on platforms without loopback support.
func CleanupOrphanedAggregateDevices() {}

// NewLoopbackSession returns an error on platforms without loopback support.
func NewLoopbackSession(outputDeviceName, blackholeInputDeviceName string) (LoopbackSession, error) {
	return nil, fmt.Errorf("loopback capture is not supported on this platform")
}
