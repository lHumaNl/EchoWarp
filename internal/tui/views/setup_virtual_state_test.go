package views

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/virtualstate"
)

func TestServerOwnedKeepSinkIsNotDeletedByClientCleanup(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	stub := stubVirtualAudioFuncs(t)
	server := newVirtualStateModel(config.ModeServer)

	require.NoError(t, server.createAndTrackVirtualSink(*virtualSinkPresetWithLifecycle(
		recent.SinkKeep, recent.SinkKeep,
	)))
	stub.foundModuleID = "99"
	client := newVirtualStateModel(config.ModeClient)
	client.autoRestore(legacyVirtualSinkPreset(recent.SinkDelete, recent.SinkRecreate), "normal")

	plan := client.VirtualSinkCleanupPlan()

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.False(t, plan.Delete)
	assert.Empty(t, plan.ModuleID)
}

func TestClientLegacyPresetDoesNotDeleteServerOwnedEchoWarp(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleServer, "42", recent.SinkKeep, recent.SinkKeep)
	stub.foundModuleID = "42"
	client := newVirtualStateModel(config.ModeClient)

	client.autoRestore(legacyVirtualSinkPreset(recent.SinkDelete, recent.SinkRecreate), "normal")
	plan := client.VirtualSinkCleanupPlan()

	assert.Empty(t, stub.createdNames)
	assert.False(t, plan.Delete)
}

func TestClientManualCreateServerOwnedSinkReturnsOwnershipError(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleServer, "42", recent.SinkKeep, recent.SinkKeep)
	stub.foundModuleID = "42"
	client := newVirtualStateModel(config.ModeClient)

	err := client.ensureVirtualSink(*client.defaultVirtualSinkPreset(), virtualSinkEnsureOptions{explicitCreate: true})
	device := mustLoadVirtualStateDevice(t)
	plan := client.VirtualSinkCleanupPlan()

	require.EqualError(t, err, "EchoWarp virtual audio device already exists and is owned by server")
	assert.Equal(t, virtualstate.RoleServer, device.Ownership.CreatedBy)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStop)
	assert.Empty(t, stub.createdNames)
	assert.False(t, plan.Delete)
	assert.Empty(t, plan.ModuleID)
}

func TestServerManualCreateClientOwnedSinkDoesNotStealOwnership(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleClient, "24", recent.SinkKeep, recent.SinkKeep)
	stub.foundModuleID = "24"
	server := newVirtualStateModel(config.ModeServer)

	err := server.ensureVirtualSink(*server.defaultVirtualSinkPreset(), virtualSinkEnsureOptions{explicitCreate: true})
	device := mustLoadVirtualStateDevice(t)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "owned by client")
	assert.Equal(t, virtualstate.RoleClient, device.Ownership.CreatedBy)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStop)
	assert.Empty(t, stub.createdNames)
}

func TestLegacyRecreateWithOtherRoleOwnedSinkDoesNotStealOwnership(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleClient, "24", recent.SinkKeep, recent.SinkKeep)
	stub.foundModuleID = "24"
	server := newVirtualStateModel(config.ModeServer)

	server.autoRestore(legacyVirtualSinkPreset(recent.SinkDelete, recent.SinkRecreate), "normal")
	device := mustLoadVirtualStateDevice(t)

	assert.Empty(t, stub.createdNames)
	assert.Equal(t, virtualstate.RoleClient, device.Ownership.CreatedBy)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStop)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStart)
}

func TestClientOwnedDeleteRecreateSinkDeletesOnClientCleanup(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	stub := stubVirtualAudioFuncs(t)
	client := newVirtualStateModel(config.ModeClient)

	require.NoError(t, client.createAndTrackVirtualSink(*virtualSinkPresetWithLifecycle(
		recent.SinkDelete, recent.SinkRecreate,
	)))
	plan := client.VirtualSinkCleanupPlan()
	device := mustLoadVirtualStateDevice(t)

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, virtualstate.RoleClient, device.Ownership.CreatedBy)
	assert.True(t, plan.Delete)
	assert.Equal(t, "99", plan.ModuleID)
}

