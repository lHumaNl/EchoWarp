package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
)

func newSectionTestModel(mode config.Mode, reverse, duplex, conference bool) SetupModel {
	cfg := newTestConfig(mode)
	cfg.Reverse = reverse
	cfg.Duplex = duplex
	cfg.Conference = conference
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	isInput := mode == config.ModeServer
	m := NewSetupModel(cfg, deviceList, isInput, 120, 40)
	m = m.WithUnifiedDeviceList(duplex || conference)
	return m
}

func TestVisibleSections_NormalServer(t *testing.T) {
	m := newSectionTestModel(config.ModeServer, false, false, false)
	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput, "server normal should show input")
	assert.False(t, showOutput, "server normal should hide output")
}

func TestVisibleSections_NormalClient(t *testing.T) {
	m := newSectionTestModel(config.ModeClient, false, false, false)
	showInput, showOutput := m.visibleSections()
	assert.False(t, showInput, "client normal should hide input")
	assert.True(t, showOutput, "client normal should show output")
}

func TestVisibleSections_ReverseServer(t *testing.T) {
	m := newSectionTestModel(config.ModeServer, true, false, false)
	showInput, showOutput := m.visibleSections()
	assert.False(t, showInput, "server reverse should hide input")
	assert.True(t, showOutput, "server reverse should show output")
}

func TestVisibleSections_ReverseClient(t *testing.T) {
	m := newSectionTestModel(config.ModeClient, true, false, false)
	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput, "client reverse should show input")
	assert.False(t, showOutput, "client reverse should hide output")
}

func TestVisibleSections_Duplex(t *testing.T) {
	m := newSectionTestModel(config.ModeServer, false, true, false)
	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput, "duplex should show input")
	assert.True(t, showOutput, "duplex should show output")
}

func TestVisibleSections_Conference(t *testing.T) {
	m := newSectionTestModel(config.ModeServer, false, false, true)
	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput, "conference should show input")
	assert.True(t, showOutput, "conference should show output")
}

func TestModeChange_SectionVisibility_Updates(t *testing.T) {
	// Start as server normal mode (input visible, output hidden)
	m := newSectionTestModel(config.ModeServer, false, false, false)
	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput)
	assert.False(t, showOutput)

	// Switch to reverse mode via Mode field toggle
	for i := range m.Fields {
		if m.Fields[i].Key == "mode" {
			m.Fields[i].Toggle() // normal -> reverse
			break
		}
	}
	m.applyFieldDependencies()

	showInput, showOutput = m.visibleSections()
	assert.False(t, showInput, "after switch to reverse, input should be hidden")
	assert.True(t, showOutput, "after switch to reverse, output should be visible")
}

func TestModeChange_FocusMovesToVisibleSection(t *testing.T) {
	// Start as server normal mode, focus on Input section
	m := newSectionTestModel(config.ModeServer, false, false, false)
	m.DeviceSection = SectionInput

	// Switch to reverse: Input becomes hidden, focus should move to Output
	for i := range m.Fields {
		if m.Fields[i].Key == "mode" {
			m.Fields[i].Toggle() // normal -> reverse
			break
		}
	}
	m.applyFieldDependencies()

	assert.Equal(t, SectionOutput, m.DeviceSection, "focus should move to Output when Input is hidden")
	assert.Equal(t, 0, m.deviceCursor, "cursor should reset to 0")

	// Switch to duplex: both visible, focus stays on Output
	for i := range m.Fields {
		if m.Fields[i].Key == "mode" {
			m.Fields[i].Toggle() // reverse -> duplex
			break
		}
	}
	m.applyFieldDependencies()
	showInput, showOutput := m.visibleSections()
	assert.True(t, showInput)
	assert.True(t, showOutput)

	// Switch back to normal: Output hidden, focus should move to Input
	// Toggle twice more: duplex -> conference -> normal (wrap around)
	for i := range m.Fields {
		if m.Fields[i].Key == "mode" {
			m.Fields[i].Toggle() // duplex -> conference
			m.Fields[i].Toggle() // conference -> normal (wraps)
			break
		}
	}
	m.applyFieldDependencies()

	// Now on normal, section should be Input
	showInput, showOutput = m.visibleSections()
	assert.True(t, showInput)
	assert.False(t, showOutput)

	// If we were on Output, it should have been corrected
	// (applyFieldDependencies handles this)
}

func TestTabSkipsHiddenSection(t *testing.T) {
	// Server normal mode: only Input visible
	m := newSectionTestModel(config.ModeServer, false, false, false)
	m.activeColumn = ColumnDevices
	m.DeviceSection = SectionInput

	// Tab should NOT switch to Output (it's hidden)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, SectionInput, m.DeviceSection, "Tab should not switch to hidden Output section")
}

func TestCtrlDownSkipsHiddenSection(t *testing.T) {
	// Server reverse mode: only Output visible
	m := newSectionTestModel(config.ModeServer, true, false, false)
	m.activeColumn = ColumnDevices
	m.DeviceSection = SectionOutput

	// Ctrl+Up should NOT switch to Input (it's hidden)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlUp})
	assert.Equal(t, SectionOutput, m.DeviceSection, "Ctrl+Up should not switch to hidden Input section")
}
