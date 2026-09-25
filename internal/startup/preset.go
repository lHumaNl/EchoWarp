package startup

import (
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// PresetFromConfig converts resolved device selections into an owned preset.
// Roles are represented by IsInput in the existing recent schema. Resolve unnamed
// legacy selections with Prepare first: this conversion has no device inventory.
func PresetFromConfig(cfg config.Config) recent.DevicePreset {
	devices := cfg.Devices
	if len(devices) == 0 {
		devices = legacyPresetEntries(cfg)
	}
	preset := recent.DevicePreset{Devices: make([]recent.PresetDevice, 0, len(devices))}
	for _, device := range devices {
		role, _ := declaredRole(device)
		if role == "" {
			role = primaryRole(cfg)
		}
		preset.Devices = append(preset.Devices, presetDevice(device, role))
	}
	return preset
}

func presetDevice(device config.DeviceEntry, role config.DeviceRole) recent.PresetDevice {
	return recent.PresetDevice{
		ID: device.ID, Name: device.Name, IsInput: role == config.RoleCapture,
		Volume: device.Volume, VolumeSet: device.Volume == 0, AGC: device.AGC,
		MixInputID: copyID(device.MixInputID), MixInputName: device.MixInputName,
	}
}

func legacyPresetEntries(cfg config.Config) []config.DeviceEntry {
	var devices []config.DeviceEntry
	roles, _ := requiredRoles(cfg)
	for _, role := range roles {
		id := cfg.InputDeviceID
		if role == config.RolePlayback {
			id = cfg.OutputDeviceID
		}
		if id != nil {
			devices = append(devices, config.DeviceEntry{ID: *id, Role: role, Volume: defaultVolume})
		}
	}
	if len(devices) == 0 && len(roles) > 0 && cfg.DeviceID != nil {
		devices = append(devices, config.DeviceEntry{ID: *cfg.DeviceID, Role: primaryRole(cfg), Volume: defaultVolume})
	}
	return devices
}
