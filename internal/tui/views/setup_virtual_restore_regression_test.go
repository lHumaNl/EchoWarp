package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestExistingEchoWarpSinkIsManageableAndRemovable(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := virtualAudioTestModel([]deviceRow{{ID: 7, Name: echowarpSinkName}})

	m.syncVirtualMicState()
	m, _ = m.openVirtualMicOverlay()
	require.NotNil(t, m.virtualDeviceOverlay)
	assert.True(t, m.virtualDeviceOverlay.Exists)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, stub)
	assert.Equal(t, []string{echowarpSinkName}, stub.findNames)
	assert.Empty(t, stub.createdNames)
	assert.Equal(t, []string{"42"}, stub.removedIDs)
	assert.False(t, m.virtualMicManageable)
	assert.Equal(t, SetupOverlayNone, m.overlay)
}

func TestExistingEchoWarpExtraDoesNotBecomeManageable(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := virtualAudioTestModel([]deviceRow{{ID: 8, Name: "EchoWarpExtra"}})

	m.syncVirtualMicState()

	assert.Empty(t, stub.findNames)
	assert.False(t, m.virtualMicManageable)
	assert.Equal(t, "Create Virtual Audio Device ▸", m.Fields[0].ActionLabel)
}

func TestMissingVirtualDevicesDoNotOpenStartupPrompt(t *testing.T) {
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpMonitorName, IsInput: true, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkKeep),
	}}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)
	assert.Empty(t, m.flashMsg)
}

func TestMissingVirtualDevicesNeverOpenStartupPrompt(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.createErr = errors.New("pactl unavailable")
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpMonitorName, IsInput: true, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)
}

func TestInputOnlyRestoreIgnoresOutputEchoWarpPreset(t *testing.T) {
	m := newInputOnlyRestoreModel()
	m.outputDevices = []deviceRow{{ID: 2, Name: echowarpSinkName, IsVirtual: true}}
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpSinkName, IsInput: false, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkKeep),
	}}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.NotContains(t, m.flashMsg, "Restored: "+echowarpSinkName)
	assert.Empty(t, m.SelectedDeviceName())
	assert.Empty(t, m.selectedVisibleRows())
}

func TestInputOnlyVirtualCreationSelectsMonitor(t *testing.T) {
	m := newInputOnlyRestoreModel()
	monitor := deviceRow{ID: 1, Name: echowarpMonitorName, IsInput: true, IsVirtual: true}
	sink := deviceRow{ID: 2, Name: echowarpSinkName, IsVirtual: true}
	m.inputDevices = []deviceRow{monitor}
	m.outputDevices = []deviceRow{sink}

	m.autoSelectVirtualDevice(echowarpSinkName)

	assert.Contains(t, m.multiSelect, monitor.selectKey())
	assert.NotContains(t, m.multiSelect, sink.selectKey())
	assert.Equal(t, echowarpMonitorName, m.SelectedDeviceName())
}

func TestVirtualSinkMatchingAvoidsEchoWarpExtra(t *testing.T) {
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpSinkName, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkKeep),
	}}}
	outputs := []deviceRow{{ID: 9, Name: "EchoWarpExtra", IsVirtual: true}}

	matched, unmatched := matchPresetDevices(preset, nil, outputs)

	assert.Empty(t, matched)
	assert.Equal(t, []string{echowarpSinkName}, unmatched)
}

func newInputOnlyRestoreModel() SetupModel {
	m := newTestSetupModel(config.ModeServer).WithUnifiedDeviceList(false)
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.inputDevices = nil
	m.outputDevices = nil
	return m
}

func virtualAudioTestModel(outputs []deviceRow) SetupModel {
	field := NewActionField("virtual_mic", "", "Create Virtual Audio Device ▸")
	return SetupModel{Fields: []SetupField{field}, outputDevices: outputs}
}

func virtualSinkPreset(onStart recent.SinkLifecycle) *recent.VirtualSinkPreset {
	return &recent.VirtualSinkPreset{
		ModuleType: "module-null-sink",
		SinkName:   echowarpSinkName,
		OnStop:     recent.SinkDelete,
		OnStart:    onStart,
	}
}

type virtualAudioStub struct {
	foundModuleID string
	createErr     error
	findNames     []string
	createdNames  []string
	removedIDs    []string
}

func stubVirtualAudioFuncs(t *testing.T) *virtualAudioStub {
	t.Helper()
	oldLinux := isLinuxRuntime
	oldCreate := createPulseAudioSinkFn
	oldRemove := removePulseAudioSinkFn
	oldFind := findPulseAudioModuleFn
	stub := &virtualAudioStub{foundModuleID: "42"}

	isLinuxRuntime = true
	createPulseAudioSinkFn = func(name string) (string, error) {
		stub.createdNames = append(stub.createdNames, name)
		return "99", stub.createErr
	}
	removePulseAudioSinkFn = func(moduleID string) error {
		stub.removedIDs = append(stub.removedIDs, moduleID)
		return nil
	}
	findPulseAudioModuleFn = func(sinkName string) (string, bool, error) {
		stub.findNames = append(stub.findNames, sinkName)
		return stub.foundModuleID, stub.foundModuleID != "", nil
	}

	t.Cleanup(func() {
		isLinuxRuntime = oldLinux
		createPulseAudioSinkFn = oldCreate
		removePulseAudioSinkFn = oldRemove
		findPulseAudioModuleFn = oldFind
	})
	return stub
}
