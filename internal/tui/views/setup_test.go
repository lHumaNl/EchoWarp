package views

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

func newTestConfig(mode config.Mode) config.Config {
	return config.Config{
		Mode:                 mode,
		Port:                 4415,
		SampleRate:           48000,
		Channels:             1,
		MaxClients:           1,
		MaxReconnectAttempts: 5,
		MaxFailedAttempts:    5,
		AudioBufferFrames:    0,
		OpusBitrate:          64000,
		LogLevel:             "info",
	}
}

func newTestSetupModel(mode config.Mode) SetupModel {
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	cfg := newTestConfig(mode)
	isInput := mode == config.ModeServer
	return NewSetupModel(cfg, deviceList, isInput, 120, 40)
}

func TestNewSetupModelServer(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	assert.Equal(t, ColumnDevices, m.activeColumn)
	assert.True(t, m.IsInput)

	// Server should have: Port, Password, Max clients, Max auth fail, Echo cancellation, Advanced
	require.GreaterOrEqual(t, len(m.Fields), 6)

	keys := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		keys[i] = f.Key
	}
	assert.Contains(t, keys, "port")
	assert.Contains(t, keys, "password")
	assert.Contains(t, keys, "max_clients")
	assert.Contains(t, keys, "max_auth_fail")
	assert.Contains(t, keys, "echo_cancellation")

	assert.Contains(t, keys, "mode")
	// Server-specific: no Max reconnect in main (it's in advanced), no Server address
	assert.NotContains(t, keys, "max_reconnect")
	assert.NotContains(t, keys, "tls")
	assert.NotContains(t, keys, "server_address")
}

func TestNewSetupModelClient(t *testing.T) {
	m := newTestSetupModel(config.ModeClient)

	assert.False(t, m.IsInput) // client normal mode = output device

	keys := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		keys[i] = f.Key
	}
	assert.Contains(t, keys, "server_address")
	assert.Contains(t, keys, "port")
	assert.Contains(t, keys, "password")
	assert.Contains(t, keys, "max_reconnect")

	assert.NotContains(t, keys, "mode") // Mode removed from client settings (shown via probe)
	// Client-specific: no Max auth fail in main (it's in advanced), no Max clients
	assert.NotContains(t, keys, "max_auth_fail")
	assert.NotContains(t, keys, "tls")
	assert.NotContains(t, keys, "max_clients")
}

func TestSetupModelCLIPreFill(t *testing.T) {
	cfg := newTestConfig(config.ModeServer)
	cfg.Port = 8080
	cfg.Password = "secret"

	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, true, 120, 40)

	// Port should be CLI-sourced since it's non-default
	for _, f := range m.Fields {
		if f.Key == "port" {
			assert.Equal(t, SourceCLI, f.Source)
			assert.Equal(t, "8080", f.Value)
		}
		if f.Key == "password" {
			assert.Equal(t, SourceCLI, f.Source)
			assert.Equal(t, "secret", f.Value)
		}
	}
}

func TestSetupModelBuildConfig(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Change port via field
	for i := range m.Fields {
		if m.Fields[i].Key == "port" {
			m.Fields[i].SetValue("9090", SourceUser)
		}
	}
	for i := range m.Fields {
		if m.Fields[i].Key == "mode" {
			m.Fields[i].SetValue("reverse (client → server)", SourceUser)
		}
	}

	cfg := m.BuildConfig()
	assert.Equal(t, 9090, cfg.Port)
	assert.True(t, cfg.Reverse)
}

func TestSetupModelAdvancedFields(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	require.NotEmpty(t, m.AdvancedFields)

	keys := make([]string, len(m.AdvancedFields))
	for i, f := range m.AdvancedFields {
		keys[i] = f.Key
	}
	assert.Contains(t, keys, "tls")
	assert.Contains(t, keys, "sample_rate")
	assert.Contains(t, keys, "channels")
	assert.Contains(t, keys, "opus_bitrate")
	assert.NotContains(t, keys, "mode")          // Mode is in main fields
	assert.NotContains(t, keys, "max_auth_fail") // Max auth fail is in main fields
	assert.NotContains(t, keys, "max_reconnect") // not relevant for server
}

func TestSetupModelView(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	view := m.View()

	// Should contain column titles
	assert.Contains(t, view, "Select Input Device")
	assert.Contains(t, view, "Settings")

	// Should show settings validation hint (server has no required empty fields)
	assert.Contains(t, view, "Settings OK")
}

