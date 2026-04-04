//go:build linux

package audio

// NewVirtualSpeaker creates a virtual speaker on Linux using PulseAudio pipe sink.
// Other applications output audio to this PulseAudio sink, and EchoWarp
// reads from it to send to the remote side in duplex mode.
//
// Requires PulseAudio server to be running. The virtual device is automatically
// cleaned up when Close() is called.
func NewVirtualSpeaker(name string, sampleRate, channels uint32) (VirtualSpeaker, error) {
	return newLinuxVirtualSpeaker(name, sampleRate, channels)
}
