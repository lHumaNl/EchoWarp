package audio

import "context"

// PlatformAEC is the interface for OS-level acoustic echo cancellation.
// Platform AEC (CoreAudio on macOS, PulseAudio on Linux) can achieve better
// results than the software NLMS filter because it has access to the exact
// speaker output timing from the OS audio subsystem.
//
// When a platform AEC is available AND enabled, it should be preferred over
// the software AECProcessor. The software AEC can serve as a fallback.
type PlatformAEC interface {
	// Name returns the platform AEC backend name (e.g., "CoreAudio", "PulseAudio").
	Name() string

	// Available reports whether this platform AEC can be used on the current system.
	Available() bool

	// Start initializes the platform AEC with the given sample rate and channels.
	Start(sampleRate uint32, channels uint32) error

	// Process applies platform-level echo cancellation to the captured samples.
	// The implementation accesses the OS speaker output internally.
	Process(ctx context.Context, samples []float32) ([]float32, error)

	// Stop releases platform AEC resources.
	Stop() error
}

// GetPlatformAEC returns the platform-specific AEC implementation.
// Returns nil if no platform AEC is available for the current OS.
// This is a dispatcher function — the actual implementation is in
// platform_aec_darwin.go, platform_aec_linux.go, and platform_aec_stub.go.
func GetPlatformAEC() PlatformAEC {
	return getPlatformAEC()
}