func TestSetupModelViewClient(t *testing.T) {
	m := newTestSetupModel(config.ModeClient)
	view := m.View()

	assert.Contains(t, view, "Select Output Device")
	assert.Contains(t, view, "Settings")

	// Client without server selected shows hint to select
	assert.Contains(t, view, "Select a server to start")
}

func TestSetupModelHelpKeys(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Default: devices column focused
	help := m.HelpKeys()
	assert.Contains(t, help, "filter")
	assert.Contains(t, help, "tab")

	// Switch to settings
	m.activeColumn = ColumnSettings
	help = m.HelpKeys()
	assert.Contains(t, help, "navigate")
	assert.Contains(t, help, "tab")
}

func TestSetupModelNarrowView(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	m.SetSize(60, 20)

	view := m.View()
	// Should render single column (width < 80)
	assert.NotEmpty(t, view)
}

func TestSetupModelFieldDependencies(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Initially TLS is off, so TLS Cert/Key should not be required
	for _, f := range m.AdvancedFields {
		if f.Key == "tls_cert" || f.Key == "tls_key" {
			assert.False(t, f.Required, "%s should not be required when TLS is off", f.Label)
			assert.Empty(t, f.Hint)
		}
	}

	// Turn TLS on (TLS is in AdvancedFields)
	for i := range m.AdvancedFields {
		if m.AdvancedFields[i].Key == "tls" {
			m.AdvancedFields[i].SetValue("on", SourceUser)
		}
	}
	m.applyFieldDependencies()

	// Now TLS Cert/Key should be required
	for _, f := range m.AdvancedFields {
		if f.Key == "tls_cert" || f.Key == "tls_key" {
			assert.True(t, f.Required, "%s should be required when TLS is on", f.Label)
			assert.Contains(t, f.Hint, "required")
		}
	}

	// Advanced should auto-open
	assert.True(t, m.advancedOpen)
}

func TestSetupModelAdvancedFieldsTLSCertKey(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	keys := make([]string, len(m.AdvancedFields))
	for i, f := range m.AdvancedFields {
		keys[i] = f.Key
	}
	assert.Contains(t, keys, "tls_cert")
	assert.Contains(t, keys, "tls_key")
}

func TestSetupModelOverlayHelpKeys(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	m.overlay = SetupOverlaySummary
	help := m.HelpKeys()
	assert.Contains(t, help, "start")
	assert.Contains(t, help, "copy cmd")

	m.overlay = SetupOverlayVirtualDevice
	help = m.HelpKeys() //nolint:ineffassign
	assert.Contains(t, help, "navigate")
}

// --- Mode switching tests ---

// setModeField sets the "Mode" field by value string and calls applyFieldDependencies.
func setModeField(m *SetupModel, value string) {
	for i := range m.Fields {
		if m.Fields[i].Key == "mode" {
			m.Fields[i].SetValue(value, SourceUser)
			break
		}
	}
	m.applyFieldDependencies()
}

func getFieldValue(fields []SetupField, key string) string {
	for _, f := range fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

func getFieldHidden(fields []SetupField, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return f.Hidden
		}
	}
	return false
}

func TestModeSwitchConferenceMaxClients(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Default: normal mode, Max clients = 1
	assert.Equal(t, "1", getFieldValue(m.Fields, "max_clients"))

	// Switch to conference → Max clients should auto-bump to 2
	setModeField(&m, "conference (multi-user)")
	assert.True(t, m.isConferenceMode)
	val := getFieldValue(m.Fields, "max_clients")
	assert.Equal(t, "2", val, "Conference should auto-bump Max clients to 2")

	// Switch back to normal → Max clients should restore to 1
	setModeField(&m, "normal (server → client)")
	assert.False(t, m.isConferenceMode)
	val = getFieldValue(m.Fields, "max_clients")
	assert.Equal(t, "1", val, "Leaving conference should restore Max clients to previous value")
}

func TestModeSwitchConferenceMaxClientsPreservesUserValue(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// User sets Max clients to 5
	for i := range m.Fields {
		if m.Fields[i].Key == "max_clients" {
			m.Fields[i].SetValue("5", SourceUser)
			break
		}
	}

	// Switch to conference (5 >= 2, so no bump needed)
	setModeField(&m, "conference (multi-user)")
	val := getFieldValue(m.Fields, "max_clients")
	assert.Equal(t, "5", val, "Conference should not change Max clients when already >= 2")

	// Switch back to normal → should stay 5 (no restore needed since it wasn't bumped)
	setModeField(&m, "normal (server → client)")
	val = getFieldValue(m.Fields, "max_clients")
	assert.Equal(t, "5", val, "Max clients should stay at user's value after leaving conference")
}

