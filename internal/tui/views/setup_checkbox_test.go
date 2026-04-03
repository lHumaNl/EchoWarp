package views

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

// helper: create a duplex setup model with input and output devices populated.
func newDuplexSetupModel() SetupModel {
	m := newTestSetupModel(config.ModeServer)
	m.inputDevices = []deviceRow{
		{Name: "Built-in Mic", ID: 1, IsInput: true},
		{Name: "USB Mic", ID: 2, IsInput: true},
	}
	m.outputDevices = []deviceRow{
		{Name: "Built-in Speaker", ID: 3, IsInput: false},
		{Name: "Headphones", ID: 4, IsInput: false},
	}
	m.unifiedDuplex = true
	m.isDuplexMode = true
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.activeColumn = ColumnDevices
	m.DeviceSection = SectionInput
	m.deviceCursor = 0
	return m
}

func TestSingleCheckbox_InputDevice_SetsCaptureRole(t *testing.T) {
	m := newDuplexSetupModel()
	m.DeviceSection = SectionInput
	m.deviceCursor = 0

	m, _ = m.handleMultiSelectToggle()

	key := m.inputDevices[0].selectKey()
	roles, ok := m.multiSelect[key]
	require.True(t, ok, "device should be selected")
	assert.True(t, roles.Capture, "input device should have Capture=true")
	assert.False(t, roles.Playback, "input device should have Playback=false")
}

func TestSingleCheckbox_OutputDevice_SetsPlaybackRole(t *testing.T) {
	m := newDuplexSetupModel()
	m.DeviceSection = SectionOutput
	m.deviceCursor = 0

	m, _ = m.handleMultiSelectToggle()

	key := m.outputDevices[0].selectKey()
	roles, ok := m.multiSelect[key]
	require.True(t, ok, "device should be selected")
	assert.False(t, roles.Capture, "output device should have Capture=false")
	assert.True(t, roles.Playback, "output device should have Playback=true")
}

func TestSingleCheckbox_ToggleOff(t *testing.T) {
	m := newDuplexSetupModel()
	m.DeviceSection = SectionInput
	m.deviceCursor = 0

	// Toggle on
	m, _ = m.handleMultiSelectToggle()
	key := m.inputDevices[0].selectKey()
	_, ok := m.multiSelect[key]
	require.True(t, ok, "device should be selected after first toggle")

	// Toggle off
	m, _ = m.handleMultiSelectToggle()
	_, ok = m.multiSelect[key]
	assert.False(t, ok, "device should be removed from multiSelect after second toggle")
}

func TestSingleCheckbox_NoDualColumns(t *testing.T) {
	m := newDuplexSetupModel()

	// Left arrow in Devices column should switch to Settings, not decrement a checkboxCol
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, ColumnSettings, m2.activeColumn, "right arrow should switch to Settings")

	// Go back to devices
	m3, _ := m2.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, ColumnDevices, m3.activeColumn, "left arrow should switch back to Devices")

	// Another left arrow should NOT do anything special (no checkboxCol to decrement)
	m4, _ := m3.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, ColumnDevices, m4.activeColumn, "left arrow in Devices should stay in Devices")
}

func TestBuildConfig_RoleDeterminedBySection(t *testing.T) {
	m := newDuplexSetupModel()

	// Select an input and output device
	m.DeviceSection = SectionInput
	m.deviceCursor = 0
	m, _ = m.handleMultiSelectToggle()

	m.DeviceSection = SectionOutput
	m.deviceCursor = 0
	m, _ = m.handleMultiSelectToggle()

	cfg := m.BuildConfig()

	require.Len(t, cfg.Devices, 2, "should have 2 devices")

	// Find each device by name
	for _, d := range cfg.Devices {
		switch d.Name {
		case "Built-in Mic":
			assert.Equal(t, config.RoleCapture, d.Role, "input device should have RoleCapture")
		case "Built-in Speaker":
			assert.Equal(t, config.RolePlayback, d.Role, "output device should have RolePlayback")
		}
	}
}

func TestBuildConfig_DuplexValidation(t *testing.T) {
	// Only input selected -> validation error
	m := newDuplexSetupModel()
	m.DeviceSection = SectionInput
	m.deviceCursor = 0
	m, _ = m.handleMultiSelectToggle()

	m2, _ := m.tryStart()
	assert.Contains(t, m2.validationError, "capture", "should require both capture and playback")

	// Now add an output device -> should pass validation
	m.DeviceSection = SectionOutput
	m.deviceCursor = 0
	m, _ = m.handleMultiSelectToggle()

	m3, cmd := m.tryStart()
	assert.Empty(t, m3.validationError, "should have no validation error with both input and output")
	assert.NotNil(t, cmd, "should return a command (SetupDoneMsg)")
}

func TestDeviceStatusHint_Duplex_CountsBySection(t *testing.T) {
	m := newDuplexSetupModel()

	// Select 1 input, 1 output
	m.DeviceSection = SectionInput
	m.deviceCursor = 0
	m, _ = m.handleMultiSelectToggle()

	m.DeviceSection = SectionOutput
	m.deviceCursor = 0
	m, _ = m.handleMultiSelectToggle()

	hint := m.deviceStatusHint()
	assert.True(t, strings.Contains(hint, "1 capture"), "should show 1 capture, got: %s", hint)
	assert.True(t, strings.Contains(hint, "1 playback"), "should show 1 playback, got: %s", hint)
}
