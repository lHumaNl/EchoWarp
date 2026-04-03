package audio

import "context"

// AudioDevice represents a physical or virtual audio device on the system.
// The ID field is an index into the device enumeration list and may change
// between sessions if devices are added or removed.
type AudioDevice struct {
	ID         uint32
	Name       string
	IsInput    bool
	Channels   uint32
	SampleRate uint32
	BitDepth   uint32 // 16, 24, 32 (from malgo FormatType); 0 if unknown
}

// DeviceEnumerator provides access to audio device enumeration.
// Implementations should handle platform-specific audio APIs (e.g., CoreAudio,
// WASAPI, PulseAudio) through the malgo library.
type DeviceEnumerator interface {
	ListInputDevices() ([]AudioDevice, error)
	ListOutputDevices() ([]AudioDevice, error)
}

// AudioCapturer captures PCM audio from an input device and sends samples
// to the provided channel. The samples are in float32 format ranging from -1.0 to 1.0.
//
// Implementations must:
//   - Send samples to outCh when available
//   - Stop capturing when ctx is canceled
//   - Clean up all resources in Close()
type AudioCapturer interface {
	Start(ctx context.Context, deviceID uint32, outCh chan<- []float32) error
	SampleRate() uint32
	Channels() uint32
	Close() error
}

// AudioPlayer plays PCM audio to an output device by reading samples from
// the provided channel. The samples must be in float32 format ranging from -1.0 to 1.0.
//
// Implementations must:
//   - Read samples from inCh and play them to the device
//   - Handle channel closure gracefully
//   - Clean up all resources in Close()
type AudioPlayer interface {
	Start(ctx context.Context, deviceID uint32, inCh <-chan []float32) error
	Close() error
}

// LoopbackDevice describes an output device available for loopback capture via BlackHole.
type LoopbackDevice struct {
	OutputDevice AudioDevice // The output device (speakers/headphones) the user wants to capture.
	BlackHole    AudioDevice // The BlackHole input device used for actual capture.
	Channels     uint32      // Effective channel count (min of output and BlackHole).
}

// LoopbackSession manages the lifecycle of a loopback capture session.
// On macOS this creates a CoreAudio Aggregate Device combining the output
// device with BlackHole. On Windows this uses native WASAPI loopback capture.
type LoopbackSession interface {
	// CaptureDeviceID returns the device index to pass to MalgoCapturer.Start().
	// On macOS this is the BlackHole input device. On Windows this is the
	// playback device index (used with malgo.Loopback device type).
	CaptureDeviceID() uint32
	// IsNativeLoopback returns true when the session uses native OS loopback
	// (e.g. WASAPI loopback on Windows). When true, the capturer must be
	// created with WithLoopbackMode() so it opens the device as malgo.Loopback
	// instead of malgo.Capture.
	IsNativeLoopback() bool
	// Close releases all resources and restores previous audio state.
	Close() error
}

// VirtualMic represents a virtual microphone that can receive audio samples
// and make them available to other applications as a system input device.
// This is used to route captured network audio to other applications.
type VirtualMic interface {
	Write(samples []float32) error
	DeviceName() string
	Close() error
}

// VirtualSpeaker represents a virtual output device that captures audio from
// other applications. Apps output audio to this virtual device, and EchoWarp
// captures it to send to the remote side. Used in duplex mode.
type VirtualSpeaker interface {
	// Read returns the next buffer of captured audio samples.
	// Blocks until samples are available or context is canceled.
	Read() ([]float32, error)
	DeviceName() string
	Close() error
}
