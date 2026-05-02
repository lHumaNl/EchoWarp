package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// getFieldSource returns the Source of a field by key.
func getFieldSource(fields []SetupField, key string) FieldSource {
	for _, f := range fields {
		if f.Key == key {
			return f.Source
		}
	}
	return SourceDefault
}

// Bug 3 — server: a preset with log_level="info" (default) must not mark the
// field as user-set on restore.
func TestRestoreModePreset_LogLevelDefault_NoUserSetMarker(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	m.restoreModePreset(preset.ModePreset{LogLevel: "info"})

	assert.Equal(t, "info", getFieldValue(m.AdvancedFields, "log_level"))
	assert.Equal(t, SourceDefault, getFieldSource(m.AdvancedFields, "log_level"),
		"log_level=info (default) must keep SourceDefault — no ✓ marker")
}

// Bug 3 — server: a preset with a non-default log_level must be restored and
// marked as user-set.
func TestRestoreModePreset_LogLevelCustom_UserSetMarker(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	m.restoreModePreset(preset.ModePreset{LogLevel: "debug"})

	assert.Equal(t, "debug", getFieldValue(m.AdvancedFields, "log_level"))
	assert.Equal(t, SourceConfig, getFieldSource(m.AdvancedFields, "log_level"),
		"log_level=debug (custom) must be SourceConfig — ✓ marker shown")
}

// Bug 3 — client: a recent server entry with LogLevel="info" must not mark the
// field as user-set when NewSetupModel loads it.
func TestNewSetupModel_Client_LogLevelDefaultNotMarked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	servers := []recent.Server{
		{Address: "1.2.3.4", Port: 4415, Hostname: "Host", LogLevel: "info"},
	}
	require.NoError(t, recent.Save(servers))

	cfg := newTestConfig(config.ModeClient)
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, false, 120, 40)

	assert.Equal(t, "info", getFieldValue(m.AdvancedFields, "log_level"))
	assert.Equal(t, SourceDefault, getFieldSource(m.AdvancedFields, "log_level"),
		"client: LogLevel=info from recent must keep SourceDefault")
}

// Bug 3 — client: a custom LogLevel from the recent entry must be applied with
// SourceConfig so the ✓ marker appears.
func TestNewSetupModel_Client_LogLevelCustomRestored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	servers := []recent.Server{
		{Address: "1.2.3.4", Port: 4415, Hostname: "Host", LogLevel: "debug"},
	}
	require.NoError(t, recent.Save(servers))

	cfg := newTestConfig(config.ModeClient)
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, false, 120, 40)

	assert.Equal(t, "debug", getFieldValue(m.AdvancedFields, "log_level"))
	assert.Equal(t, SourceConfig, getFieldSource(m.AdvancedFields, "log_level"),
		"client: LogLevel=debug must be SourceConfig")
}

// Bug 1 — reverse mode: after construction the focused DeviceSection must be
// Output (the only visible section) with cursor at 0, so ↑/↓ work immediately.
func TestNewSetupModel_Reverse_CursorOnOutputSection(t *testing.T) {
	m := newSectionTestModel(config.ModeServer, true /*reverse*/, false, false)
	assert.Equal(t, SectionOutput, m.DeviceSection,
		"reverse mode must focus Output section (input is hidden)")
	assert.Equal(t, 0, m.deviceCursor, "cursor must start at row 0")
}

// Bug 4 — conference preset with max_clients omitted (zero) must re-hydrate
// to the mode default (2), not fall back to cfg.MaxClients (1).
func TestRestoreModePreset_Conference_MaxClientsDefaults(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	// Switch the Mode field to conference before restoring — restoreModePreset
	// reads the current mode to pick DefaultsFor().
	setModeFieldFull(&m, "conference (multi-user)")

	// Empty preset: simulates a saved file where max_clients matched the
	// default (2) and was therefore stripped on Save.
	m.restoreModePreset(preset.ModePreset{})

	assert.Equal(t, "2", getFieldValue(m.Fields, "max_clients"),
		"conference: max_clients must default to 2 when omitted from preset")
	assert.Equal(t, SourceDefault, getFieldSource(m.Fields, "max_clients"),
		"conference default max_clients must keep SourceDefault (no ✓ marker)")
}

// Bug 5 — switching to a mode whose preset has no log_level must reset the
// field to the "info" default, not leak the previous mode's value.
func TestLoadPresetForMode_LogLevelResetsBetweenModes(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	m.serverPresets = &preset.ServerPresets{
		Presets: map[string]preset.ModePreset{
			"reverse": {LogLevel: "debug"},
			"duplex":  {}, // no log_level
		},
	}

	// Land on reverse — log_level becomes "debug".
	m.loadPresetForMode("reverse")
	assert.Equal(t, "debug", getFieldValue(m.AdvancedFields, "log_level"))

	// Switch to duplex — log_level must reset to the "info" default,
	// not leak "debug" from the reverse preset.
	m.loadPresetForMode("duplex")
	assert.Equal(t, "info", getFieldValue(m.AdvancedFields, "log_level"),
		"log_level must reset to default when target mode has no saved value")
	assert.Equal(t, SourceDefault, getFieldSource(m.AdvancedFields, "log_level"),
		"reset log_level must be SourceDefault (no ✓ marker)")
}

// Bug 2 — duplex: when last_mode=duplex is restored from server presets,
// visibleSections() must return input=true, output=true on startup (without
// requiring a manual mode toggle).
func TestNewSetupModel_Duplex_BothSectionsVisibleOnStart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Seed a server preset file with last_mode=duplex.
	sp := preset.ServerPresets{
		LastMode: "duplex",
		Presets: map[string]preset.ModePreset{
			"duplex": {Port: 4415},
		},
	}
	require.NoError(t, preset.Save(sp))

	// cfg.Duplex=false simulates a plain restart without CLI flags —
	// this is exactly the case where Bug 2 used to hide the Output section.
	cfg := newTestConfig(config.ModeServer)
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, true, 120, 40)
	m = m.WithUnifiedDeviceList(cfg.Duplex /* == false */)

	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput, "duplex (from last_mode): Input section must be visible at startup")
	assert.True(t, showOutput, "duplex (from last_mode): Output section must be visible at startup")
	assert.True(t, m.isDuplexMode, "isDuplexMode must be true when last_mode=duplex")
}
