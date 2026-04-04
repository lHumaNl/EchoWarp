//go:build windows

package audio

// NewVirtualMic creates a virtual microphone on Windows using VB-Audio Virtual Cable.
// Audio written to the virtual mic appears as input from the virtual cable device,
// allowing other applications to capture it.
//
// Requires VB-Audio Virtual Cable to be installed: https://vb-audio.com/Cable/
func NewVirtualMic(name string, sampleRate, channels uint32) (VirtualMic, error) {
	return newWindowsVirtualMic(name, sampleRate, channels)
}
