package startup

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func fillSavedDevices(devices []config.DeviceEntry, inventory []audio.AudioDevice, saved *recent.Server, mode string, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	missing := missingRoles(devices, roles)
	if saved == nil || len(missing) == 0 {
		return devices, nil
	}
	preset, ok := saved.Presets[mode]
	if !ok {
		return devices, nil
	}
	if err := safePresetLifecycle(preset); err != nil {
		return nil, err
	}
	restored, err := restoreDevices(preset.Devices, inventory, missing)
	return append(devices, restored...), err
}

func restoreDevices(saved []recent.PresetDevice, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	devices := make([]config.DeviceEntry, 0, len(saved))
	for _, device := range saved {
		if !slices.Contains(roles, roleForInput(device.IsInput)) {
			continue
		}
		entry, err := restoreDevice(device, inventory)
		if err != nil {
			return nil, err
		}
		devices = append(devices, entry)
	}
	return devices, nil
}

func restoreDevice(saved recent.PresetDevice, inventory []audio.AudioDevice) (config.DeviceEntry, error) {
	if strings.TrimSpace(saved.Name) == "" {
		return config.DeviceEntry{}, errors.New("saved device has no name; numeric IDs cannot be restored safely")
	}
	if saved.MixInputID != nil && strings.TrimSpace(saved.MixInputName) == "" {
		return config.DeviceEntry{}, errors.New("saved mix input has no name; numeric IDs cannot be restored safely")
	}
	entry := config.DeviceEntry{
		Name: saved.Name, Role: roleForInput(saved.IsInput), Volume: saved.Volume, AGC: saved.AGC,
		MixInputID: copyID(saved.MixInputID), MixInputName: saved.MixInputName,
	}
	return resolveEntry(entry, inventory, []config.DeviceRole{entry.Role})
}

func safePresetLifecycle(preset recent.DevicePreset) error {
	for _, sink := range preset.VirtualSinks {
		if err := safeSinkLifecycle(sink); err != nil {
			return err
		}
	}
	for _, device := range preset.Devices {
		if device.VirtualSink != nil {
			if err := safeSinkLifecycle(*device.VirtualSink); err != nil {
				return err
			}
		}
	}
	return nil
}

func safeSinkLifecycle(sink recent.VirtualSinkPreset) error {
	switch sink.OnStart {
	case "", recent.SinkKeep:
		return nil
	default:
		return fmt.Errorf("virtual sink %q requires unsafe startup action %q; use manual setup", sink.SinkName, sink.OnStart)
	}
}
