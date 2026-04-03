package views

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// makeDeviceList creates a list.Model from mock device items.
func makeDeviceList(devices []mockDeviceItem) list.Model {
	items := make([]list.Item, len(devices))
	for i, d := range devices {
		items[i] = d
	}
	return list.New(items, list.NewDefaultDelegate(), 80, 20)
}

// newServerModel creates a server SetupModel, sets ECHOWARP_CONFIG_DIR to tempDir,
// then calls WithUnifiedDeviceList to trigger preset auto-restore logic.
func newServerModel(t *testing.T, tempDir string, devices []mockDeviceItem, isDuplex bool) SetupModel {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", tempDir)
	dl := makeDeviceList(devices)
	cfg := newTestConfig(config.ModeServer)
	m := NewSetupModel(cfg, dl, true, 120, 40)
	return m.WithUnifiedDeviceList(isDuplex)
}

// writePresets writes server_presets.json to tempDir.
func writePresets(t *testing.T, tempDir string, presets map[string]recent.DevicePreset) {
	t.Helper()
	sp := preset.ServerPresets{Presets: presets}
	data, err := json.MarshalIndent(sp, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(tempDir, 0700))
	require.NoError(t, os.WriteFile(tempDir+"/server_presets.json", data, 0600))
}

// writeRecentServers writes recent_servers.json to tempDir.
func writeRecentServers(t *testing.T, tempDir string, servers []recent.Server) {
	t.Helper()
	data, err := json.MarshalIndent(servers, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(tempDir, 0700))
	require.NoError(t, os.WriteFile(tempDir+"/recent_servers.json", data, 0600))
}

// --- Scenario 2: Server — startup auto-restore → mode change → auto-restore again ---

func TestIntegration_ServerStartupRestore_ThenModeChange(t *testing.T) {
	dir := t.TempDir()

	// Write presets for both "normal" and "reverse" modes
	writePresets(t, dir, map[string]recent.DevicePreset{
		"normal":  {Devices: []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}}},
		"reverse": {Devices: []recent.PresetDevice{{ID: 2, Name: "Speaker"}}},
	})

	devices := []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
		{name: "Speaker", id: 2, isInput: false},
	}

	// Step 1: Create server model — should auto-restore "normal" preset silently
	m := newServerModel(t, dir, devices, false)
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for auto-restore")
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	assert.True(t, micSelected, "Mic should be auto-restored for normal mode")
	assert.Contains(t, m.flashMsg, "Restored")

	// Step 2: Dismiss normal mode
	m.presetDismissed["normal"] = true

	// Step 3: Switch mode to "reverse" and trigger auto-restore
	setModeField(&m, "reverse (client → server)")
	m.tryShowServerRestoreOverlay()

	// Step 4: Speaker auto-restored silently
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for auto-restore after mode switch")
	_, speakerSelected := m.multiSelect[selectKeyFor(2, "Speaker", false)]
	assert.True(t, speakerSelected, "Speaker should be auto-restored for reverse mode")

	// Step 5: Flash message updated
	assert.Contains(t, m.flashMsg, "Restored")
}

// --- Scenario 3: Device disappeared → flash warning ---

func TestIntegration_DeviceDisappeared_FlashWarning(t *testing.T) {
	dir := t.TempDir()

	// Preset references "USB Mic" (id=5) but current devices don't include it
	writePresets(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{{ID: 5, Name: "USB Mic", IsInput: true}}},
	})

	// Available devices: no "USB Mic"
	devices := []mockDeviceItem{
		{name: "Built-in Mic", id: 1, isInput: true},
	}

	m := newServerModel(t, dir, devices, false)

	// Auto-restore finds no matched devices — shows "Not found" flash
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay when all preset devices are missing")
	assert.Contains(t, m.flashMsg, "Not found")
	assert.Contains(t, m.flashMsg, "USB Mic")

	// Directly test applyRestore with the missing device: should produce flash message
	m.flashMsg = ""
	p := recent.DevicePreset{Devices: []recent.PresetDevice{{ID: 5, Name: "USB Mic", IsInput: true}}}
	cmd := m.applyRestore(p, false)
	assert.NotNil(t, cmd, "flash timer cmd should be returned for unmatched device")
	assert.Contains(t, m.flashMsg, "USB Mic")
	assert.Contains(t, m.flashMsg, "not found")
}

