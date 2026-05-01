package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestLifecycleOnlyRecreateRefreshDoesNotSelectVirtualMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	m.inputDevices = []deviceRow{{ID: 1, Name: "Mic", IsInput: true}}
	m.multiSelect[m.inputDevices[0].selectKey()] = DeviceRoleSet{Capture: true}
	preset := recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}},
		VirtualSinks: []recent.VirtualSinkPreset{
			*virtualSinkPreset(recent.SinkRecreate),
		},
	}

	m.autoRestore(preset, "normal")
	m = m.WithDeviceRefreshFunc(refreshedMicAndEchoWarpItems)

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.True(t, m.virtualMicManageable)
	assert.Len(t, m.selectedVisibleRows(), 1)
	assert.Equal(t, "Mic", m.SelectedDeviceName())
	assert.NotContains(t, m.multiSelect, virtualMonitorRow().selectKey())
}

func TestSelectedVirtualMonitorRecreateSelectsAfterRefresh(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{
		Devices: []recent.PresetDevice{{
			Name: echowarpMonitorName, IsInput: true, Virtual: true,
			VirtualSink: virtualSinkPreset(recent.SinkRecreate),
		}},
		VirtualSinks: []recent.VirtualSinkPreset{
			*virtualSinkPreset(recent.SinkRecreate),
		},
	}

	cmd := m.autoRestore(preset, "normal")
	m = m.WithDeviceRefreshFunc(refreshedEchoWarpItems)

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Contains(t, m.multiSelect, virtualMonitorRow().selectKey())
	assert.Equal(t, echowarpMonitorName, m.SelectedDeviceName())
}

func TestManualCreateStillSelectsVirtualMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel().WithDeviceRefreshFunc(refreshedEchoWarpItems)
	m.virtualDeviceOverlay = NewVirtualDeviceOverlay(false, echowarpSinkName)
	m.overlay = SetupOverlayVirtualDevice

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, SetupOverlayVirtualSinkLifecycle, m.overlay)
	assert.Contains(t, m.multiSelect, virtualMonitorRow().selectKey())
	assert.Equal(t, echowarpMonitorName, m.SelectedDeviceName())
}

func refreshedMicAndEchoWarpItems() ([]list.Item, error) {
	return []list.Item{
		mockDeviceItem{name: "Mic", id: 1, isInput: true},
		mockDeviceItem{name: echowarpMonitorName, id: 11, isInput: true},
		mockDeviceItem{name: echowarpSinkName, id: 12, isInput: false},
	}, nil
}

func virtualMonitorRow() deviceRow {
	return deviceRow{ID: 11, Name: echowarpMonitorName, IsInput: true, IsVirtual: true}
}
