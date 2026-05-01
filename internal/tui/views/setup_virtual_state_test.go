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
	client.virtualDeviceOverlay = NewVirtualDeviceOverlay(false, echowarpSinkName)
	client.overlay = SetupOverlayVirtualDevice

	client, _ = client.Update(tea.KeyMsg{Type: tea.KeyEnter})
	device := mustLoadVirtualStateDevice(t)
	plan := client.VirtualSinkCleanupPlan()

	assert.Equal(t, "EchoWarp virtual audio device already exists and is owned by server", client.virtualDeviceOverlay.Error)
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
	server.virtualDeviceOverlay = NewVirtualDeviceOverlay(false, echowarpSinkName)
	server.overlay = SetupOverlayVirtualDevice

	server, _ = server.Update(tea.KeyMsg{Type: tea.KeyEnter})
	device := mustLoadVirtualStateDevice(t)

	assert.Contains(t, server.virtualDeviceOverlay.Error, "owned by client")
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

func TestCurrentRoleExplicitRemoveMarksAbsentAndSuppressesLegacyRecreate(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleClient, "42", recent.SinkKeep, recent.SinkKeep)
	client := newVirtualStateModel(config.ModeClient)
	client.virtualMicManageable = true
	client.virtualMicManagedModule = "42"

	require.NoError(t, client.removeManagedVirtualMic())
	device := mustLoadVirtualStateDevice(t)
	client.autoRestore(legacyVirtualSinkPreset(recent.SinkDelete, recent.SinkRecreate), "normal")

	assert.Equal(t, []string{"42"}, stub.removedIDs)
	assert.Equal(t, virtualstate.DesiredAbsent, device.State.Desired)
	assert.Equal(t, virtualstate.RoleUser, device.Ownership.CreatedBy)
	assert.Empty(t, stub.createdNames)
}

func TestImportedExplicitRemoveMarksAbsent(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	writeVirtualState(t, virtualstate.RoleImported, "51", recent.SinkKeep, recent.SinkKeep)
	server := newVirtualStateModel(config.ModeServer)
	server.virtualMicManageable = true
	server.virtualMicManagedModule = "51"

	require.NoError(t, server.removeManagedVirtualMic())
	device := mustLoadVirtualStateDevice(t)

	assert.Equal(t, []string{"51"}, stub.removedIDs)
	assert.Equal(t, virtualstate.DesiredAbsent, device.State.Desired)
	assert.Equal(t, virtualstate.RoleUser, device.Ownership.CreatedBy)
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