func TestModeSwitchEchoCancellationVisibility(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Normal mode: Echo cancellation should be hidden
	assert.True(t, getFieldHidden(m.Fields, "echo_cancellation"),
		"Echo cancellation should be hidden in normal mode")

	// Switch to duplex → should be visible
	setModeField(&m, "duplex (bidirectional)")
	assert.False(t, getFieldHidden(m.Fields, "echo_cancellation"),
		"Echo cancellation should be visible in duplex mode")

	// Switch to conference → should still be visible
	setModeField(&m, "conference (multi-user)")
	assert.False(t, getFieldHidden(m.Fields, "echo_cancellation"),
		"Echo cancellation should be visible in conference mode")

	// Switch back to normal → should be hidden again and reset to off
	setModeField(&m, "normal (server → client)")
	assert.True(t, getFieldHidden(m.Fields, "echo_cancellation"),
		"Echo cancellation should be hidden after switching back to normal")
	assert.Equal(t, "off", getFieldValue(m.Fields, "echo_cancellation"),
		"Echo cancellation should reset to off when hidden")
}

func TestModeSwitchRoundTrip(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Full round-trip: normal → duplex → conference → reverse → normal
	modes := []struct {
		value     string
		isDuplex  bool
		isConf    bool
		ecVisible bool
	}{
		{"normal (server → client)", false, false, false},
		{"duplex (bidirectional)", true, false, true},
		{"conference (multi-user)", true, true, true},
		{"reverse (client → server)", false, false, false},
		{"normal (server → client)", false, false, false},
	}

	for _, mode := range modes {
		setModeField(&m, mode.value)
		assert.Equal(t, mode.isDuplex, m.isDuplexMode, "isDuplexMode for %s", mode.value)
		assert.Equal(t, mode.isConf, m.isConferenceMode, "isConferenceMode for %s", mode.value)
		assert.Equal(t, mode.ecVisible, !getFieldHidden(m.Fields, "echo_cancellation"),
			"Echo cancellation visibility for %s", mode.value)
	}
}

func TestModeSwitchFieldIndicesStable(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Record field count and labels in normal mode
	normalLabels := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		normalLabels[i] = f.Label
	}

	// Switch to conference and back
	setModeField(&m, "conference (multi-user)")
	setModeField(&m, "normal (server → client)")

	// Field count and labels must be identical (hidden fields still occupy indices)
	require.Equal(t, len(normalLabels), len(m.Fields), "Field count changed after mode round-trip")
	for i, f := range m.Fields {
		assert.Equal(t, normalLabels[i], f.Label,
			"Field at index %d changed label after mode round-trip", i)
	}
}

func TestModeSwitchBuildConfigConference(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	setModeField(&m, "conference (multi-user)")
	cfg := m.BuildConfig()

	assert.True(t, cfg.Conference)
	assert.GreaterOrEqual(t, cfg.MaxClients, 2, "Conference requires at least 2 clients")
}

func TestModeSwitchBuildConfigNormalAfterConference(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	// Switch to conference then back
	setModeField(&m, "conference (multi-user)")
	setModeField(&m, "normal (server → client)")
	cfg := m.BuildConfig()

	assert.False(t, cfg.Conference)
	assert.False(t, cfg.Duplex)
	assert.False(t, cfg.Reverse)
	assert.Equal(t, 1, cfg.MaxClients, "Max clients should restore after leaving conference")
}

// --- Phase 3: server preset overlay tests ---

// mockDeviceItem implements the interfaces required by rebuildDeviceGroups.
type mockDeviceItem struct {
	name    string
	id      uint32
	isInput bool
}

func (d mockDeviceItem) Title() string       { return d.name }
func (d mockDeviceItem) Description() string { return "" }
func (d mockDeviceItem) FilterValue() string { return d.name }
func (d mockDeviceItem) DeviceID() uint32    { return d.id }
func (d mockDeviceItem) IsInputDevice() bool { return d.isInput }

// writeServerPresetsFile writes a server_presets.json to dir with the given presets map.
func writeServerPresetsFile(t *testing.T, dir string, presets map[string]recent.DevicePreset) {
	t.Helper()
	sp := preset.ServerPresets{Presets: presets}
	data, err := json.MarshalIndent(sp, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(dir+"/server_presets.json", data, 0600))
}

