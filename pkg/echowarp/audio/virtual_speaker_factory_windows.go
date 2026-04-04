//go:build windows

package audio

// NewVirtualSpeaker creates a virtual speaker on Windows using VB-Audio Virtual Cable.
// Other applications output audio to the VB-Cable device, and EchoWarp
// captures it to send to the remote side in duplex mode.
//
// Requires VB-Audio Virtual Cable to be installed: https://vb-audio.com/Cable/
func NewVirtualSpeaker(name string, sampleRate, channels uint32) (VirtualSpeaker, error) {
	return newWindowsVirtualSpeaker(name, sampleRate, channels)
}