func TestClientCannotExplicitlyRemoveServerOwnedSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleServer, "42", recent.SinkKeep, recent.SinkKeep)
	client := newVirtualStateModel(config.ModeClient)
	client.virtualMicManageable = true
	client.virtualMicManagedModule = "42"

	err := client.removeManagedVirtualMic()
	device := mustLoadVirtualStateDevice(t)

	require.EqualError(t, err, "EchoWarp virtual audio device is owned by server")
	assert.Empty(t, stub.removedIDs)
	assert.Equal(t, virtualstate.DesiredPresent, device.State.Desired)
	assert.Equal(t, virtualstate.RoleServer, device.Ownership.CreatedBy)
	assert.Equal(t, "42", device.State.ModuleID)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStop)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStart)
}

func TestServerCannotExplicitlyRemoveClientOwnedSink(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleClient, "24", recent.SinkKeep, recent.SinkKeep)
	server := newVirtualStateModel(config.ModeServer)
	server.virtualMicManageable = true
	server.virtualMicManagedModule = "24"

	err := server.removeManagedVirtualMic()
	device := mustLoadVirtualStateDevice(t)

	require.EqualError(t, err, "EchoWarp virtual audio device is owned by client")
	assert.Empty(t, stub.removedIDs)
	assert.Equal(t, virtualstate.DesiredPresent, device.State.Desired)
	assert.Equal(t, virtualstate.RoleClient, device.Ownership.CreatedBy)
	assert.Equal(t, "24", device.State.ModuleID)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStop)
	assert.Equal(t, recent.SinkKeep, device.Policy.OnStart)
}

func TestClientDiscoversServerOwnedCustomSinkButCleanupIsEmpty(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	vs := customVirtualSinkPreset("echowarp_server_studio", "Server Studio")
	writeCustomVirtualState(t, virtualstate.RoleServer, "42", vs)
	stub.foundModules = map[string]string{vs.SinkName: "42"}
	client := newVirtualStateModel(config.ModeClient)

	client.syncVirtualMicState()
	m, _ := client.openVirtualMicOverlay()
	tracked, ok := client.trackedVirtualSinks[vs.SinkName]

	require.True(t, ok)
	assert.True(t, tracked.Manageable)
	assert.Equal(t, "42", tracked.ModuleID)
	assert.True(t, client.virtualMicManageable)
	assert.Empty(t, client.VirtualSinkCleanupPlans())
	require.NotNil(t, m.virtualDeviceOverlay)
	require.Len(t, m.virtualDeviceOverlay.Devices, 1)
	assert.Equal(t, vs.SinkName, m.virtualDeviceOverlay.Devices[0].SinkName)
	assert.False(t, m.virtualDeviceOverlay.Devices[0].Removable)
}

func TestClientOverlayShowsServerOwnedSinkAsNonRemovable(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	vs := customVirtualSinkPreset("echowarp_server_owned", "Server Owned")
	writeCustomVirtualState(t, virtualstate.RoleServer, "42", vs)
	stub.foundModules = map[string]string{vs.SinkName: "42"}
	client := newVirtualStateModel(config.ModeClient)
	m, _ := client.openVirtualMicOverlay()

	view := m.virtualDeviceOverlay.View(90)
	action := m.virtualDeviceOverlay.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Contains(t, view, "owned by server (not removable)")
	assert.Equal(t, VirtualActionNone, action)
	assert.Contains(t, m.virtualDeviceOverlay.Error, "owned by server")
	assert.Empty(t, stub.removedIDs)
}

func TestVirtualOverlayDoesNotListStaleStateSink(t *testing.T) {
	stubVirtualAudioFuncs(t)
	vs := customVirtualSinkPreset("echowarp_stale_state", "Stale State")
	writeCustomVirtualState(t, virtualstate.RoleServer, "42", vs)
	m := newVirtualStateModel(config.ModeServer)

	m, _ = m.openVirtualMicOverlay()

	require.NotNil(t, m.virtualDeviceOverlay)
	assert.Empty(t, m.virtualDeviceOverlay.Devices)
	assert.True(t, m.virtualDeviceOverlay.IsCreateMode())
	assert.NotContains(t, m.virtualDeviceOverlay.View(90), "Remove "+vs.PlaybackName)
}

