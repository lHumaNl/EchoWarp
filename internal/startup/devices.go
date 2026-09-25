package startup

import (
	"fmt"
	"slices"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func prepareDevices(cfg *config.Config, inventory []audio.AudioDevice, saved *recent.Server, roles []config.DeviceRole) error {
	devices, err := configuredDevices(*cfg, inventory, roles)
	if err != nil {
		return err
	}
	if devices, err = fillModeDevices(cfg, devices, inventory, roles); err != nil {
		return err
	}
	if devices, err = fillSavedDevices(devices, inventory, saved, cfg.AudioMode(), roles); err != nil {
		return err
	}
	if err := validateSelections(devices, roles); err != nil {
		return err
	}
	normalizeDevices(cfg, devices)
	return nil
}

func configuredDevices(cfg config.Config, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	devices, err := resolveEntries(cfg.Devices, inventory, roles)
	if err != nil {
		return nil, err
	}
	return completeLegacyDevices(devices, cfg, inventory, roles)
}

func completeLegacyDevices(devices []config.DeviceEntry, cfg config.Config, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	devices, err := fillLegacyRoles(devices, cfg, inventory, roles)
	if err != nil || cfg.DeviceID == nil || len(missingRoles(devices, roles)) == 0 {
		return devices, err
	}
	return fillLegacyDevice(devices, *cfg.DeviceID, inventory, roles)
}

func fillLegacyDevice(devices []config.DeviceEntry, id uint32, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	entry := config.DeviceEntry{ID: id, Volume: defaultVolume}
	resolved, err := resolveEntry(entry, inventory, roles)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(missingRoles(devices, roles), resolved.Role) {
		return devices, nil
	}
	return append(devices, resolved), nil
}

func fillLegacyRoles(devices []config.DeviceEntry, cfg config.Config, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	for _, role := range missingRoles(devices, roles) {
		id := cfg.InputDeviceID
		if role == config.RolePlayback {
			id = cfg.OutputDeviceID
		}
		if id == nil {
			continue
		}
		entry, err := resolveEntry(config.DeviceEntry{ID: *id, Role: role, Volume: defaultVolume}, inventory, roles)
		if err != nil {
			return nil, err
		}
		devices = append(devices, entry)
	}
	return devices, nil
}

func resolveEntries(entries []config.DeviceEntry, inventory []audio.AudioDevice, roles []config.DeviceRole) ([]config.DeviceEntry, error) {
	devices := make([]config.DeviceEntry, 0, len(entries))
	for _, entry := range entries {
		resolved, err := resolveEntry(entry, inventory, roles)
		if err != nil {
			return nil, err
		}
		devices = append(devices, resolved)
	}
	return devices, nil
}

func resolveEntry(entry config.DeviceEntry, inventory []audio.AudioDevice, roles []config.DeviceRole) (config.DeviceEntry, error) {
	role, err := entryRole(entry, inventory, roles)
	if err != nil {
		return entry, err
	}
	device, err := uniqueDevice(entry.ID, entry.Name, role, inventory)
	if err != nil {
		return entry, err
	}
	entry.ID, entry.Name, entry.Role = device.ID, device.Name, role
	entry.Type = deviceType(role)
	return resolveMixInput(entry, inventory)
}

func entryRole(entry config.DeviceEntry, inventory []audio.AudioDevice, roles []config.DeviceRole) (config.DeviceRole, error) {
	role, err := declaredRole(entry)
	if err != nil {
		return "", err
	}
	if role == "" && len(roles) == 1 {
		role = roles[0]
	}
	if role == "" {
		return inferRole(entry, inventory, roles)
	}
	if !slices.Contains(roles, role) {
		return "", fmt.Errorf("device %q has unexpected %s role", entry.Name, role)
	}
	return role, nil
}

func declaredRole(entry config.DeviceEntry) (config.DeviceRole, error) {
	role := entry.Role
	if role != "" && role != config.RoleCapture && role != config.RolePlayback {
		return "", fmt.Errorf("invalid device role %q", role)
	}
	if entry.Type == "" {
		return role, nil
	}
	typedRole := roleForInput(entry.Type == config.DeviceInput)
	if entry.Type != config.DeviceInput && entry.Type != config.DeviceOutput {
		return "", fmt.Errorf("invalid device type %q", entry.Type)
	}
	if role != "" && role != typedRole {
		return "", fmt.Errorf("device %q has conflicting role and type", entry.Name)
	}
	return typedRole, nil
}

func inferRole(entry config.DeviceEntry, inventory []audio.AudioDevice, roles []config.DeviceRole) (config.DeviceRole, error) {
	var matches []config.DeviceRole
	for _, role := range roles {
		for _, device := range inventory {
			if matchesDevice(device, entry.ID, entry.Name, role) {
				matches = append(matches, role)
			}
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("device %q (ID %d) has missing or ambiguous direction; specify capture or playback", entry.Name, entry.ID)
	}
	return matches[0], nil
}

func uniqueDevice(id uint32, name string, role config.DeviceRole, inventory []audio.AudioDevice) (audio.AudioDevice, error) {
	var found audio.AudioDevice
	count := 0
	for _, device := range inventory {
		if matchesDevice(device, id, name, role) {
			found = device
			count++
		}
	}
	if count != 1 {
		return found, fmt.Errorf("%s device %q (ID %d): expected one match, found %d (missing or ambiguous)", role, name, id, count)
	}
	return found, nil
}

func matchesDevice(device audio.AudioDevice, id uint32, name string, role config.DeviceRole) bool {
	if device.IsInput != (role == config.RoleCapture) {
		return false
	}
	if name != "" {
		return device.Name == name
	}
	return device.ID == id
}

func resolveMixInput(entry config.DeviceEntry, inventory []audio.AudioDevice) (config.DeviceEntry, error) {
	if entry.MixInputID == nil && entry.MixInputName == "" {
		return entry, nil
	}
	var id uint32
	if entry.MixInputID != nil {
		id = *entry.MixInputID
	}
	device, err := uniqueDevice(id, entry.MixInputName, config.RoleCapture, inventory)
	if err != nil {
		return entry, fmt.Errorf("mix input: %w", err)
	}
	entry.MixInputID, entry.MixInputName = &device.ID, device.Name
	return entry, nil
}

func missingRoles(devices []config.DeviceEntry, roles []config.DeviceRole) []config.DeviceRole {
	missing := make([]config.DeviceRole, 0, len(roles))
	for _, role := range roles {
		if !slices.ContainsFunc(devices, func(d config.DeviceEntry) bool { return d.Role == role }) {
			missing = append(missing, role)
		}
	}
	return missing
}

func validateSelections(devices []config.DeviceEntry, roles []config.DeviceRole) error {
	if missing := missingRoles(devices, roles); len(missing) > 0 {
		return fmt.Errorf("missing required audio device roles: %v", missing)
	}
	type key struct {
		id   uint32
		role config.DeviceRole
	}
	seen := make(map[key]bool, len(devices))
	for _, device := range devices {
		k := key{device.ID, device.Role}
		if seen[k] {
			return fmt.Errorf("duplicate %s device %q (ID %d)", device.Role, device.Name, device.ID)
		}
		seen[k] = true
	}
	return nil
}

func normalizeDevices(cfg *config.Config, devices []config.DeviceEntry) {
	cfg.Devices = devices
	cfg.DeviceID, cfg.InputDeviceID, cfg.OutputDeviceID = nil, nil, nil
	for _, device := range devices {
		if device.Role == config.RoleCapture && cfg.InputDeviceID == nil {
			cfg.InputDeviceID = copyID(&device.ID)
		}
		if device.Role == config.RolePlayback && cfg.OutputDeviceID == nil {
			cfg.OutputDeviceID = copyID(&device.ID)
		}
	}
	cfg.DeviceID = copyID(cfg.OutputDeviceID)
	if primaryRole(*cfg) == config.RoleCapture {
		cfg.DeviceID = copyID(cfg.InputDeviceID)
	}
}

func roleForInput(input bool) config.DeviceRole {
	if input {
		return config.RoleCapture
	}
	return config.RolePlayback
}

func deviceType(role config.DeviceRole) config.DeviceType {
	if role == config.RoleCapture {
		return config.DeviceInput
	}
	return config.DeviceOutput
}
