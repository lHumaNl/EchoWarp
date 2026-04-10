package app

import (
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// deviceCommandEnqueueTimeout bounds how long HandleDeviceCommand will block
// while trying to push a command onto the internal channel. It must be short
// enough that an HTTP API caller does not perceive a stall when the channel
// consumer goroutine is slow or not yet started.
const deviceCommandEnqueueTimeout = 100 * time.Millisecond

// translateDeviceCommand converts a public echowarp.DeviceCommand into the
// internal DeviceCommand format used by device_control.go / HandleDeviceCommands.
//
// Absolute-value operations (SetMute / SetVolume) coming from the API do not
// have a direct 1:1 mapping to the existing toggle-based DeviceAction enum
// (which was designed for TUI keystrokes). We preserve the absolute semantics
// on the wire by carrying Muted / Volume on the returned command; the consumer
// (task 013's HandleDeviceCommands) is responsible for honoring them.
//
// For backwards compatibility with the existing toggle-based consumer, the
// Action field is set to the closest matching toggle primitive:
//   - SetMute  → DeviceToggleMute  (consumer should treat as absolute when
//     Muted is meaningful; see TODO below)
//   - SetVolume → DeviceVolumeUp   (placeholder; consumer should honor the
//     absolute Volume field carried on the command)
//
// TODO(task-013): extend internal DeviceCommand with absolute mute/volume
// fields and update HandleDeviceCommands to honor them; until then, API
// calls reach the channel but the current drainDeviceCommands consumer only
// logs them. See .tasks/013-volume-mute-controls.md.
func translateDeviceCommand(cmd echowarp.DeviceCommand) DeviceCommand {
	internal := DeviceCommand{
		DeviceID: uint32(cmd.DeviceID),
	}
	switch cmd.Action {
	case echowarp.DeviceActionSetMute:
		internal.Action = DeviceToggleMute
	case echowarp.DeviceActionSetVolume:
		// Volume up/down are both toggles; the absolute value is not yet
		// consumed by the internal pipeline. We deliberately pick VolumeUp
		// as a placeholder so the command is still uniquely identifiable
		// in log output (see drainDeviceCommands).
		internal.Action = DeviceVolumeUp
	}
	return internal
}

// HandleDeviceCommand implements echowarp.DeviceCommandReceiver. It enqueues
// the command onto the ServerApp's internal device command channel with a
// short timeout to avoid blocking the caller (typically an HTTP handler).
//
// Returns an ErrNotRunning error if the channel is not initialized (which
// only happens for zero-valued ServerApps constructed outside NewServerApp)
// and an ErrBufferOverflow error if the channel remains full for longer than
// deviceCommandEnqueueTimeout.
func (s *ServerApp) HandleDeviceCommand(cmd echowarp.DeviceCommand) error {
	if s.deviceCmdCh == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Server device command channel not initialized").
			WithSuggestion("Construct ServerApp via NewServerApp")
	}
	internal := translateDeviceCommand(cmd)
	timer := time.NewTimer(deviceCommandEnqueueTimeout)
	defer timer.Stop()
	select {
	case s.deviceCmdCh <- internal:
		return nil
	case <-timer.C:
		return ewerrors.NewError(ewerrors.ErrBufferOverflow, "Device command queue full").
			WithContext("device_id", cmd.DeviceID).
			WithSuggestion("Reduce command rate or check that the consumer goroutine is running")
	}
}

// HandleDeviceCommand implements echowarp.DeviceCommandReceiver for ClientApp.
// See the ServerApp equivalent for semantics — the implementation is identical
// so a single runner type does not have to be aware of the mode.
func (c *ClientApp) HandleDeviceCommand(cmd echowarp.DeviceCommand) error {
	if c.deviceCmdCh == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Client device command channel not initialized").
			WithSuggestion("Construct ClientApp via NewClientApp")
	}
	internal := translateDeviceCommand(cmd)
	timer := time.NewTimer(deviceCommandEnqueueTimeout)
	defer timer.Stop()
	select {
	case c.deviceCmdCh <- internal:
		return nil
	case <-timer.C:
		return ewerrors.NewError(ewerrors.ErrBufferOverflow, "Device command queue full").
			WithContext("device_id", cmd.DeviceID).
			WithSuggestion("Reduce command rate or check that the consumer goroutine is running")
	}
}