// --- Scenario 4: Preset dismissed not restored again ---

func TestIntegration_PresetDismissed_NotShownAgain(t *testing.T) {
	dir := t.TempDir()

	writePresets(t, dir, map[string]recent.DevicePreset{
		"normal":  {Devices: []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}}},
		"reverse": {Devices: []recent.PresetDevice{{ID: 2, Name: "Speaker"}}},
	})

	devices := []mockDeviceItem{
		{name: "Mic", id: 1, isInput: true},
		{name: "Speaker", id: 2, isInput: false},
	}

	m := newServerModel(t, dir, devices, false)

	// Auto-restored for "normal"
	_, micSelected := m.multiSelect[selectKeyFor(1, "Mic", true)]
	assert.True(t, micSelected, "Mic should be auto-restored")

	// Dismiss normal
	m.presetDismissed["normal"] = true
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.flashMsg = ""

	// Switch to reverse and dismiss it too
	setModeField(&m, "reverse (client → server)")
	m.tryShowServerRestoreOverlay()
	m.presetDismissed["reverse"] = true
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.flashMsg = ""

	// Back to normal — already dismissed, must NOT restore
	setModeField(&m, "normal (server → client)")
	m.tryShowServerRestoreOverlay()
	assert.Equal(t, SetupOverlayNone, m.overlay, "dismissed preset must not reappear for same mode")
	assert.Empty(t, m.multiSelect, "devices should not be re-restored after dismiss")
}

// --- Scenario 5: Virtual device not found → overlay shown ---

func TestIntegration_VirtualDeviceNotFound_OverlayShown(t *testing.T) {
	dir := t.TempDir()

	// Preset contains a virtual device + a real device
	writePresets(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{
			{ID: 1, Name: "Built-in Mic", IsInput: true},
			{ID: 10, Name: "BlackHole 2ch", IsInput: true, Virtual: true},
		}},
	})

	// Available devices: only Built-in Mic, no BlackHole
	devices := []mockDeviceItem{
		{name: "Built-in Mic", id: 1, isInput: true},
	}

	m := newServerModel(t, dir, devices, false)

	// Non-virtual device auto-restored, virtual device triggers overlay
	_, micSelected := m.multiSelect[selectKeyFor(1, "Built-in Mic", true)]
	assert.True(t, micSelected, "Built-in Mic should be auto-restored")
	assert.Equal(t, SetupOverlayRestore, m.overlay, "overlay should show for missing virtual devices")
	require.NotNil(t, m.restoreOverlay)
	assert.True(t, m.restoreOverlay.VirtualOnly)
	assert.Len(t, m.restoreOverlay.MissingVirtual, 1)
	assert.Equal(t, "BlackHole 2ch", m.restoreOverlay.MissingVirtual[0].Name)
}

// --- Scenario 5b: Virtual device not found, no real devices match → no overlay, just flash ---

func TestIntegration_OnlyVirtualDeviceNotFound_OverlayShown(t *testing.T) {
	dir := t.TempDir()

	// Preset contains only a virtual device
	writePresets(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{
			{ID: 10, Name: "BlackHole 2ch", IsInput: true, Virtual: true},
		}},
	})

	// Available devices: no BlackHole
	devices := []mockDeviceItem{
		{name: "Built-in Mic", id: 1, isInput: true},
	}

	m := newServerModel(t, dir, devices, false)

	// Virtual device missing → overlay shown
	assert.Equal(t, SetupOverlayRestore, m.overlay, "overlay should show for missing virtual device")
	require.NotNil(t, m.restoreOverlay)
	assert.True(t, m.restoreOverlay.VirtualOnly)
}

