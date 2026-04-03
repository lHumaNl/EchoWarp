//go:build darwin

package audio

// NewVirtualSpeaker creates a virtual speaker on macOS using BlackHole.
// Other applications output audio to the BlackHole device, and EchoWarp
// captures it to send to the remote side in duplex mode.
//
// Requires BlackHole to be installed: https://existential.audio/blackhole/
// or run: brew install blackhole-2ch
func NewVirtualSpeaker(name string, sampleRate, channels uint32) (VirtualSpeaker, error) {
	return newDarwinVirtualSpeaker(name, sampleRate, channels)
}