func TestCurrentRoleExplicitRemoveDeletesVirtualStateRecord(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleClient, "42", recent.SinkKeep, recent.SinkKeep)
	client := newVirtualStateModel(config.ModeClient)
	stub.foundModuleID = "42"
	client.virtualMicManageable = true
	client.virtualMicManagedModule = "42"

	require.NoError(t, client.removeManagedVirtualMic())
	_, ok, err := virtualstate.LoadDevice(echowarpSinkName)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"42"}, stub.removedIDs)
}

func TestImportedExplicitRemoveDeletesVirtualStateRecord(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleImported, "51", recent.SinkKeep, recent.SinkKeep)
	server := newVirtualStateModel(config.ModeServer)
	stub.foundModuleID = "51"
	server.virtualMicManageable = true
	server.virtualMicManagedModule = "51"

	require.NoError(t, server.removeManagedVirtualMic())
	_, ok, err := virtualstate.LoadDevice(echowarpSinkName)

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"51"}, stub.removedIDs)
}

func TestDesiredAbsentSuppressesLegacyPresetRecreate(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	stub := stubVirtualAudioFuncs(t)
	require.NoError(t, virtualstate.MarkAbsent(echowarpSinkName, virtualstate.RoleUser))
	m := newVirtualStateModel(config.ModeClient)

	m.autoRestore(legacyVirtualSinkPreset(recent.SinkDelete, recent.SinkRecreate), "normal")

	assert.Empty(t, stub.createdNames)
}

func TestVirtualStateRecreateDoesNotSelectMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleServer, "", recent.SinkDelete, recent.SinkRecreate)
	m := newVirtualStateModel(config.ModeServer)
	m.inputDevices = []deviceRow{{ID: 1, Name: echowarpMonitorName, IsInput: true, IsVirtual: true}}
	m.outputDevices = []deviceRow{{ID: 2, Name: echowarpSinkName, IsVirtual: true}}

	m.recreateVirtualSinksFromState(nil)

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Empty(t, m.SelectedDeviceName())
	assert.Empty(t, m.multiSelect)
}

func TestServerKeepNoRecreateStateKeepsMonitorUnchecked(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleServer, "42", recent.SinkKeep, recent.SinkKeep)
	m := newVirtualStateModel(config.ModeServer)
	m.inputDevices = []deviceRow{{ID: 1, Name: echowarpMonitorName, IsInput: true, IsVirtual: true}}

	m.recreateVirtualSinksFromState(nil)
	plan := m.VirtualSinkCleanupPlan()

	assert.Empty(t, stub.createdNames)
	assert.Empty(t, m.SelectedDeviceName())
	assert.False(t, plan.Delete)
}

func TestTrackedVirtualSinkCleanupPlansIncludeEachCreatedDeleteSink(t *testing.T) {
	m := newVirtualStateModel(config.ModeServer)
	keep := m.virtualSinkPresetForBaseName("Keep Device")
	keep.OnStop = recent.SinkKeep
	deleteOne := m.virtualSinkPresetForBaseName("Delete One")
	deleteTwo := m.virtualSinkPresetForBaseName("Delete Two")
	m.trackVirtualSink("41", keep, true)
	m.trackVirtualSink("42", deleteOne, true)
	m.trackVirtualSink("43", deleteTwo, true)

	plans := m.VirtualSinkCleanupPlans()

	require.Len(t, plans, 2)
	assert.ElementsMatch(t, []string{deleteOne.SinkName, deleteTwo.SinkName}, []string{plans[0].SinkName, plans[1].SinkName})
	assert.ElementsMatch(t, []string{"42", "43"}, []string{plans[0].ModuleID, plans[1].ModuleID})
	for _, plan := range plans {
		assert.True(t, plan.Delete)
		assert.True(t, plan.AllowNameFallback)
	}
}