// --- Scenario 6: Nickname → hostname migration ---

func TestIntegration_NicknameToHostnameMigration(t *testing.T) {
	dir := t.TempDir()

	// Write recent_servers.json with old "nickname" field
	raw := `[{"address":"192.168.1.1","port":4415,"nickname":"OldPC","last_connected":"2025-01-01T00:00:00Z"}]`
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(dir+"/recent_servers.json", []byte(raw), 0600))

	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	servers, err := recent.Load()
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "OldPC", servers[0].Hostname, "hostname should be migrated from nickname")

	// Save and reload to confirm modern YAML format
	require.NoError(t, recent.Save(servers))

	data, err := os.ReadFile(dir + "/recent_servers.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(data), "hostname:")
	assert.NotContains(t, string(data), "nickname:")
}

// --- Multiple sessions: last streamingStartedMsg overwrites preset ---

func TestIntegration_MultiplePresetSaves_LastWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// First save: normal with 1 device
	sp := preset.ServerPresets{Presets: make(map[string]recent.DevicePreset)}
	sp.Set("normal", recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 1, Name: "Mic"}},
	})
	require.NoError(t, preset.Save(sp))

	// Second save: normal with 2 devices (overwrite)
	sp2 := preset.Load()
	sp2.Set("normal", recent.DevicePreset{
		Devices: []recent.PresetDevice{
			{ID: 1, Name: "Mic"},
			{ID: 2, Name: "Speaker"},
		},
	})
	require.NoError(t, preset.Save(sp2))

	// Load and verify last write wins
	sp3 := preset.Load()
	p := sp3.Get("normal")
	require.NotNil(t, p)
	assert.Len(t, p.Devices, 2, "second save should overwrite first")

	names := make([]string, 0, 2)
	for _, d := range p.Devices {
		names = append(names, d.Name)
	}
	assert.Contains(t, names, "Mic")
	assert.Contains(t, names, "Speaker")
}

// --- Mode key extraction: tryShowServerRestoreOverlay uses first word ---

func TestIntegration_ModeKeyExtraction(t *testing.T) {
	dir := t.TempDir()

	// Preset for "duplex" mode
	writePresets(t, dir, map[string]recent.DevicePreset{
		"duplex": {Devices: []recent.PresetDevice{{ID: 3, Name: "Duplex Mic", IsInput: true}}},
	})

	devices := []mockDeviceItem{
		{name: "Duplex Mic", id: 3, isInput: true},
	}

	m := newServerModel(t, dir, devices, false)

	// normal mode initially — no auto-restore (no normal preset)
	assert.Equal(t, SetupOverlayNone, m.overlay)

	// Switch to duplex mode and trigger auto-restore
	setModeField(&m, "duplex (bidirectional)")
	m.tryShowServerRestoreOverlay()

	// Auto-restored silently
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for non-virtual auto-restore")
	_, micSelected := m.multiSelect[selectKeyFor(3, "Duplex Mic", true)]
	assert.True(t, micSelected, "Duplex Mic should be auto-restored")
	assert.Contains(t, m.flashMsg, "Restored")
}

// newClientModel creates a client SetupModel with ECHOWARP_CONFIG_DIR set to tempDir,
// loads recentServers from the dir, then calls WithUnifiedDeviceList.
func newClientModel(t *testing.T, tempDir string, devices []mockDeviceItem) SetupModel {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", tempDir)
	dl := makeDeviceList(devices)
	cfg := newTestConfig(config.ModeClient)
	m := NewSetupModel(cfg, dl, false, 120, 40)
	return m.WithUnifiedDeviceList(false)
}

// --- Client auto-restore integration tests ---

