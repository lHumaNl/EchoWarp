package startup

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func fillModeDevices(cfg *config.Config, devices []config.DeviceEntry, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	if cfg.Mode != config.ModeClient || cfg.ClientModes[cfg.AudioMode()] == nil {
		return devices, nil
	}
	modeConfig, err := clientModeConfig(*cfg)
	if err != nil {
		return nil, err
	}
	// Apply non-device mode settings using the existing config contract.
	cfg.ApplyClientModeData(cfg.ClientModes[cfg.AudioMode()])
	cfg.Devices = devices
	return mergeModeDevices(modeConfig, devices, inventory, roles)
}

func mergeModeDevices(modeConfig config.Config, devices []config.DeviceEntry, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	missing := missingRoles(devices, roles)
	entries, err := selectModeEntries(modeConfig.Devices, inventory, roles, missing)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveEntries(entries, inventory, roles)
	if err != nil {
		return nil, err
	}
	return completeLegacyDevices(append(devices, resolved...), modeConfig, inventory, roles)
}

func clientModeConfig(cfg config.Config) (config.Config, error) {
	data, err := yaml.Marshal(cfg.ClientModes[cfg.AudioMode()])
	if err != nil {
		return config.Config{}, fmt.Errorf("invalid %s client mode settings: %w", cfg.AudioMode(), err)
	}
	var mode struct {
		Audio config.Config `yaml:"audio"`
	}
	if err = yaml.Unmarshal(data, &mode); err != nil {
		return config.Config{}, fmt.Errorf("invalid %s client mode settings: %w", cfg.AudioMode(), err)
	}
	return mode.Audio, nil
}

func selectModeEntries(entries []config.DeviceEntry, inventory []audio.AudioDevice, roles, missing []config.DeviceRole) ([]config.DeviceEntry, error) {
	selected := make([]config.DeviceEntry, 0, len(entries))
	if len(missing) == 0 {
		return selected, nil
	}
	for _, entry := range entries {
		role, err := entryRole(entry, inventory, roles)
		if err != nil {
			return nil, fmt.Errorf("client mode device: %w", err)
		}
		if slices.Contains(missing, role) {
			entry.Role = role
			selected = append(selected, entry)
		}
	}
	return selected, nil
}
