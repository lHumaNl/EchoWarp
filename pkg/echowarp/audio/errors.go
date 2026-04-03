package audio

import (
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Sentinel errors for backward compatibility with errors.Is().
// These are preserved to avoid breaking existing error handling code.
// New code should use the structured error constructors below.
var (
	// ErrDeviceNotFound indicates the requested audio device does not exist
	// or has been disconnected since enumeration.
	//
	// Error code: E100
	// Deprecated: Use NewDeviceNotFoundError() for structured errors.
	ErrDeviceNotFound = ewerrors.NewSentinel(ewerrors.ErrDeviceNotFound, "audio device not found")

	// ErrDeviceInUse indicates the audio device is already in use by another
	// application and cannot be opened exclusively.
	//
	// Error code: E101
	// Deprecated: Use NewDeviceBusyError() for structured errors.
	ErrDeviceInUse = ewerrors.NewSentinel(ewerrors.ErrDeviceBusy, "audio device is in use")

	// ErrFormatUnsupported indicates the requested audio format (sample rate,
	// channels, etc.) is not supported by the device.
	//
	// Error code: E105
	// Deprecated: Use NewFormatUnsupportedError() for structured errors.
	ErrFormatUnsupported = ewerrors.NewSentinel(ewerrors.ErrFormatUnsupported, "audio format not supported by device")

	// ErrBlackHoleNotInstalled indicates the BlackHole virtual audio driver
	// is not installed on macOS. Required for virtual microphone functionality.
	//
	// Error code: E106
	// Deprecated: Use NewBlackHoleNotInstalledError() for structured errors.
	ErrBlackHoleNotInstalled = ewerrors.NewSentinel(ewerrors.ErrBlackHoleNotInstalled, "BlackHole virtual audio device not installed")

	// ErrVirtualCableNotInstalled indicates the VB-Audio Virtual Cable driver
	// is not installed on Windows. Required for virtual microphone functionality.
	//
	// Error code: E107
	// Deprecated: Use NewVirtualCableNotInstalledError() for structured errors.
	ErrVirtualCableNotInstalled = ewerrors.NewSentinel(ewerrors.ErrVirtualCableNotInstalled, "VB-Audio Virtual Cable not installed")
)

// NewDeviceNotFoundError creates a structured error for device not found.
// Includes context about which device was requested.
//
// Example:
//
//	err := audio.NewDeviceNotFoundError("Built-in Microphone")
//	fmt.Println(err.Format()) // Prints formatted error with suggestion
func NewDeviceNotFoundError(deviceName string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrDeviceNotFound, "audio device not found").
		WithContext("device_name", deviceName).
		WithSuggestion("Verify the device is connected and recognized by the system. Run 'list-devices' to see available devices.")
}

// NewDeviceBusyError creates a structured error for device in use.
// Includes context about which device is busy and optionally which process is using it.
//
// Example:
//
//	err := audio.NewDeviceBusyError("Built-in Microphone", "Zoom")
func NewDeviceBusyError(deviceName string, usedBy string) *ewerrors.EchoWarpError {
	err := ewerrors.NewError(ewerrors.ErrDeviceBusy, "audio device is in use").
		WithContext("device_name", deviceName)

	if usedBy != "" {
		err = err.WithContext("used_by", usedBy)
	}

	return err.WithSuggestion("Close other applications using this audio device before starting EchoWarp.")
}

// NewFormatUnsupportedError creates a structured error for unsupported audio format.
// Includes context about the requested format and device capabilities.
//
// Example:
//
//	err := audio.NewFormatUnsupportedError(96000, 2, "Built-in Microphone")
func NewFormatUnsupportedError(sampleRate int, channels int, deviceName string) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrFormatUnsupported, "audio format not supported by device").
		WithContext("requested_sample_rate", sampleRate).
		WithContext("requested_channels", channels).
		WithContext("device_name", deviceName).
		WithSuggestion("Try a standard format like 48000 Hz, 2 channels (stereo).")
}

// NewBlackHoleNotInstalledError creates a structured error for missing BlackHole driver (macOS).
//
// Example:
//
//	err := audio.NewBlackHoleNotInstalledError()
func NewBlackHoleNotInstalledError() *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrBlackHoleNotInstalled, "BlackHole virtual audio device not installed").
		WithSuggestion("Install BlackHole from https://existential.audio/blackhole/ for virtual microphone functionality on macOS.")
}

// NewVirtualCableNotInstalledError creates a structured error for missing VB-Audio Virtual Cable (Windows).
//
// Example:
//
//	err := audio.NewVirtualCableNotInstalledError()
func NewVirtualCableNotInstalledError() *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrVirtualCableNotInstalled, "VB-Audio Virtual Cable not installed").
		WithSuggestion("Install VB-Audio Virtual Cable from https://vb-audio.com/Cable/ for virtual microphone functionality on Windows.")
}

// NewOpusEncodeError creates a structured error for Opus encoding failures.
// Includes context about the encoding parameters and underlying cause.
//
// Example:
//
//	err := audio.NewOpusEncodeError(48000, 2, underlyingError)
func NewOpusEncodeError(sampleRate int, channels int, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrOpusEncode, "Opus encoding failed").
		WithContext("sample_rate", sampleRate).
		WithContext("channels", channels).
		WithSuggestion("Check audio format compatibility and encoder configuration.")
}

// NewOpusDecodeError creates a structured error for Opus decoding failures.
// Includes context about the decoding parameters and underlying cause.
//
// Example:
//
//	err := audio.NewOpusDecodeError(48000, 2, underlyingError)
func NewOpusDecodeError(sampleRate int, channels int, cause error) *ewerrors.EchoWarpError {
	return ewerrors.Wrap(cause, ewerrors.ErrOpusDecode, "Opus decoding failed").
		WithContext("sample_rate", sampleRate).
		WithContext("channels", channels).
		WithSuggestion("The received audio data may be corrupted. Check network connection.")
}

// NewBufferOverflowError creates a structured error for audio buffer overflow.
// Includes context about buffer size and current state.
//
// Example:
//
//	err := audio.NewBufferOverflowError(4096, 0.95)
func NewBufferOverflowError(bufferSize int, fillRatio float64) *ewerrors.EchoWarpError {
	return ewerrors.NewError(ewerrors.ErrBufferOverflow, "audio buffer overflow").
		WithContext("buffer_size", bufferSize).
		WithContext("fill_ratio", fillRatio).
		WithSuggestion("The system cannot process audio data fast enough. Try reducing audio quality or check CPU usage.")
}
