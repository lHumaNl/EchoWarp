//go:build darwin

package audio

// NewVirtualMic creates a virtual microphone on macOS using BlackHole.
// Audio written to the virtual mic appears as input from the BlackHole device,
// allowing other applications to capture it.
//
// Requires BlackHole to be installed: https://existential.audio/blackhole/
// or run: brew install blackhole-2ch
func NewVirtualMic(name string, sampleRate, channels uint32) (VirtualMic, error) {
	return newDarwinVirtualMic(name, sampleRate, channels)
}