// newServerModelWithPresets creates a server SetupModel using a temp dir with preset data.
// After creation, it calls WithUnifiedDeviceList to populate devices and trigger overlay logic.
func newServerModelWithDevicesAndPresets(
	t *testing.T,
	tempDir string,
	devices []mockDeviceItem,
) SetupModel {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", tempDir)

	items := make([]list.Item, len(devices))
	for i, d := range devices {
		items[i] = d
	}
	dl := list.New(items, list.NewDefaultDelegate(), 80, 20)
	cfg := newTestConfig(config.ModeServer)
	m := NewSetupModel(cfg, dl, true, 120, 40)
	m = m.WithUnifiedDeviceList(false)
	return m
}

func TestServerTUI_WithPreset_AutoRestoredAtInit(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}}},
	})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
	})

	// Auto-restore: no overlay, device silently selected, flash message shown
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for non-virtual auto-restore")
	assert.Nil(t, m.restoreOverlay)
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	assert.True(t, micSelected, "Mic should be auto-restored")
	assert.Contains(t, m.flashMsg, "Restored")
	assert.Contains(t, m.flashMsg, "Mic")
}

func TestServerTUI_WithoutPreset_OverlayNotShown(t *testing.T) {
	dir := t.TempDir()
	// Write empty presets (no entry for "normal")
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
	})

	assert.Equal(t, SetupOverlayNone, m.overlay, "overlay should not be shown without a preset")
	assert.Nil(t, m.restoreOverlay)
}

func TestServerTUI_ModeChange_WithPreset_AutoRestored(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"reverse": {Devices: []recent.PresetDevice{{ID: 2, Name: "Speaker"}}},
	})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Speaker", id: 2, isInput: false},
	})

	// Initially in "normal" mode — no preset, no auto-restore
	assert.Equal(t, SetupOverlayNone, m.overlay)

	// Switch to "reverse" mode which has a preset, then trigger auto-restore
	setModeField(&m, "reverse (client → server)")
	m.tryShowServerRestoreOverlay()

	// Auto-restored silently, no overlay
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for non-virtual auto-restore")
	_, speakerSelected := m.multiSelect[selectKeyFor(2, "Speaker", false)]
	assert.True(t, speakerSelected, "Speaker should be auto-restored")
	assert.Contains(t, m.flashMsg, "Restored")
}

func TestServerTUI_ModeChange_Dismissed_NotRestoredAgain(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}}},
	})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
	})

	// Auto-restored for "normal" mode at init
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	assert.True(t, micSelected, "Mic should be auto-restored at init")

	// Dismiss the mode (mark as dismissed)
	m.presetDismissed["normal"] = true
	m.flashMsg = ""
	// Clear selection to test that dismissed mode won't re-restore
	m.multiSelect = make(map[string]DeviceRoleSet)

	// Switch mode and back to normal — should not auto-restore again
	setModeField(&m, "reverse (client → server)")
	m.tryShowServerRestoreOverlay() // no reverse preset
	setModeField(&m, "normal (server → client)")
	m.tryShowServerRestoreOverlay() // must not restore because "normal" is dismissed

	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay after dismiss")
	assert.Empty(t, m.multiSelect, "devices should not be re-restored after dismiss")
}

func TestServerTUI_RestoreOnServer_DevicesAutoSelected(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{
			{ID: 1, Name: "Mic", IsInput: true},
			{ID: 2, Name: "Speaker"},
		}},
	})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
		{name: "Speaker", id: 2, isInput: false},
	})

	// Auto-restored silently at init — no overlay
	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)

	// Both devices should be auto-selected
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	_, speakerSelected := m.multiSelect[selectKeyFor(2, "Speaker", false)]
	assert.True(t, micSelected, "Mic should be auto-selected after restore")
	assert.True(t, speakerSelected, "Speaker should be auto-selected after restore")
	assert.Contains(t, m.flashMsg, "Restored")
}

// TestAutoRestore_AlreadySelectedDevices_NoFlash verifies that if devices already match
// the preset, no flash message is shown and no double-restore occurs.
func TestAutoRestore_AlreadySelectedDevices_NoFlash(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}}},
	})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
	})

	// Auto-restored at init — Mic is now in multiSelect
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	assert.True(t, micSelected, "Mic should be auto-restored at init")

	// Simulate: flash is set, now we record it and clear it, then try restore again
	m.flashMsg = ""

	// Trigger restore again — selection already matches, nothing should happen
	cmd := m.tryShowServerRestoreOverlay()
	assert.Nil(t, cmd, "no cmd when selection already matches preset")
	assert.Empty(t, m.flashMsg, "no flash when devices already selected")
}

