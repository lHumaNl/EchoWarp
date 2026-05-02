package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

type backendDeviceItem struct {
	name      string
	id        uint32
	isInput   bool
	backendID string
}

func (d backendDeviceItem) Title() string           { return d.name }
func (d backendDeviceItem) Description() string     { return "" }
func (d backendDeviceItem) FilterValue() string     { return d.name }
func (d backendDeviceItem) DeviceID() uint32        { return d.id }
func (d backendDeviceItem) IsInputDevice() bool     { return d.isInput }
func (d backendDeviceItem) DeviceBackendID() string { return d.backendID }

func TestManagedVirtualOutputsResolveByBackendID(t *testing.T) {
	echowarp := customVirtualSinkPreset("sink_echo", "EchoWarp")
	asd := customVirtualSinkPreset("sink_asd", "Asd")
	m := backendIdentityModel(t, []recent.VirtualSinkPreset{echowarp, asd}, []backendDeviceItem{
		{name: genericPlaybackName, id: 10, backendID: asd.SinkName},
		{name: genericPlaybackName, id: 11, backendID: echowarp.SinkName},
	})

	rows := outputRowsByBackendID(m.outputDevices)

	require.Len(t, rows, 2)
	assertVirtualRow(t, rows[echowarp.SinkName], "Playback EchoWarp", echowarp.SinkName)
	assertVirtualRow(t, rows[asd.SinkName], "Playback Asd", asd.SinkName)
	assert.Empty(t, m.multiSelect, "classification must not auto-select devices")
}

func TestManagedVirtualInputsResolveByBackendID(t *testing.T) {
	echowarp := customVirtualSinkPreset("sink_echo", "EchoWarp")
	asd := customVirtualSinkPreset("sink_asd", "Asd")
	m := backendIdentityModel(t, []recent.VirtualSinkPreset{echowarp, asd}, []backendDeviceItem{
		{name: genericPlaybackMonitorName, id: 20, isInput: true, backendID: asd.MonitorName},
		{name: genericPlaybackMonitorName, id: 21, isInput: true, backendID: echowarp.SinkName + ".monitor"},
	})

	rows := inputRowsByBackendID(m.inputDevices)

	require.Len(t, rows, 2)
	assertVirtualRow(t, rows[echowarp.SinkName+".monitor"], "Capture EchoWarp", echowarp.SinkName)
	assertVirtualRow(t, rows[asd.MonitorName], "Capture Asd", asd.SinkName)
}

func TestGenericRowWithNonMatchingBackendIDIsNotVirtual(t *testing.T) {
	echo := customVirtualSinkPreset("sink_echo", "EchoWarp")
	m := backendIdentityModel(t, []recent.VirtualSinkPreset{echo}, []backendDeviceItem{
		{name: genericPlaybackName, id: 10, backendID: "unrelated_sink"},
	})

	require.Len(t, m.outputDevices, 1)
	assert.Equal(t, genericPlaybackName, m.outputDevices[0].Name)
	assert.False(t, m.outputDevices[0].IsVirtual)
	assert.Empty(t, m.outputDevices[0].VirtualSinkName)
}

func TestDuplicateGenericRowsWithoutBackendIDRemainConservative(t *testing.T) {
	echo := customVirtualSinkPreset("sink_echo", "EchoWarp")
	m := backendIdentityModel(t, []recent.VirtualSinkPreset{echo}, []backendDeviceItem{
		{name: genericPlaybackName, id: 10},
		{name: genericPlaybackName, id: 11},
		{name: genericPlaybackMonitorName, id: 20, isInput: true},
		{name: genericPlaybackMonitorName, id: 21, isInput: true},
	})

	assertGenericRows(t, m.outputDevices, []string{"Playback #1", "Playback #2"})
	assertGenericRows(t, m.inputDevices, []string{"Monitor of Playback #1", "Monitor of Playback #2"})
}

func TestVirtualPresetCollectionUsesVirtualSinkIdentity(t *testing.T) {
	vs := customVirtualSinkPreset("sink_echo", "EchoWarp")
	m := backendIdentityModel(t, []recent.VirtualSinkPreset{vs}, []backendDeviceItem{
		{name: genericPlaybackName, id: 10, backendID: vs.SinkName},
	})
	m.outputDevices[0].Name = "Renamed display"
	m.multiSelect[m.outputDevices[0].selectKey()] = DeviceRoleSet{Playback: true}

	preset := m.CollectPresetDevices()

	require.Len(t, preset.Devices, 1)
	require.NotNil(t, preset.Devices[0].VirtualSink)
	assert.Equal(t, vs.SinkName, preset.Devices[0].VirtualSink.SinkName)
}

