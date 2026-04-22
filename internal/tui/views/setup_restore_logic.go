package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	presetpkg "github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// startProbeIfReady returns a debounced probe command if address and port are filled.
func (m *SetupModel) startProbeIfReady() tea.Cmd {
	var addr, port string
	for _, f := range m.Fields {
		if f.Key == "server_address" {
			addr = f.Value
		}
		if f.Key == "port" {
			port = f.Value
		}
	}
	if addr == "" || port == "" {
		return nil
	}
	m.probeAddr = addr + ":" + port
	m.probeStatus = "probing"
	m.probeError = ""
	for i := range m.Fields {
		if m.Fields[i].Key == "server_address" {
			m.Fields[i].Hint = "⠋ probing server..."
		}
	}
	return StartProbeDebounceCmd(addr, port)
}

// tryShowRestoreOverlay checks if a device preset exists for the selected server + mode,
// and auto-restores matching devices. Shows overlay only for missing virtual devices.
func (m *SetupModel) tryShowRestoreOverlay(addr string, port int, mode string) tea.Cmd {
	if m.presetDismissed[mode] {
		return nil
	}

	// Find preset for this server + mode
	for _, rs := range m.recentServers {
		if rs.Address != addr || rs.Port != port {
			continue
		}
		if rs.Presets == nil {
			return nil
		}
		preset, ok := rs.Presets[mode]
		if !ok || len(preset.Devices) == 0 {
			return nil
		}

		// Skip if current selection already matches the preset
		if m.currentSelectionMatchesPreset(preset) {
			return nil
		}

		return m.autoRestore(preset, mode)
	}
	return nil
}

// tryShowServerRestoreOverlay checks if a server device preset exists for the current mode,
// and auto-restores matching devices. Shows overlay only for missing virtual devices.
func (m *SetupModel) tryShowServerRestoreOverlay() tea.Cmd {
	if m.serverPresets == nil {
		return nil
	}

	// Determine current mode from field value
	mode := ""
	for _, f := range m.Fields {
		if f.Key == "mode" {
			mode = modeKeyFromValue(f.Value)
			break
		}
	}
	if mode == "" {
		return nil
	}

	if m.presetDismissed[mode] {
		return nil
	}

	p := m.serverPresets.Get(mode)
	if p == nil || len(p.Devices) == 0 {
		return nil
	}

	devPreset := recent.DevicePreset{Devices: p.Devices}
	// Skip if current selection already matches the preset
	if m.currentSelectionMatchesPreset(devPreset) {
		return nil
	}

	return m.autoRestore(devPreset, mode)
}

