package views

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

const clientRestorePort = 4415

func TestClientLifecycleOnlyVirtualSinksDoNotSelectMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := newClientRestoreModel(t, lifecycleOnlyRecentServers())

	m = applyClientProbe(t, m, "127.0.0.1", "srv-life", "reverse")

	assert.Empty(t, stub.createdNames)
	assert.True(t, m.virtualSinkLifecycleConfigured)
	assertSelected(t, m, 1, "Real Mic", true)
	assertNotSelected(t, m, 11, echowarpMonitorName, true)
}

func TestClientSelectedVirtualMonitorPresetSelectsMonitor(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := newClientRestoreModel(t, []recent.Server{recentServerWithPreset(
		"127.0.0.1", "srv-monitor", monitorPreset(),
	)})

	m = applyClientProbe(t, m, "127.0.0.1", "srv-monitor", "reverse")

	assertSelected(t, m, 11, echowarpMonitorName, true)
}

func TestClientProbeIdentityChangeClearsStaleVirtualSelection(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := newClientRestoreModel(t, []recent.Server{
		recentServerWithPreset("127.0.0.1", "srv-a", monitorPreset()),
		recentServerWithPreset("127.0.0.2", "srv-b", lifecycleOnlyPreset()),
	})

	m = applyClientProbe(t, m, "127.0.0.1", "srv-a", "reverse")
	m = applyClientProbe(t, m, "127.0.0.2", "srv-b", "reverse")

	assertSelected(t, m, 1, "Real Mic", true)
	assertNotSelected(t, m, 11, echowarpMonitorName, true)
}

func TestClientProbeIdentityChangeClearsVirtualLifecycleRestoreState(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newClientRestoreModel(t, []recent.Server{
		recentServerWithPreset("127.0.0.1", "srv-a", virtualLifecycleOnlyPreset()),
		{Address: "127.0.0.2", Port: clientRestorePort, ServerID: "srv-b"},
	})

	m = applyClientProbe(t, m, "127.0.0.1", "srv-a", "reverse")
	require.True(t, m.virtualSinkLifecycleConfigured)
	require.NotEmpty(t, m.CollectPresetDevices().VirtualSinks)

	m = applyClientProbe(t, m, "127.0.0.2", "srv-b", "reverse")
	preset := m.CollectPresetDevices()

	assert.Equal(t, []string{echowarpSinkName}, stub.createdNames)
	assert.Empty(t, preset.VirtualSinks)
	assert.False(t, m.virtualSinkLifecycleConfigured)
	assert.Empty(t, m.virtualSinkOnStop)
	assert.Empty(t, m.virtualSinkOnStart)
	assert.False(t, m.virtualMicCreated)
	assert.Empty(t, m.virtualMicModule)
	assert.Nil(t, m.pendingVirtualSinkSelection)
	assert.True(t, m.virtualMicManageable)
	assert.Equal(t, "99", m.virtualMicManagedModule)
}

func TestClientProbeIdentitySameServerModeKeepsVirtualLifecycleState(t *testing.T) {
	selected := map[string]DeviceRoleSet{selectKeyFor(1, "Real Mic", true): {Capture: true}}
	m := clientPresetModelWithSelections(selected)
	result := &ProbeServerResult{Mode: "reverse", ServerID: "srv-a"}
	m.probeIdentity = newClientProbeIdentity("127.0.0.1", clientRestorePort, result)
	m.virtualSinkOnStop = recent.SinkKeep
	m.virtualSinkOnStart = recent.SinkRecreate
	m.virtualSinkLifecycleConfigured = true
	m.pendingVirtualSinkSelection = map[string]bool{echowarpSinkName: true}

	m.resetClientSelectionOnProbeChange("127.0.0.1", clientRestorePort, result)

	assert.True(t, m.virtualSinkLifecycleConfigured)
	assert.Equal(t, recent.SinkKeep, m.virtualSinkOnStop)
	assert.Equal(t, recent.SinkRecreate, m.virtualSinkOnStart)
	assert.True(t, m.pendingVirtualSinkSelection[echowarpSinkName])
	assertSelected(t, m, 1, "Real Mic", true)
}

func TestClientSaveAfterUncheckingMonitorUsesLifecycleOnlyVirtualSink(t *testing.T) {
	m := clientPresetModelWithSelections(map[string]DeviceRoleSet{
		selectKeyFor(1, "Real Mic", true): {Capture: true},
	}).WithVirtualSinkLifecycle(recent.SinkDelete, recent.SinkRecreate)

	p := m.CollectPresetDevices()

	require.Len(t, p.Devices, 1)
	assert.Equal(t, "Real Mic", p.Devices[0].Name)
	assert.Empty(t, deviceNamesMatching(p.Devices, echowarpMonitorName))
	require.Len(t, p.VirtualSinks, 1)
	assert.Equal(t, echowarpSinkName, p.VirtualSinks[0].SinkName)
}

func TestClientRestoreMatchesRecentServerByServerIDAfterAddressChange(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	stub.foundModuleID = "42"
	m := newClientRestoreModel(t, []recent.Server{recentServerWithPreset(
		"192.168.1.10", "stable-server", lifecycleOnlyPreset(),
	)})

	m = applyClientProbe(t, m, "10.0.0.25", "stable-server", "reverse")

	assertSelected(t, m, 1, "Real Mic", true)
}

func TestClientRestoreDoesNotUseLegacyPresetWhenProbeHasServerID(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	m := newClientRestoreModel(t, []recent.Server{recentServerWithPreset(
		"127.0.0.1", "", monitorPreset(),
	)})

	m = applyClientProbe(t, m, "127.0.0.1", "new-server", "reverse")

	assert.Empty(t, stub.createdNames)
	assertNotSelected(t, m, 11, echowarpMonitorName, true)
}