// TestAutoRestore_MissingNonVirtualDevice_FlashWarning verifies that when a preset
// device is not found in the system, a "Not found" flash message is shown.
func TestAutoRestore_MissingNonVirtualDevice_FlashWarning(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{{ID: 99, Name: "USB Headset"}}},
	})

	// USB Headset is NOT in the available devices
	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Built-in Mic", id: 1, isInput: true},
	})

	// Overlay must not be shown for a missing non-virtual device
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for missing non-virtual device")
	assert.Nil(t, m.restoreOverlay)

	// Flash must indicate the device was not found
	assert.Contains(t, m.flashMsg, "Not found", "flash should contain 'Not found'")
	assert.Contains(t, m.flashMsg, "USB Headset", "flash should name the missing device")
}

// TestAutoRestore_MixedFoundAndMissing_FlashBoth verifies that when some devices are
// found and some are not, the flash shows both "Restored" and "Not found".
func TestAutoRestore_MixedFoundAndMissing_FlashBoth(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{
			{ID: 1, Name: "Mic", IsInput: true},
			{ID: 99, Name: "Missing Speaker"},
		}},
	})

	// Only "Mic" is available; "Missing Speaker" is not
	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
	})

	assert.Equal(t, SetupOverlayNone, m.overlay)

	// Mic should be selected
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	assert.True(t, micSelected, "Mic should be restored")

	// Flash should contain both parts
	assert.Contains(t, m.flashMsg, "Restored", "flash should contain 'Restored'")
	assert.Contains(t, m.flashMsg, "Mic", "flash should name restored device")
	assert.Contains(t, m.flashMsg, "Not found", "flash should contain 'Not found'")
	assert.Contains(t, m.flashMsg, "Missing Speaker", "flash should name the missing device")
}

// TestAutoRestore_EmptyPreset_NoAction verifies that a preset with no devices does nothing.
func TestAutoRestore_EmptyPreset_NoAction(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{}},
	})

	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
	})

	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)
	assert.Empty(t, m.multiSelect, "no devices should be selected with empty preset")
	assert.Empty(t, m.flashMsg, "no flash with empty preset")
}

// TestVirtualRestoreOverlay_OnlyMissingVirtual verifies that when only virtual devices
// are missing, the overlay is VirtualOnly and lists the missing virtual devices.
func TestVirtualRestoreOverlay_OnlyMissingVirtual(t *testing.T) {
	dir := t.TempDir()
	writeServerPresetsFile(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{
			{ID: 10, Name: "BlackHole 2ch", Virtual: true},
			{ID: 11, Name: "VirtualAudio", Virtual: true},
		}},
	})

	// No virtual devices available
	m := newServerModelWithDevicesAndPresets(t, dir, []mockDeviceItem{
		{name: "Built-in Mic", id: 1, isInput: true},
	})

	assert.Equal(t, SetupOverlayRestore, m.overlay, "overlay should be shown for missing virtual devices")
	require.NotNil(t, m.restoreOverlay)
	assert.True(t, m.restoreOverlay.VirtualOnly, "overlay should be VirtualOnly mode")

	names := make([]string, 0, len(m.restoreOverlay.MissingVirtual))
	for _, d := range m.restoreOverlay.MissingVirtual {
		names = append(names, d.Name)
	}
	assert.Contains(t, names, "BlackHole 2ch")
	assert.Contains(t, names, "VirtualAudio")
	assert.Len(t, m.restoreOverlay.MissingVirtual, 2)
}

// TestVirtualRestoreOverlay_CreateButton verifies that pressing Enter on the "Create"
// button (index 0) returns RestoreActionAll.
func TestVirtualRestoreOverlay_CreateButton(t *testing.T) {
	missingVirtual := []recent.PresetDevice{
		{ID: 10, Name: "BlackHole 2ch", Virtual: true},
	}
	overlay := NewVirtualRestoreOverlay(missingVirtual, "normal")
	require.True(t, overlay.VirtualOnly)
	assert.Equal(t, 0, overlay.ButtonIdx, "default button should be Create (0)")

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, RestoreActionAll, action, "Enter on Create should return RestoreActionAll")
}