func TestIntegration_ClientAutoRestore_AfterProbe(t *testing.T) {
	dir := t.TempDir()

	// Write a recent server with a preset for "normal" mode
	writeRecentServers(t, dir, []recent.Server{
		{
			Address:  "192.168.1.10",
			Port:     4415,
			Hostname: "TestServer",
			Presets: map[string]recent.DevicePreset{
				"normal": {Devices: []recent.PresetDevice{
					{ID: 1, Name: "Headphones"},
				}},
			},
		},
	})

	devices := []mockDeviceItem{
		{name: "Headphones", id: 1, isInput: false},
	}

	m := newClientModel(t, dir, devices)

	// Verify recent servers were loaded
	require.NotEmpty(t, m.recentServers, "recentServers should be loaded from disk")

	// Simulate probe result arriving for this server in "normal" mode
	cmd := m.tryShowRestoreOverlay("192.168.1.10", 4415, "normal")

	// Headphones should be auto-restored silently, no overlay
	assert.NotNil(t, cmd, "flash cmd should be returned")
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay for non-virtual auto-restore")
	assert.Nil(t, m.restoreOverlay)
	_, selected := m.multiSelect[selectKeyFor(1, "Headphones", false)]
	assert.True(t, selected, "Headphones should be auto-restored")
	assert.Contains(t, m.flashMsg, "Restored")
	assert.Contains(t, m.flashMsg, "Headphones")
}

func TestIntegration_AutoRestore_ScanDoesNotRetrigger(t *testing.T) {
	dir := t.TempDir()

	writeRecentServers(t, dir, []recent.Server{
		{
			Address:  "10.0.0.1",
			Port:     4415,
			Hostname: "HomeServer",
			Presets: map[string]recent.DevicePreset{
				"normal": {Devices: []recent.PresetDevice{
					{ID: 3, Name: "USB Speaker"},
				}},
			},
		},
	})

	devices := []mockDeviceItem{
		{name: "USB Speaker", id: 3, isInput: false},
	}

	m := newClientModel(t, dir, devices)
	require.NotEmpty(t, m.recentServers)

	// First probe — auto-restore happens
	cmd := m.tryShowRestoreOverlay("10.0.0.1", 4415, "normal")
	assert.NotNil(t, cmd, "first probe should produce a flash cmd")
	_, selected := m.multiSelect[selectKeyFor(3, "USB Speaker", false)]
	assert.True(t, selected, "USB Speaker should be restored on first probe")

	// Clear flash to check second probe does not re-trigger
	m.flashMsg = ""

	// Second probe (e.g., rescan) — selection already matches, no re-restore
	cmd2 := m.tryShowRestoreOverlay("10.0.0.1", 4415, "normal")
	assert.Nil(t, cmd2, "second probe should not re-trigger restore when selection matches")
	assert.Empty(t, m.flashMsg, "no flash on second probe when already selected")
}

func TestIntegration_AutoRestore_VirtualFound_SilentRestore(t *testing.T) {
	dir := t.TempDir()

	writePresets(t, dir, map[string]recent.DevicePreset{
		"normal": {Devices: []recent.PresetDevice{
			{ID: 10, Name: "BlackHole 2ch", IsInput: true, Virtual: true},
		}},
	})

	// BlackHole IS available in the system
	devices := []mockDeviceItem{
		{name: "BlackHole 2ch", id: 10, isInput: true},
	}

	m := newServerModel(t, dir, devices, false)

	// Virtual device is present — should be silently restored, no overlay
	assert.Equal(t, SetupOverlayNone, m.overlay, "no overlay when virtual device is found in system")
	assert.Nil(t, m.restoreOverlay)
	_, selected := m.multiSelect[selectKeyFor(10, "BlackHole 2ch", true)]
	assert.True(t, selected, "BlackHole 2ch should be silently restored")
	assert.Contains(t, m.flashMsg, "Restored")
	assert.NotContains(t, m.flashMsg, "Not found", "should not show 'Not found' for a found device")
}