func TestMarkVirtualSinkCleanedByNameOnlyMarksTargetDeviceAbsent(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	m := newVirtualStateModel(config.ModeServer)
	first := m.virtualSinkPresetForBaseName("Studio One")
	second := m.virtualSinkPresetForBaseName("Studio Two")
	require.NoError(t, m.persistVirtualSinkPresent("41", first))
	require.NoError(t, m.persistVirtualSinkPresent("42", second))
	m.trackVirtualSink("41", first, true)
	m.trackVirtualSink("42", second, true)

	require.NoError(t, m.MarkVirtualSinkCleanedByName(first.SinkName))
	firstDevice, ok, err := virtualstate.LoadDevice(first.SinkName)
	require.NoError(t, err)
	require.True(t, ok)
	secondDevice, ok, err := virtualstate.LoadDevice(second.SinkName)
	require.NoError(t, err)
	require.True(t, ok)

	assert.Equal(t, virtualstate.DesiredAbsent, firstDevice.State.Desired)
	assert.Equal(t, virtualstate.DesiredPresent, secondDevice.State.Desired)
	assert.NotContains(t, m.trackedVirtualSinks, first.SinkName)
	assert.Contains(t, m.trackedVirtualSinks, second.SinkName)
}

func TestStateVirtualSinkCleanupPlanRequiresExactSessionID(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	m := newVirtualStateModel(config.ModeServer)
	vs := m.virtualSinkPresetForBaseName("Session Scoped")
	policy := virtualstate.DevicePolicy{OnStop: recent.SinkDelete, OnStart: recent.SinkRecreate}
	require.NoError(t, virtualstate.UpsertPresentWithMetadata(
		vs.SinkName,
		virtualSinkMonitorName(vs),
		"51",
		virtualstate.RoleServer,
		policy,
		virtualstate.DeviceMetadata{ID: vs.ID, BaseName: vs.BaseName, SessionID: virtualstate.NewSessionID()},
	))

	plans := m.VirtualSinkCleanupPlans()

	assert.Empty(t, plans)
}

func TestCreateVirtualSinkReturnsPersistenceError(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	setConfigDirToFile(t)
	m := newVirtualStateModel(config.ModeServer)

	err := m.createAndTrackVirtualSink(*virtualSinkPresetWithLifecycle(recent.SinkDelete, recent.SinkRecreate))

	require.Error(t, err)
	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Equal(t, []string{"99"}, stub.removedIDs)
	assert.False(t, m.virtualMicCreated)
}

func TestExplicitRemoveReturnsPersistenceError(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	setConfigDirToFile(t)
	m := newVirtualStateModel(config.ModeClient)
	m.virtualMicManageable = true
	m.virtualMicManagedModule = "42"

	err := m.removeManagedVirtualMic()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load virtual audio device state")
	assert.Empty(t, stub.removedIDs)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "42", m.virtualMicManagedModule)
}

func setConfigDirToFile(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config-file")
	require.NoError(t, os.WriteFile(path, []byte("not a directory"), 0o600))
	t.Setenv("ECHOWARP_CONFIG_DIR", path)
}

func newVirtualStateModel(mode config.Mode) SetupModel {
	m := newTestSetupModel(mode)
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.inputDevices = nil
	m.outputDevices = nil
	return m
}

func legacyVirtualSinkPreset(onStop, onStart recent.SinkLifecycle) recent.DevicePreset {
	return recent.DevicePreset{VirtualSinks: []recent.VirtualSinkPreset{
		*virtualSinkPresetWithLifecycle(onStop, onStart),
	}}
}

func writeVirtualState(
	t *testing.T,
	owner string,
	moduleID string,
	onStop recent.SinkLifecycle,
	onStart recent.SinkLifecycle,
) {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	policy := virtualstate.DevicePolicy{OnStop: onStop, OnStart: onStart}
	require.NoError(t, virtualstate.UpsertPresent(echowarpSinkName, echowarpMonitorName, moduleID, owner, policy))
}

func mustLoadVirtualStateDevice(t *testing.T) virtualstate.Device {
	t.Helper()
	device, ok, err := virtualstate.LoadDevice(echowarpSinkName)
	require.NoError(t, err)
	require.True(t, ok)
	return device
}