func TestVirtualRestoreMatchesByVirtualSinkIdentity(t *testing.T) {
	vs := customVirtualSinkPreset("sink_echo", "EchoWarp")
	row := deviceRow{ID: 44, Name: "Renamed display", VirtualSinkName: vs.SinkName, IsVirtual: true}
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		ID: 1, Name: "Old display", Virtual: true, VirtualSink: &vs, Volume: 0.7,
	}}}

	matched, unmatched := matchPresetDevices(preset, nil, []deviceRow{row})

	assert.Empty(t, unmatched)
	require.Len(t, matched, 1)
	assert.Equal(t, row.selectKey(), matched[0].selectKey())
}

func TestRestoreMixInputsMatchesVirtualOutputByIdentityBeforeIDOrName(t *testing.T) {
	echo := customVirtualSinkPreset("sink_echo", "EchoWarp")
	asd := customVirtualSinkPreset("sink_asd", "Asd")
	mic := deviceRow{ID: 300, Name: "Studio Mic", IsInput: true}
	echoRow := virtualOutputRow(22, genericPlaybackName, echo)
	asdRow := virtualOutputRow(11, genericPlaybackName, asd)
	m := SetupModel{inputDevices: []deviceRow{mic}}
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{{
		ID: asdRow.ID, Name: genericPlaybackName, Virtual: true,
		VirtualSink: &echo, MixInputID: uint32Ptr(mic.ID), MixInputName: mic.Name,
	}}}

	m.restoreMixInputsFromPreset(preset, []deviceRow{asdRow, echoRow})

	require.Contains(t, m.mixInputs, echoRow.selectKey())
	assert.Contains(t, m.mixInputs[echoRow.selectKey()], mic.selectKey())
	assert.NotContains(t, m.mixInputs, asdRow.selectKey())
}

func TestRestoreVolumeAGCMatchesVirtualOutputsByIdentityBeforeNameFallback(t *testing.T) {
	echo := customVirtualSinkPreset("sink_echo", "EchoWarp")
	asd := customVirtualSinkPreset("sink_asd", "Asd")
	echoRow := virtualOutputRow(22, genericPlaybackName, echo)
	asdRow := virtualOutputRow(11, genericPlaybackName, asd)
	echoRow.Volume, asdRow.Volume, asdRow.AGC = 1.0, 1.0, true
	m := SetupModel{outputDevices: []deviceRow{asdRow, echoRow}}
	preset := recent.DevicePreset{Devices: []recent.PresetDevice{
		{ID: asdRow.ID, Name: genericPlaybackName, Virtual: true, VirtualSink: &echo, Volume: 0.25, AGC: true},
		{ID: echoRow.ID, Name: genericPlaybackName, Virtual: true, VirtualSink: &asd, Volume: 0.75},
	}}

	m.restoreVolumeAGCFromPreset(preset, []deviceRow{asdRow, echoRow})

	assert.Equal(t, 0.75, m.outputDevices[0].Volume)
	assert.False(t, m.outputDevices[0].AGC)
	assert.Equal(t, 0.25, m.outputDevices[1].Volume)
	assert.True(t, m.outputDevices[1].AGC)
}

func backendIdentityModel(
	t *testing.T,
	presets []recent.VirtualSinkPreset,
	devices []backendDeviceItem,
) SetupModel {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	items := make([]list.Item, len(devices))
	for i, d := range devices {
		items[i] = d
	}
	m := NewSetupModel(newTestConfig(config.ModeServer), list.New(items, list.NewDefaultDelegate(), 80, 20), true, 120, 40)
	for _, vs := range presets {
		m.trackVirtualSink("module-"+vs.SinkName, vs, true)
	}
	return m.WithUnifiedDeviceList(true)
}

func outputRowsByBackendID(rows []deviceRow) map[string]deviceRow {
	byID := make(map[string]deviceRow, len(rows))
	for _, row := range rows {
		byID[row.BackendID] = row
	}
	return byID
}

func inputRowsByBackendID(rows []deviceRow) map[string]deviceRow {
	byID := make(map[string]deviceRow, len(rows))
	for _, row := range rows {
		byID[row.BackendID] = row
	}
	return byID
}

func assertVirtualRow(t *testing.T, row deviceRow, name, sinkName string) {
	t.Helper()
	assert.Equal(t, name, row.Name)
	assert.True(t, row.IsVirtual)
	assert.Equal(t, sinkName, row.VirtualSinkName)
	assert.Contains(t, formatDeviceInfo(row), "adaptive")
}

func assertGenericRows(t *testing.T, rows []deviceRow, names []string) {
	t.Helper()
	require.Len(t, rows, len(names))
	for i, name := range names {
		assert.Equal(t, name, rows[i].Name)
		assert.False(t, rows[i].IsVirtual)
	}
}

func virtualOutputRow(id uint32, name string, vs recent.VirtualSinkPreset) deviceRow {
	return deviceRow{
		ID: id, Name: name, BackendID: vs.SinkName,
		VirtualSinkName: vs.SinkName, IsVirtual: true,
	}
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}