func TestSplitProbeAddressHandlesIPv6(t *testing.T) {
	host, port := splitProbeAddress("[2001:db8::1]:4415")
	assert.Equal(t, "2001:db8::1", host)
	assert.Equal(t, clientRestorePort, port)

	host, port = splitProbeAddress("2001:db8::1")
	assert.Equal(t, "2001:db8::1", host)
	assert.Zero(t, port)
}

func TestProbeIdentityUsesSeparatedIPv6HostPortFields(t *testing.T) {
	selected := map[string]DeviceRoleSet{selectKeyFor(11, echowarpMonitorName, true): {Capture: true}}
	m := clientPresetModelWithSelections(selected)
	m.Fields = []SetupField{
		{Key: "server_address", Value: "2001:db8::1"},
		{Key: "port", Value: "4415"},
	}
	result := &ProbeServerResult{Mode: "reverse"}
	m.probeIdentity = newClientProbeIdentity("2001:db8::1", clientRestorePort, result)

	host, port := probeAddressFromFields(m.Fields, "2001:db8::1:4415")
	assert.Equal(t, "2001:db8::1", host)
	assert.Equal(t, clientRestorePort, port)
	m.resetClientSelectionOnProbeChange(host, port, result)

	assertSelected(t, m, 11, echowarpMonitorName, true)
}

func TestClientProbeIdentityRequiresBothServerIDs(t *testing.T) {
	legacy := clientProbeIdentity{address: "10.0.0.1", port: clientRestorePort, mode: "reverse"}
	identified := clientProbeIdentity{address: "10.0.0.1", port: clientRestorePort, mode: "reverse", serverID: "server-a"}

	assert.False(t, legacy.matches(identified))
	assert.False(t, identified.matches(legacy))
	assert.True(t, legacy.matches(clientProbeIdentity{address: "10.0.0.1", port: clientRestorePort, mode: "reverse"}))
}

func newClientRestoreModel(t *testing.T, servers []recent.Server) SetupModel {
	t.Helper()
	dir := t.TempDir()
	writeRecentServers(t, dir, servers)
	devices := []mockDeviceItem{
		{name: "Real Mic", id: 1, isInput: true},
		{name: echowarpMonitorName, id: 11, isInput: true},
		{name: echowarpSinkName, id: 12, isInput: false},
	}
	return newClientModel(t, dir, devices)
}

func applyClientProbe(t *testing.T, m SetupModel, addr, serverID, mode string) SetupModel {
	t.Helper()
	setClientServerFields(&m, addr, clientRestorePort)
	msg := ProbeServerMsg{Result: &ProbeServerResult{Mode: mode, ServerID: serverID}}
	msg.Addr = fmt.Sprintf("%s:%d", addr, clientRestorePort)
	updated, _ := m.Update(msg)
	return updated
}

func setClientServerFields(m *SetupModel, addr string, port int) {
	for i := range m.Fields {
		switch m.Fields[i].Key {
		case "server_address":
			m.Fields[i].Value = addr
		case "port":
			m.Fields[i].Value = fmt.Sprintf("%d", port)
		}
	}
}

func lifecycleOnlyRecentServers() []recent.Server {
	return []recent.Server{recentServerWithPreset("127.0.0.1", "srv-life", lifecycleOnlyPreset())}
}

func recentServerWithPreset(addr, serverID string, preset recent.DevicePreset) recent.Server {
	return recent.Server{
		Address:  addr,
		Port:     clientRestorePort,
		ServerID: serverID,
		Presets:  map[string]recent.DevicePreset{"reverse": preset},
	}
}

func lifecycleOnlyPreset() recent.DevicePreset {
	return recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 1, Name: "Real Mic", IsInput: true}},
		VirtualSinks: []recent.VirtualSinkPreset{
			*virtualSinkPreset(recent.SinkRecreate),
		},
	}
}

func virtualLifecycleOnlyPreset() recent.DevicePreset {
	return recent.DevicePreset{VirtualSinks: []recent.VirtualSinkPreset{
		*virtualSinkPreset(recent.SinkRecreate),
	}}
}

func monitorPreset() recent.DevicePreset {
	return recent.DevicePreset{Devices: []recent.PresetDevice{{
		ID:          11,
		Name:        echowarpMonitorName,
		IsInput:     true,
		Virtual:     true,
		VirtualSink: virtualSinkPreset(recent.SinkRecreate),
	}}}
}

func clientPresetModelWithSelections(selected map[string]DeviceRoleSet) SetupModel {
	m := newSetupModelForPresets(
		[]deviceRow{
			{ID: 1, Name: "Real Mic", IsInput: true},
			{ID: 11, Name: echowarpMonitorName, IsInput: true, IsVirtual: true},
		},
		nil,
		selected,
	)
	m.cfg.Mode = config.ModeClient
	return m
}

func assertSelected(t *testing.T, m SetupModel, id uint32, name string, isInput bool) {
	t.Helper()
	_, selected := m.multiSelect[selectKeyFor(id, name, isInput)]
	assert.True(t, selected, "%s should be selected", name)
}

func assertNotSelected(t *testing.T, m SetupModel, id uint32, name string, isInput bool) {
	t.Helper()
	_, selected := m.multiSelect[selectKeyFor(id, name, isInput)]
	assert.False(t, selected, "%s should not be selected", name)
}

func deviceNamesMatching(devices []recent.PresetDevice, name string) []string {
	var matches []string
	for _, device := range devices {
		if device.Name == name {
			matches = append(matches, device.Name)
		}
	}
	return matches
}
