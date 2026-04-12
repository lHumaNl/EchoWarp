package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// listAllDevices enumerates all input and output audio devices via a DeviceManager.
// The caller must close dm after use.
func listAllDevices(dm *audio.MalgoDeviceManager) ([]audio.AudioDevice, error) {
	inputDevices, err := dm.ListInputDevices()
	if err != nil {
		return nil, fmt.Errorf("list input devices: %w", err)
	}
	outputDevices, outErr := dm.ListOutputDevices()
	if outErr != nil {
		return nil, fmt.Errorf("list output devices: %w", outErr)
	}
	devices := make([]audio.AudioDevice, 0, len(inputDevices)+len(outputDevices))
	devices = append(devices, inputDevices...)
	devices = append(devices, outputDevices...)
	return devices, nil
}

// appendLoopbackDevices discovers loopback devices and appends them to the device list.
// Returns the updated list and a loopback lookup map (device name → LoopbackDevice).
// loadAllDevicesWithLoopback loads all input, output, and loopback devices for the TUI.
func loadAllDevicesWithLoopback(dm *audio.MalgoDeviceManager) ([]audio.AudioDevice, error) {
	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to list input devices: %w", err)
	}
	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to list output devices: %w", err)
	}
	inputs = append(inputs, outputs...)
	devices := inputs
	// Inject loopback devices (output→input capture)
	devices, _ = appendLoopbackDevices(dm, devices, true)
	if len(devices) == 0 {
		return nil, fmt.Errorf("no audio devices found")
	}
	return devices, nil
}

func appendLoopbackDevices(dm *audio.MalgoDeviceManager, devices []audio.AudioDevice, needsLoopback bool) ([]audio.AudioDevice, map[string]audio.LoopbackDevice) {
	loopbackMap := make(map[string]audio.LoopbackDevice)
	if !needsLoopback {
		return devices, loopbackMap
	}
	loopbackDevices, _ := audio.ListLoopbackDevices(dm)
	for _, lb := range loopbackDevices {
		name := lb.OutputDevice.Name + " [loopback]"
		devices = append(devices, audio.AudioDevice{
			ID:         lb.OutputDevice.ID,
			Name:       name,
			IsInput:    true,
			Channels:   lb.Channels,
			SampleRate: lb.OutputDevice.SampleRate,
		})
		loopbackMap[name] = lb
	}
	return devices, loopbackMap
}

// sendError sends an error to errCh without blocking. If the channel buffer is
// full the error is logged instead of being silently dropped.
func sendError(errCh chan<- error, err error, logger *slog.Logger) {
	if err == nil {
		return
	}
	select {
	case errCh <- err:
	default:
		if logger != nil {
			logger.Error("Error channel full, error dropped", "error", err)
		}
	}
}

// bridgeParticipantCommands forwards TUI participant commands to app commands.
// Exits when ctx is canceled or src is closed.
func bridgeParticipantCommands(ctx context.Context, src <-chan tui.ParticipantCommand, dst chan<- app.ParticipantCommand) {
	defer close(dst)
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-src:
			if !ok {
				return
			}
			select {
			case dst <- app.ParticipantCommand{
				Action:        app.ParticipantAction(cmd.Action),
				ParticipantID: cmd.ParticipantID,
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// bridgeRecordingCommands forwards TUI recording commands to app commands.
// Exits when ctx is canceled or src is closed.
func bridgeRecordingCommands(ctx context.Context, src <-chan tui.RecordingCommand, dst chan<- app.RecordingCommand) {
	defer close(dst)
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-src:
			if !ok {
				return
			}
			select {
			case dst <- app.RecordingCommand{
				Start:          cmd.Start,
				Mode:           audio.RecordingMode(cmd.Mode),
				LocalDeviceIDs: cmd.LocalDeviceIDs,
				RemoteIDs:      cmd.RemoteIDs,
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// bridgeConferenceStats forwards app conference stats to the TUI channel.
// Exits when ctx is canceled or src is closed.
func bridgeConferenceStats(ctx context.Context, src <-chan app.ConferenceStatsPayload, dst chan<- tui.ConferenceStatsPayload) {
	defer close(dst)
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-src:
			if !ok {
				return
			}
			select {
			case dst <- tui.ConferenceStatsPayload{
				States:             p.States,
				Recording:          p.Recording,
				PausedParticipants: p.PausedParticipants,
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// bridgeClientCommands forwards TUI client commands to app commands.
// Exits when ctx is canceled or src is closed.
func bridgeClientCommands(ctx context.Context, src <-chan tui.ClientCommand, dst chan<- app.ClientCommand) {
	defer close(dst)
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-src:
			if !ok {
				return
			}
			select {
			case dst <- app.ClientCommand{
				Action:      app.ClientAction(cmd.Action),
				ClientID:    cmd.ClientID,
				IP:          cmd.IP,
				Reason:      cmd.Reason,
				BanCriteria: cmd.BanCriteria,
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// bridgeDeviceCommands forwards TUI device commands to the app's internal device
// command channel. Exits when ctx is canceled or src is closed.
func bridgeDeviceCommands(ctx context.Context, src <-chan tui.DeviceCommand, dst chan<- app.DeviceCommand) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-src:
			if !ok {
				return
			}
			select {
			case dst <- app.DeviceCommand{Action: app.DeviceAction(cmd.Action), DeviceID: cmd.DeviceID}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// injectVirtualMicDevice detects a virtual audio device and appends it to cfg.Devices
// as a playback entry. Called for CLI (non-TUI) flows when --virtual-mic is set.
// Logs a warning and returns without error if no virtual device is found.
func injectVirtualMicDevice(cfg *config.Config, logger *slog.Logger) {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		logger.Warn("Virtual mic: cannot init audio device manager", "error", err)
		return
	}
	defer dm.Close() //nolint:errcheck

	name, found := audio.DetectVirtualDevice(dm)
	if !found {
		logger.Warn("Virtual mic: no virtual audio device found (install BlackHole or VB-Audio Virtual Cable)")
		return
	}

	// Find the device ID for the detected virtual device name.
	outputs, listErr := dm.ListOutputDevices()
	if listErr != nil {
		logger.Warn("Virtual mic: cannot list output devices", "error", listErr)
		return
	}
	for _, d := range outputs {
		if d.Name == name {
			cfg.Devices = append(cfg.Devices, config.DeviceEntry{
				ID:     d.ID,
				Name:   d.Name,
				Role:   config.RolePlayback,
				Volume: 1.0,
			})
			logger.Info("Virtual mic enabled", "device", name, "id", d.ID)
			return
		}
	}
	// Also check inputs (Linux pipe sources appear as inputs).
	inputs, listErr := dm.ListInputDevices()
	if listErr != nil {
		logger.Warn("Virtual mic: cannot list input devices", "error", listErr)
		return
	}
	for _, d := range inputs {
		if d.Name == name {
			cfg.Devices = append(cfg.Devices, config.DeviceEntry{
				ID:     d.ID,
				Name:   d.Name,
				Role:   config.RolePlayback,
				Volume: 1.0,
			})
			logger.Info("Virtual mic enabled", "device", name, "id", d.ID)
			return
		}
	}
	logger.Warn("Virtual mic: device detected by name but not found in device list", "device", name)
}
