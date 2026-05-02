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
	"github.com/lHumaNl/echowarp/internal/virtualstate"
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

func TestRefreshInjectionAfterAutoRecreateDoesNotSelectMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newInputOnlyRestoreModel()
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: echowarpMonitorName, IsInput: true, Virtual: true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}

	m.autoRestore(preset, "normal")
	stub.foundModuleID = "99"
	m = m.WithDeviceRefreshFunc(refreshedEchoWarpItems)

	assert.Empty(t, m.SelectedDeviceName())
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

func TestRemoveManagedVirtualSinkPreservesRemainingTrackedSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModules = map[string]string{"custom_a": "42"}
	m := newTestSetupModel(config.ModeServer)
	first := customVirtualSinkPreset("custom_a", "Studio A")
	second := customVirtualSinkPreset("custom_b", "Studio B")
	m.trackVirtualSink("42", first, true)
	m.trackVirtualSink("43", second, true)

	require.NoError(t, m.removeManagedVirtualSink(first.SinkName))

	assert.Equal(t, []string{"42"}, stub.removedIDs)
	assert.NotContains(t, m.trackedVirtualSinks, first.SinkName)
	assert.Contains(t, m.trackedVirtualSinks, second.SinkName)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "43", m.virtualMicManagedModule)
	assert.Equal(t, second.SinkName, m.defaultVirtualOverlaySinkName())
}

func TestClientResetPreservesCurrentSessionTrackedVirtualSinks(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	m := newVirtualStateModel(config.ModeClient)
	vs := customVirtualSinkPreset("custom_session", "Session Device")
	require.NoError(t, m.persistVirtualSinkPresent("88", vs))
	m.trackedVirtualSinks = nil

	m.clearClientVirtualRestoreState()

	tracked, ok := m.trackedVirtualSinks[vs.SinkName]
	require.True(t, ok)
	assert.True(t, tracked.CreatedThisSession)
	assert.Equal(t, "88", tracked.ModuleID)
	assert.Len(t, m.VirtualSinkCleanupPlans(), 1)
}

func TestSyncVirtualMicStateDiscoversTrackedCustomSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModules = map[string]string{"custom_b": "43"}
	m := newTestSetupModel(config.ModeServer)
	vs := customVirtualSinkPreset("custom_b", "Studio B")
	m.trackVirtualSink("", vs, false)

	m.syncVirtualMicState()

	assert.Equal(t, []string{vs.SinkName}, stub.findNames[:1])
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "43", m.virtualMicManagedModule)
}

func TestSyncVirtualMicStateDiscoversPreviousSessionCustomSinkSafely(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	vs := customVirtualSinkPreset("custom_previous", "Previous Studio")
	stub.foundModules = map[string]string{vs.SinkName: "44"}
	previousSessionID := virtualstate.NewSessionID()
	policy := virtualstate.DevicePolicy{OnStop: recent.SinkDelete, OnStart: recent.SinkRecreate}
	require.NoError(t, virtualstate.UpsertPresentWithMetadata(
		vs.SinkName,
		virtualSinkMonitorName(vs),
		"43",
		virtualstate.RoleServer,
		policy,
		virtualstate.DeviceMetadata{
			ID: vs.ID, BaseName: vs.BaseName, PlaybackName: vs.PlaybackName,
			CaptureName: vs.CaptureName, SessionID: previousSessionID,
		},
	))
	m := newTestSetupModel(config.ModeServer)

	m.syncVirtualMicState()
	device, ok, err := virtualstate.LoadDevice(vs.SinkName)

	require.NoError(t, err)
	require.True(t, ok)
	assert.Contains(t, stub.findNames, vs.SinkName)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "44", m.virtualMicManagedModule)
	assert.Empty(t, m.VirtualSinkCleanupPlans())
	assert.Equal(t, previousSessionID, device.Ownership.SessionID)
}

func TestVirtualOverlayUsesCustomCaptureNameForExistingSink(t *testing.T) {
	stubVirtualAudioFuncs(t)
	m := newTestSetupModel(config.ModeServer)
	vs := customVirtualSinkPreset("custom_capture", "Capture Rig")
	m.trackVirtualSink("45", vs, false)
	m.syncVirtualMicState()

	m, _ = m.openVirtualMicOverlay()

	require.NotNil(t, m.virtualDeviceOverlay)
	assert.Equal(t, vs.CaptureName, m.virtualDeviceOverlay.CaptureName)
	assert.Contains(t, m.virtualDeviceOverlay.View(80), vs.CaptureName)
}

func TestCollectVirtualSinkPresetsUsesDeterministicSinkOrder(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	second := customVirtualSinkPreset("custom_b", "Studio B")
	first := customVirtualSinkPreset("custom_a", "Studio A")
	m.trackVirtualSink("2", second, true)
	m.trackVirtualSink("1", first, true)

	presets := m.CollectVirtualSinkPresets()

	require.Len(t, presets, 2)
	assert.Equal(t, []string{first.SinkName, second.SinkName}, []string{presets[0].SinkName, presets[1].SinkName})
}

func TestCurrentSelectionMatchesPresetUsesVirtualAliases(t *testing.T) {
	m := newTestSetupModel(config.ModeClient)
	vs := customVirtualSinkPreset("custom_playback", "Studio Playback")
	row := deviceRow{ID: 8, Name: vs.PlaybackName, IsVirtual: true}
	m.outputDevices = []deviceRow{row}
	m.multiSelect = map[string]DeviceRoleSet{row.selectKey(): {Playback: true}}
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		Name: vs.SinkName, IsInput: false, Virtual: true, VirtualSink: &vs,
	}}}

	assert.True(t, m.currentSelectionMatchesPreset(preset))
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

func TestInputOnlyVirtualCreationSelectionHelperIsUnused(t *testing.T) {
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
	foundModules  map[string]string
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
	createPulseAudioSinkFn = func(vs recent.VirtualSinkPreset) (string, error) {
		stub.createdNames = append(stub.createdNames, vs.SinkName)
		return "99", stub.createErr
	}
	removePulseAudioSinkFn = func(moduleID string) error {
		stub.removedIDs = append(stub.removedIDs, moduleID)
		stub.foundModuleID = ""
		return nil
	}
	findPulseAudioModuleFn = func(sinkName string) (string, bool, error) {
		stub.findNames = append(stub.findNames, sinkName)
		if moduleID, ok := stub.foundModules[sinkName]; ok {
			return moduleID, moduleID != "", nil
		}
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

func customVirtualSinkPreset(sinkName, baseName string) recent.VirtualSinkPreset {
	return recent.VirtualSinkPreset{
		ID: sinkName + "_id", BaseName: baseName, ModuleType: "module-null-sink",
		SinkName: sinkName, MonitorName: sinkName + ".monitor",
		PlaybackName: "Playback " + baseName, CaptureName: "Capture " + baseName,
		OnStop: recent.SinkDelete, OnStart: recent.SinkRecreate,
	}
}
