package views

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
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
	assert.Equal(t, []string{echowarpSinkName, echowarpSinkName, echowarpSinkName}, stub.findNames)
	assert.Empty(t, stub.createdNames)
	assert.Equal(t, []string{"42"}, stub.removedIDs)
	assert.False(t, m.virtualMicManageable)
	assert.Equal(t, SetupOverlayNone, m.overlay)
}

func TestExistingEchoWarpExtraDoesNotBecomeManageable(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := virtualAudioTestModel([]deviceRow{{ID: 8, Name: "EchoWarpExtra"}})

	m.syncVirtualMicState()

	assert.Equal(t, []string{echowarpSinkName}, stub.findNames)
	assert.False(t, m.virtualMicManageable)
	assert.Equal(t, "Create Virtual Audio Device ▸", m.Fields[0].ActionLabel)
}

func TestAutoRecreateBeforeRefreshThenManualCreateIsIdempotent(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpMonitorName, IsInput: true, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}

	m.autoRestore(preset, "normal")
	stub.foundModuleID = "99"
	m.virtualDeviceOverlay = NewVirtualDeviceOverlay(false, echowarpSinkName)
	m.overlay = SetupOverlayVirtualDevice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, "99", m.virtualMicModule)
	assert.True(t, m.virtualMicManageable)
}

func TestManualCreateWithExistingModuleAndStaleRowsManagesOnly(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := virtualAudioTestModel(nil)
	m.virtualDeviceOverlay = NewVirtualDeviceOverlay(false, echowarpSinkName)
	m.overlay = SetupOverlayVirtualDevice

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Empty(t, stub.createdNames)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "42", m.virtualMicManagedModule)
	assert.False(t, m.virtualMicCreated)
	assert.Equal(t, SetupOverlayVirtualSinkLifecycle, m.overlay)
}

func TestRecreateUsesExistingPulseAudioModuleWithoutOutputRow(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpSinkName, IsInput: false, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}

	m.autoRestore(preset, "normal")

	assert.Empty(t, stub.createdNames)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "42", m.virtualMicManagedModule)
}

func TestDuplicateEchoWarpPresetEntriesCreateSingleSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{
		{Name: echowarpSinkName, IsInput: false, Virtual: true, VirtualSink: virtualSinkPreset(recent.SinkRecreate)},
		{Name: echowarpMonitorName, IsInput: true, Virtual: true},
	}}

	m.autoRestore(preset, "normal")

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
}

func TestLifecycleOnlyPresetRecreatesMissingVirtualSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{VirtualSinks: []recent.VirtualSinkPreset{
		*virtualSinkPreset(recent.SinkRecreate),
	}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.True(t, m.virtualSinkLifecycleConfigured)
}

func TestLifecycleOnlyPresetExistingUnknownSinkCleanupPlanKeepsManagedModule(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{VirtualSinks: []recent.VirtualSinkPreset{
		*virtualSinkPreset(recent.SinkRecreate),
	}}

	cmd := m.autoRestore(preset, "normal")
	plan := m.VirtualSinkCleanupPlan()

	assert.Nil(t, cmd)
	assert.Empty(t, stub.createdNames)
	assert.False(t, plan.Delete)
	assert.Empty(t, plan.ModuleID)
	assert.False(t, plan.AllowNameFallback)
}

func TestClientRestoreWithOnlyVirtualLifecycleBypassesEmptyDeviceReturn(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{VirtualSinks: []recent.VirtualSinkPreset{
		*virtualSinkPreset(recent.SinkRecreate),
	}}
	m.recentServers = []recent.Server{selectedPresetServer(preset)}

	cmd := m.tryShowRestoreOverlay("127.0.0.1", 4415, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
}

func TestServerRestoreWithOnlyVirtualLifecycleBypassesEmptyDeviceReturn(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	serverPresets := presetServerWithVirtualLifecycle()
	m.serverPresets = &serverPresets

	cmd := m.tryShowServerRestoreOverlay()

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
}

func TestRefreshInjectionAfterAutoRecreateSelectsMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpMonitorName, IsInput: true, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}

	m.autoRestore(preset, "normal")
	stub.foundModuleID = "99"
	m = m.WithDeviceRefreshFunc(refreshedEchoWarpItems)

	assert.Equal(t, echowarpMonitorName, m.SelectedDeviceName())
	assert.Len(t, m.inputDevices, 1)
	assert.Len(t, m.outputDevices, 1)
}

