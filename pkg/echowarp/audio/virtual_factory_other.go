//go:build !linux && !darwin && !windows

package audio

import "fmt"

// NewVirtualMic returns an error on unsupported platforms.
// Virtual microphones are only supported on Linux, macOS, and Windows.
func NewVirtualMic(name string, sampleRate, channels uint32) (VirtualMic, error) {
	return nil, fmt.Errorf("virtual mic not supported on this platform")
}
