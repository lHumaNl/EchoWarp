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

	// Skip if current selection already matches the preset
	if m.currentSelectionMatchesPreset(*p) {
		return nil
	}

	return m.autoRestore(*p, mode)
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

// restoreServerSettings applies saved server settings to the TUI fields.
// Fields that have a zero/empty value in the settings are skipped (backward compat).
func (m *SetupModel) restoreServerSettings(s presetpkg.ServerSettings) {
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

	if s.LastMode != "" {
		// Mode field stores LOCALIZED descriptive values (e.g. "normale (server → client)"
		// in Italian); s.LastMode is the canonical English key ("normal" / "reverse" /
		// "duplex" / "conference"). Match by reverse-lookup through modeKeyFromValue,
		// which returns the canonical key for a localized option in the current locale.
		// Silently skips restore when s.LastMode is not a known canonical key — this
		// gracefully handles legacy preset files written before the modeKeyFromValue
		// fix, which may contain localized values like "normale" or "thường".
		for i := range m.Fields {
			if m.Fields[i].Key != "mode" {
				continue
			}
			for _, opt := range m.Fields[i].Options {
				if modeKeyFromValue(opt) == s.LastMode {
					m.Fields[i].SetValue(opt, SourceConfig)
					break
				}
			}
			break
		}
	}

	if s.Port != 0 {
		setIfNonEmpty(m.Fields, "port", fmt.Sprintf("%d", s.Port))
	}
	setIfNonEmpty(m.Fields, "password", s.Password)
	if s.MaxClients != 0 {
		setIfNonEmpty(m.Fields, "max_clients", fmt.Sprintf("%d", s.MaxClients))
	}

	tlsVal := "off"
	if s.TLS {
		tlsVal = "on"
	}
	if s.TLS {
		setIfNonEmpty(m.AdvancedFields, "tls", tlsVal)
	}
	setIfNonEmpty(m.AdvancedFields, "tls_cert", s.TLSCert)
	setIfNonEmpty(m.AdvancedFields, "tls_key", s.TLSKey)

	m.applyFieldDependencies()
}