// autoRestore silently restores matched devices and shows overlay only for missing virtual devices.
func (m *SetupModel) autoRestore(preset recent.DevicePreset, mode string) tea.Cmd {
	// Auto-create virtual sinks with OnStart == SinkRecreate before matching.
	for _, pd := range preset.Devices {
		if pd.VirtualSink == nil || pd.VirtualSink.OnStart != recent.SinkRecreate {
			continue
		}
		// Check if already present by name.
		sinkLower := strings.ToLower(pd.VirtualSink.SinkName)
		alreadyExists := false
		allDevs := append(append([]deviceRow{}, m.inputDevices...), m.outputDevices...)
		for _, d := range allDevs {
			if strings.Contains(strings.ToLower(d.Name), sinkLower) {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			continue
		}
		// Create the virtual sink.
		moduleID, err := createPulseAudioSink(pd.VirtualSink.SinkName)
		if err == nil {
			m.virtualMicCreated = true
			m.virtualMicModule = moduleID
			// Wait for PulseAudio and refresh device list.
			time.Sleep(200 * time.Millisecond)
			m.refreshDevicesFromOS()
			m.rebuildDeviceGroups()
		}
	}

	matched, unmatched := matchPresetDevices(preset, m.inputDevices, m.outputDevices)

	if len(matched) == 0 && len(unmatched) == 0 {
		return nil
	}

	// Separate unmatched into virtual and non-virtual
	var unmatchedVirtual []recent.PresetDevice
	var unmatchedNonVirtual []string
	unmatchedSet := make(map[string]bool, len(unmatched))
	for _, name := range unmatched {
		unmatchedSet[name] = true
	}
	for _, pd := range preset.Devices {
		if unmatchedSet[pd.Name] {
			if pd.Virtual {
				unmatchedVirtual = append(unmatchedVirtual, pd)
			} else {
				unmatchedNonVirtual = append(unmatchedNonVirtual, pd.Name)
			}
		}
	}

	// Apply matched devices to multiSelect
	if m.multiSelect == nil {
		m.multiSelect = make(map[string]DeviceRoleSet)
	}
	for _, d := range matched {
		key := d.selectKey()
		role := m.multiSelect[key]
		if d.IsInput {
			role.Capture = true
		} else {
			role.Playback = true
		}
		m.multiSelect[key] = role
	}

	// Restore mix inputs from preset data
	m.restoreMixInputsFromPreset(preset, matched)

	// Restore volume and AGC from preset to deviceRow slices
	m.restoreVolumeAGCFromPreset(preset, matched)

	// Build flash message
	var restoredNames []string
	for _, d := range matched {
		restoredNames = append(restoredNames, d.Name)
	}

	// If there are missing virtual devices that need creation → show overlay
	if len(unmatchedVirtual) > 0 {
		// Flash for restored devices (if any) — short duration
		if len(restoredNames) > 0 {
			m.flashMsg = "Restored: " + strings.Join(restoredNames, ", ")
			m.flashTimer = time.Now().Add(3 * time.Second)
		}
		m.restoreOverlay = NewVirtualRestoreOverlay(unmatchedVirtual, mode)
		m.overlay = SetupOverlayRestore
		flashCmd := tea.Tick(3*time.Second, func(time.Time) tea.Msg { return FlashDismissMsg{} })
		if len(restoredNames) > 0 {
			return flashCmd
		}
		return nil
	}

	// No missing virtual devices — just flash
	flashDuration := 3 * time.Second
	if len(restoredNames) > 0 && len(unmatchedNonVirtual) > 0 {
		m.flashMsg = "Restored: " + strings.Join(restoredNames, ", ") +
			". ⚠ Not found: " + strings.Join(unmatchedNonVirtual, ", ")
		flashDuration = 5 * time.Second
	} else if len(restoredNames) > 0 {
		m.flashMsg = "Restored: " + strings.Join(restoredNames, ", ")
	} else if len(unmatchedNonVirtual) > 0 {
		m.flashMsg = "⚠ Not found: " + strings.Join(unmatchedNonVirtual, ", ")
		flashDuration = 5 * time.Second
	} else {
		return nil
	}

	m.flashTimer = time.Now().Add(flashDuration)
	return tea.Tick(flashDuration, func(time.Time) tea.Msg { return FlashDismissMsg{} })
}

// currentSelectionMatchesPreset returns true if the currently selected devices
// already match (or exceed) the preset — no need to show the restore overlay.
func (m *SetupModel) currentSelectionMatchesPreset(preset recent.DevicePreset) bool {
	if len(m.multiSelect) == 0 {
		return false
	}
	allDevices := append(append([]deviceRow{}, m.inputDevices...), m.outputDevices...)
	for _, pd := range preset.Devices {
		found := false
		for _, d := range allDevices {
			if d.Name == pd.Name && d.IsInput == pd.IsInput {
				if _, selected := m.multiSelect[d.selectKey()]; selected {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// applyRestore selects matched devices from the preset into the multiSelect map.
// If skipVirtual is true, virtual devices are excluded.
// Returns a tea.Cmd for flash messages about unmatched devices.
func (m *SetupModel) applyRestore(preset recent.DevicePreset, skipVirtual bool) tea.Cmd {
	filteredPreset := preset
	if skipVirtual {
		var filtered []recent.PresetDevice
		for _, d := range preset.Devices {
			if !d.Virtual {
				filtered = append(filtered, d)
			}
		}
		filteredPreset = recent.DevicePreset{Devices: filtered}
	}

	matched, unmatched := matchPresetDevices(filteredPreset, m.inputDevices, m.outputDevices)

	if m.multiSelect == nil {
		m.multiSelect = make(map[string]DeviceRoleSet)
	}

	for _, d := range matched {
		key := d.selectKey()
		role := m.multiSelect[key]
		if d.IsInput {
			role.Capture = true
		} else {
			role.Playback = true
		}
		m.multiSelect[key] = role
	}

	// Restore mix inputs from preset data
	m.restoreMixInputsFromPreset(filteredPreset, matched)

	// Restore volume and AGC from preset to deviceRow slices
	m.restoreVolumeAGCFromPreset(filteredPreset, matched)

	if len(unmatched) > 0 {
		m.flashMsg = "⚠ Device '" + unmatched[0] + "' not found, skipped"
		if len(unmatched) > 1 {
			m.flashMsg = fmt.Sprintf("⚠ %d devices not found, skipped", len(unmatched))
		}
		m.flashTimer = time.Now().Add(3 * time.Second)
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return FlashDismissMsg{} })
	}
	return nil
}

// restoreLastMode applies the saved top-level last_mode to the Mode field, if set.
// last_mode is the canonical English key ("normal"/"reverse"/"duplex"/"conference");
// it is reverse-looked-up against the current locale's Mode options.
func (m *SetupModel) restoreLastMode(lastMode string) {
	if lastMode == "" {
		return
	}
	for i := range m.Fields {
		if m.Fields[i].Key != "mode" {
			continue
		}
		for _, opt := range m.Fields[i].Options {
			if modeKeyFromValue(opt) == lastMode {
				m.Fields[i].SetValue(opt, SourceConfig)
				break
			}
		}
		break
	}
}

// restoreModePreset applies a per-mode preset snapshot to the TUI fields.
// All server fields (port, password, max_clients, tls*) are sourced from the
// ModePreset; fields whose values are zero in the preset are left untouched
// so the user's current in-flight values survive when a preset is only
// partially populated.
func (m *SetupModel) restoreModePreset(mp presetpkg.ModePreset) {
	setIfNonEmpty := func(fields []SetupField, key, value string) {
		if value == "" {
			return
		}
		for i := range fields {
			if fields[i].Key == key {
				fields[i].SetValue(value, SourceConfig)
				return
			}
		}
	}

	// Apply per-mode defaults for fields that are zero in the stored preset —
	// these are omitted on Save when they match DefaultsFor(mode), so we must
	// re-hydrate them here. Otherwise conference starts with max_clients=1
	// from cfg defaults instead of the mode default 2.
	currentMode := ""
	for _, f := range m.Fields {
		if f.Key == "mode" {
			currentMode = modeKeyFromValue(f.Value)
			break
		}
	}
	if currentMode == "" {
		currentMode = "normal"
	}
	d := presetpkg.DefaultsFor(currentMode)
	if mp.Port == 0 {
		mp.Port = d.Port
	}
	if mp.MaxClients == 0 {
		mp.MaxClients = d.MaxClients
	}

	// Use SourceDefault when the restored value matches the mode default, so
	// unchanged fields don't get a ✓ user-set marker (symmetric with
	// loadPresetForMode).
	setFieldWithDefault := func(fields []SetupField, key, value, defaultValue string) {
		for i := range fields {
			if fields[i].Key == key {
				src := SourceConfig
				if value == defaultValue {
					src = SourceDefault
				}
				fields[i].SetValue(value, src)
				return
			}
		}
	}
	setFieldWithDefault(m.Fields, "port", fmt.Sprintf("%d", mp.Port), fmt.Sprintf("%d", d.Port))
	setIfNonEmpty(m.Fields, "password", mp.Password)
	setFieldWithDefault(m.Fields, "max_clients", fmt.Sprintf("%d", mp.MaxClients), fmt.Sprintf("%d", d.MaxClients))

	if mp.TLS {
		setIfNonEmpty(m.AdvancedFields, "tls", "on")
	}
	setIfNonEmpty(m.AdvancedFields, "tls_cert", mp.TLSCert)
	setIfNonEmpty(m.AdvancedFields, "tls_key", mp.TLSKey)
	// log_level: compare to the "info" default — use SourceDefault so the UI
	// doesn't mark an unchanged value with a ✓. Symmetric with loadPresetForMode.
	if mp.LogLevel != "" {
		src := SourceConfig
		if mp.LogLevel == "info" {
			src = SourceDefault
		}
		for i := range m.AdvancedFields {
			if m.AdvancedFields[i].Key == "log_level" {
				m.AdvancedFields[i].SetValue(mp.LogLevel, src)
				break
			}
		}
	}

	m.applyFieldDependencies()
}

// loadPresetForMode applies the preset for the given mode if one exists, otherwise
// resets server fields to DefaultsFor(mode). Called on explicit mode switches in the
// TUI so each mode behaves as a self-contained snapshot.
func (m *SetupModel) loadPresetForMode(mode string) {
	var mp presetpkg.ModePreset
	if m.serverPresets != nil {
		if p := m.serverPresets.Get(mode); p != nil {
			mp = *p
		}
	}
	d := presetpkg.DefaultsFor(mode)
	if mp.Port == 0 {
		mp.Port = d.Port
	}
	if mp.MaxClients == 0 {
		mp.MaxClients = d.MaxClients
	}
	// Reset fields to preset values. Use SourceDefault when the value matches
	// the mode default so the UI doesn't show a ✓ or * marker on unchanged fields.
	setField := func(fields []SetupField, key, value, defaultValue string) {
		for i := range fields {
			if fields[i].Key == key {
				src := SourceConfig
				if value == defaultValue {
					src = SourceDefault
				}
				fields[i].SetValue(value, src)
				return
			}
		}
	}
	setField(m.Fields, "port", fmt.Sprintf("%d", mp.Port), fmt.Sprintf("%d", d.Port))
	setField(m.Fields, "password", mp.Password, "")
	setField(m.Fields, "max_clients", fmt.Sprintf("%d", mp.MaxClients), fmt.Sprintf("%d", d.MaxClients))
	tlsVal := "off"
	if mp.TLS {
		tlsVal = "on"
	}
	setField(m.AdvancedFields, "tls", tlsVal, "off")
	setField(m.AdvancedFields, "tls_cert", mp.TLSCert, "")
	setField(m.AdvancedFields, "tls_key", mp.TLSKey, "")
	// Always reset log_level on mode switch — if the target mode's preset has
	// no log_level saved, fall back to the "info" default instead of leaking
	// the previous mode's value.
	logLevel := mp.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	setField(m.AdvancedFields, "log_level", logLevel, "info")

	m.applyFieldDependencies()
}
