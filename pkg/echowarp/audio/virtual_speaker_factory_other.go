//go:build !darwin && !linux && !windows

package audio

import "fmt"

// NewVirtualSpeaker returns an error on unsupported platforms.
// Virtual speakers are only supported on Linux, macOS, and Windows.
func NewVirtualSpeaker(name string, sampleRate, channels uint32) (VirtualSpeaker, error) {
	return nil, fmt.Errorf("virtual speaker not supported on this platform")
}
