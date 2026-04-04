package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestBuildDeviceTitle(t *testing.T) {
	tests := []struct {
		name     string
		isInput  bool
		expected string
	}{
		{"input device title", true, "Select Input Device"},
		{"output device title", false, "Select Output Device"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDeviceTitle(tt.isInput)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock item for list testing
type testDeviceItem struct {
	id   uint32
	name string
}

func (i testDeviceItem) FilterValue() string { return i.name }
func (i testDeviceItem) Title() string       { return i.name }
func (i testDeviceItem) Description() string { return "Device description" }

func TestDeviceSelectView_InputDevice(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Microphone 1"},
		testDeviceItem{id: 2, name: "Microphone 2"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)
	result := DeviceSelectView(deviceList, true, 80, 24)

	assert.Contains(t, result, "Select Input Device")
}

func TestDeviceSelectView_OutputDevice(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Speaker 1"},
		testDeviceItem{id: 2, name: "Speaker 2"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)
	result := DeviceSelectView(deviceList, false, 80, 24)

	assert.Contains(t, result, "Select Output Device")
}

func TestDeviceSelectView_EmptyList(t *testing.T) {
	deviceList := list.New(nil, list.NewDefaultDelegate(), 70, 15)
	result := DeviceSelectView(deviceList, true, 80, 24)

	assert.Contains(t, result, "Select Input Device")
}

func TestDeviceSelectView_SmallTerminal(t *testing.T) {
	items := []list.Item{testDeviceItem{id: 1, name: "Device"}}
	deviceList := list.New(items, list.NewDefaultDelegate(), 30, 8)
	result := DeviceSelectView(deviceList, true, 40, 12)

	assert.Contains(t, result, "Select Input Device")
}

func TestDeviceSelectView_LargeTerminal(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Device 1"},
		testDeviceItem{id: 2, name: "Device 2"},
		testDeviceItem{id: 3, name: "Device 3"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 100, 30)
	result := DeviceSelectView(deviceList, false, 120, 40)

	assert.Contains(t, result, "Select Output Device")
}

func TestDeviceSelectUpdate_KeyMessage(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Device 1"},
		testDeviceItem{id: 2, name: "Device 2"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)
	deviceList.Select(0)

	updatedList, cmd := DeviceSelectUpdate(deviceList, tea.KeyMsg{Type: tea.KeyDown})
	assert.NotNil(t, updatedList)
	_ = cmd
}

func TestDeviceSelectUpdate_EnterKey(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Device 1"},
		testDeviceItem{id: 2, name: "Device 2"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)
	deviceList.Select(0)

	updatedList, cmd := DeviceSelectUpdate(deviceList, tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, updatedList)
	_ = cmd
}

func TestDeviceSelectUpdate_FilterMode(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Device 1"},
		testDeviceItem{id: 2, name: "Device 2"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)
	updatedList, cmd := DeviceSelectUpdate(deviceList, tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'/'},
	})
	assert.NotNil(t, updatedList)
	_ = cmd
}

func TestDeviceSelectUpdate_WindowResize(t *testing.T) {
	items := []list.Item{testDeviceItem{id: 1, name: "Device 1"}}
	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)

	updatedList, cmd := DeviceSelectUpdate(deviceList, tea.WindowSizeMsg{Width: 100, Height: 30})
	assert.NotNil(t, updatedList)
	_ = cmd
}

func TestDeviceSelectUpdate_MouseEvent(t *testing.T) {
	items := []list.Item{
		testDeviceItem{id: 1, name: "Device 1"},
		testDeviceItem{id: 2, name: "Device 2"},
	}

	deviceList := list.New(items, list.NewDefaultDelegate(), 70, 15)
	updatedList, cmd := DeviceSelectUpdate(deviceList, tea.MouseMsg{
		Type: tea.MouseLeft,
		X:    10,
		Y:    5,
	})
	assert.NotNil(t, updatedList)
	_ = cmd
}