func TestMissingVirtualDevicesDoNotOpenStartupPrompt(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
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
	assert.Empty(t, stub.createdNames)
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

func TestInputOnlyHiddenOutputRecreateCreatesSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpSinkName, IsInput: false, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)
}

func TestInputOnlyInferredMonitorRecreateCreatesSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpMonitorName, IsInput: true, Virtual: true,
	}}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)
}

func TestExplicitRemoveClearsVirtualLifecyclePersistence(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := virtualAudioTestModel(nil).WithVirtualSinkLifecycle(recent.SinkDelete, recent.SinkRecreate)
	m.virtualMicManagedModule = "42"
	m.virtualMicManageable = true

	require.NoError(t, m.removeManagedVirtualMic())
	preset := m.CollectPresetDevices()

	assert.Equal(t, []string{"42"}, stub.removedIDs)
	assert.Empty(t, preset.VirtualSinks)
	assert.False(t, m.virtualSinkLifecycleConfigured)
}

func TestHiddenOutputRecreateRunsBeforeSelectionMatchSkip(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	m.inputDevices = []deviceRow{{ID: 1, Name: "Mic", IsInput: true}}
	m.multiSelect[m.inputDevices[0].selectKey()] = DeviceRoleSet{Capture: true}
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{
		{ID: 1, Name: "Mic", IsInput: true},
		{Name: echowarpSinkName, IsInput: false, Virtual: true, VirtualSink: virtualSinkPreset(recent.SinkRecreate)},
	}}
	m.recentServers = []recent.Server{{
		Address: "127.0.0.1", Port: 4415,
		Presets: map[string]recent.DevicePreset{"normal": preset},
	}}

	cmd := m.tryShowRestoreOverlay("127.0.0.1", 4415, "normal")

	assert.Nil(t, cmd)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, SetupOverlayNone, m.overlay)
	assert.Nil(t, m.restoreOverlay)
}

func TestInputOnlyHiddenOutputKeepDoesNotCreateSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpSinkName, IsInput: false, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkKeep),
	}}}

	cmd := m.autoRestore(preset, "normal")

	assert.Nil(t, cmd)
	assert.Empty(t, stub.createdNames)
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

func selectedPresetServer(preset recent.DevicePreset) recent.Server {
	return recent.Server{
		Address: "127.0.0.1",
		Port:    4415,
		Presets: map[string]recent.DevicePreset{"normal": preset},
	}
}

func presetServerWithVirtualLifecycle() preset.ServerPresets {
	return preset.ServerPresets{Presets: map[string]preset.ModePreset{
		"normal": {VirtualSinks: []recent.VirtualSinkPreset{*virtualSinkPreset(recent.SinkRecreate)}},
	}}
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
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	oldLinux := isLinuxRuntime
	oldCreate := createPulseAudioSinkFn
	oldRemove := removePulseAudioSinkFn
	oldFind := findPulseAudioModuleFn
	stub := &virtualAudioStub{}

	isLinuxRuntime = true
	createPulseAudioSinkFn = func(name string) (string, error) {
		stub.createdNames = append(stub.createdNames, name)
		return "99", stub.createErr
	}
	removePulseAudioSinkFn = func(moduleID string) error {
		stub.removedIDs = append(stub.removedIDs, moduleID)
		stub.foundModuleID = ""
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

func refreshedEchoWarpItems() ([]list.Item, error) {
	return []list.Item{
		mockDeviceItem{name: echowarpMonitorName, id: 11, isInput: true},
		mockDeviceItem{name: echowarpSinkName, id: 12, isInput: false},
	}, nil
}
