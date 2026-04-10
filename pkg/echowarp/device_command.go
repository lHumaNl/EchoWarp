package echowarp

// DeviceAction enumerates the supported device control actions that can be
// dispatched through Runner.HandleDeviceCommand.
//
// The values intentionally mirror internal/app.DeviceAction so adapters in the
// internal layer can perform a trivial 1:1 translation without a type switch.
// This type is defined in pkg/echowarp to keep the public API free of any
// internal package dependencies — see runner.go / DeviceCommandReceiver.
type DeviceAction int

const (
	// DeviceActionSetMute sets the mute state of a specific device to an
	// absolute value (not a toggle).
	DeviceActionSetMute DeviceAction = iota
	// DeviceActionSetVolume sets the volume multiplier of a specific device
	// to an absolute value in the range 0.0–1.5.
	DeviceActionSetVolume
)

// DeviceCommand is a public, transport-agnostic command for controlling a
// single audio device managed by a running Runner. It is the payload passed
// to DeviceCommandReceiver.HandleDeviceCommand.
//
// DeviceID uses int (signed) to allow callers to pass an API-layer id without
// worrying about overflow on the wire. Negative values are rejected by Node
// before reaching the runner.
type DeviceCommand struct {
	// Action specifies which operation to perform.
	Action DeviceAction
	// DeviceID is the target audio device id (must be >= 0).
	DeviceID int
	// Muted is consumed when Action == DeviceActionSetMute.
	Muted bool
	// Volume is consumed when Action == DeviceActionSetVolume. Valid range 0.0–1.5.
	Volume float64
}