// TestVirtualRestoreOverlay_SkipButton verifies that pressing the Skip button dismisses
// the overlay by returning RestoreActionSkip.
func TestVirtualRestoreOverlay_SkipButton(t *testing.T) {
	missingVirtual := []recent.PresetDevice{
		{ID: 10, Name: "BlackHole 2ch", Virtual: true},
	}
	overlay := NewVirtualRestoreOverlay(missingVirtual, "normal")

	// Navigate to Skip button (index 1)
	overlay.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, overlay.ButtonIdx, "right arrow should move to Skip")

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, RestoreActionSkip, action, "Enter on Skip should return RestoreActionSkip")
}

// TestVirtualRestoreOverlay_EscSkips verifies that pressing Escape on the virtual
// restore overlay returns RestoreActionSkip.
func TestVirtualRestoreOverlay_EscSkips(t *testing.T) {
	overlay := NewVirtualRestoreOverlay([]recent.PresetDevice{
		{ID: 10, Name: "BlackHole 2ch", Virtual: true},
	}, "normal")

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, RestoreActionSkip, action, "Esc should return RestoreActionSkip")
}

func TestApplyLoadedConfig_ServerFields(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)

	loadedCfg := config.DefaultConfig()
	loadedCfg.Mode = config.ModeServer
	loadedCfg.Port = 9999
	loadedCfg.Password = "secret123"
	loadedCfg.MaxClients = 4
	loadedCfg.Conference = true
	loadedCfg.LogLevel = "debug"
	loadedCfg.OpusBitrate = 128000
	loadedCfg.SampleRate = 44100
	loadedCfg.Channels = 2
	loadedCfg.AEC = true
	loadedCfg.HWIDRequired = true

	m.applyLoadedConfig(loadedCfg)

	// Check main fields
	for _, f := range m.Fields {
		switch f.Key {
		case "port":
			assert.Equal(t, "9999", f.Value)
			assert.Equal(t, SourceConfig, f.Source)
		case "password":
			assert.Equal(t, "secret123", f.Value)
		case "max_clients":
			assert.Equal(t, "4", f.Value)
		case "mode":
			assert.Contains(t, f.Value, "conference")
		case "echo_cancellation":
			assert.Equal(t, "on", f.Value)
		}
	}

	// Check advanced fields
	for _, f := range m.AdvancedFields {
		switch f.Key {
		case "log_level":
			assert.Equal(t, "debug", f.Value)
			assert.Equal(t, SourceConfig, f.Source)
		case "opus_bitrate":
			assert.Equal(t, FormatBitrate(128000), f.Value)
		case "sample_rate":
			assert.Equal(t, FormatSampleRate(44100), f.Value)
		case "channels":
			assert.Equal(t, "stereo", f.Value)
		case "use_simd":
			assert.Equal(t, "on", f.Value) // default
		case "hwid_collection":
			assert.Equal(t, "on", f.Value)
		}
	}
}

func TestApplyLoadedConfig_ClientFields(t *testing.T) {
	m := newTestSetupModel(config.ModeClient)

	loadedCfg := config.DefaultConfig()
	loadedCfg.Mode = config.ModeClient
	loadedCfg.Address = "10.0.0.5"
	loadedCfg.Port = 5555
	loadedCfg.Password = "pass"
	loadedCfg.Nickname = "mypc"
	loadedCfg.AutoReconnect = true
	loadedCfg.AutoReconnectAttempts = 3

	m.applyLoadedConfig(loadedCfg)

	for _, f := range m.Fields {
		switch f.Key {
		case "server_address":
			assert.Equal(t, "10.0.0.5", f.Value)
			assert.Equal(t, SourceConfig, f.Source)
		case "port":
			assert.Equal(t, "5555", f.Value)
		case "password":
			assert.Equal(t, "pass", f.Value)
		case "nickname":
			assert.Equal(t, "mypc", f.Value)
		case "auto_reconnect":
			assert.Equal(t, "on", f.Value)
		case "reconnect_limit":
			assert.Equal(t, "3", f.Value)
		}
	}
}

func TestNewSetupModel_RecentServerIDPreserved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Save recent servers with ServerID via recent.Save.
	servers := []recent.Server{
		{
			Address:  "192.168.1.50",
			Port:     4415,
			Hostname: "MyServer",
			ServerID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
		{
			Address:  "10.0.0.1",
			Port:     4415,
			Hostname: "OtherServer",
			// No ServerID — should remain empty.
		},
	}
	require.NoError(t, recent.Save(servers))

	cfg := newTestConfig(config.ModeClient)
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, false, 120, 40)

	entries := m.serverList.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", entries[0].ServerID)
	assert.Equal(t, "", entries[1].ServerID)
}
