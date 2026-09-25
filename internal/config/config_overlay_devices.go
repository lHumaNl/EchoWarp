package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Device arrays and legacy selector fields compete for the same role. Priority
// must be applied across those fields, not only independently within each key.
func applyDeviceSelectorPriority(cfg *Config, values map[string]yaml.Node) error {
	array := overlaySourceRank("devices", values)
	legacy := overlaySourceRank("device_id", values)
	input := overlaySourceRank("input_device_id", values)
	output := overlaySourceRank("output_device_id", values)
	if array > legacy {
		cfg.DeviceID = nil
	}
	if array > input {
		cfg.InputDeviceID = nil
	}
	if array > output {
		cfg.OutputDeviceID = nil
	}
	if legacy > array {
		cfg.Devices = nil
	}
	if legacy > input {
		cfg.InputDeviceID = nil
	}
	if legacy > output {
		cfg.OutputDeviceID = nil
	}
	for _, selector := range []struct {
		role DeviceRole
		rank int
		id   *uint32
	}{
		{RoleCapture, input, cfg.InputDeviceID}, {RolePlayback, output, cfg.OutputDeviceID},
	} {
		if selector.rank <= array || selector.rank < legacy {
			continue
		}
		if err := replaceLowerDeviceRole(cfg, selector.role, selector.id); err != nil {
			return err
		}
	}
	return nil
}

func replaceLowerDeviceRole(cfg *Config, role DeviceRole, id *uint32) error {
	kept := make([]DeviceEntry, 0, len(cfg.Devices)+1)
	replaced := false
	for _, device := range cfg.Devices {
		deviceRole := device.Role
		if deviceRole == "" {
			switch device.Type {
			case DeviceInput:
				deviceRole = RoleCapture
			case DeviceOutput:
				deviceRole = RolePlayback
			default:
				return fmt.Errorf("cannot override untyped device array safely; specify capture/playback roles")
			}
		}
		if deviceRole != role {
			kept = append(kept, device)
		} else {
			replaced = true
		}
	}
	// Without a competing array entry, leave the selector deferred: readiness
	// consults legacy IDs only for roles required by the authoritative mode.
	if replaced && id != nil {
		kept = append(kept, DeviceEntry{ID: *id, Role: role, Volume: 1})
	}
	cfg.Devices = kept
	return nil
}

func overlaySourceRank(key string, values map[string]yaml.Node) int {
	if _, ok := overlayEnv(key); ok {
		return 2
	}
	if _, ok := values[key]; ok {
		return 1
	}
	if alias := overlayAliases[key]; alias != "" {
		if _, ok := values[alias]; ok {
			return 1
		}
	}
	return 0
}
