//go:build linux

package audio

// NewVirtualMic creates a virtual microphone on Linux using PulseAudio pipe source.
// Audio written to the virtual mic appears as a PulseAudio source, allowing other
// applications to capture it via PulseAudio or PipeWire.
//
// Requires PulseAudio server to be running. The virtual device is automatically
// cleaned up when Close() is called.
func NewVirtualMic(name string, sampleRate, channels uint32) (VirtualMic, error) {
	return newLinuxVirtualMic(name, sampleRate, channels)
}
