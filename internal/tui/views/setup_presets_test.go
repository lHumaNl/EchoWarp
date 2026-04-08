package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// newSetupModelForPresets builds a minimal SetupModel with pre-populated
// inputDevices and outputDevices slices and an initialized multiSelect map.
func newSetupModelForPresets(inputs, outputs []deviceRow, selected map[string]DeviceRoleSet) SetupModel {
	cfg := config.Config{
		Mode:       config.ModeServer,
		SampleRate: 48000,
		Channels:   1,
	}
	dl := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, dl, true, 120, 40)
	m.inputDevices = inputs
	m.outputDevices = outputs
	m.multiSelect = selected
	return m
}

// selectKeyFor is a test helper to build a selectKey from device params.
func selectKeyFor(id uint32, name string, isInput bool) string {
	return deviceRow{ID: id, Name: name, IsInput: isInput}.selectKey()
}

func TestCollectPresetDevices_NoSelection(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{{ID: 1, Name: "Mic", IsVirtual: false, IsInput: true}},
		[]deviceRow{{ID: 2, Name: "Speaker", IsVirtual: false}},
		map[string]DeviceRoleSet{}, // nothing selected
	)

	p := m.CollectPresetDevices()

	assert.Empty(t, p.Devices)
}

func TestCollectPresetDevices_InputAndOutputSelected(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{{ID: 1, Name: "Mic", IsVirtual: false, IsInput: true}},
		[]deviceRow{{ID: 2, Name: "Speaker", IsVirtual: false}},
		map[string]DeviceRoleSet{
			selectKeyFor(1, "Mic", true):      {Capture: true},
			selectKeyFor(2, "Speaker", false): {Playback: true},
		},
	)

	p := m.CollectPresetDevices()

	assert.Len(t, p.Devices, 2)

	byName := make(map[string]recent.PresetDevice)
	for _, d := range p.Devices {
		byName[d.Name] = d
	}

	mic := byName["Mic"]
	assert.Equal(t, uint32(1), mic.ID)
	assert.False(t, mic.Virtual)

	spk := byName["Speaker"]
	assert.Equal(t, uint32(2), spk.ID)
	assert.False(t, spk.Virtual)
}

func TestCollectPresetDevices_VirtualDeviceMarkedCorrectly(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{{ID: 10, Name: "BlackHole 2ch", IsVirtual: true, IsInput: true}},
		[]deviceRow{},
		map[string]DeviceRoleSet{
			selectKeyFor(10, "BlackHole 2ch", true): {Capture: true},
		},
	)

	p := m.CollectPresetDevices()

	assert.Len(t, p.Devices, 1)
	assert.True(t, p.Devices[0].Virtual)
	assert.Equal(t, "BlackHole 2ch", p.Devices[0].Name)
	assert.Equal(t, uint32(10), p.Devices[0].ID)
}

func TestCollectPresetDevices_OnlySelectedDevicesIncluded(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{
			{ID: 1, Name: "Mic A", IsInput: true},
			{ID: 2, Name: "Mic B", IsInput: true},
		},
		[]deviceRow{
			{ID: 3, Name: "Speaker A"},
			{ID: 4, Name: "Speaker B"},
		},
		map[string]DeviceRoleSet{
			selectKeyFor(1, "Mic A", true):      {Capture: true},
			selectKeyFor(4, "Speaker B", false): {Playback: true},
		},
	)

	p := m.CollectPresetDevices()

	assert.Len(t, p.Devices, 2)
	names := make([]string, 0, 2)
	for _, d := range p.Devices {
		names = append(names, d.Name)
	}
	assert.Contains(t, names, "Mic A")
	assert.Contains(t, names, "Speaker B")
	assert.NotContains(t, names, "Mic B")
	assert.NotContains(t, names, "Speaker A")
}

func TestCollectPresetDevices_InputDevicesBeforeOutputDevices(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{{ID: 1, Name: "Input", IsInput: true}},
		[]deviceRow{{ID: 2, Name: "Output"}},
		map[string]DeviceRoleSet{
			selectKeyFor(1, "Input", true):   {Capture: true},
			selectKeyFor(2, "Output", false): {Playback: true},
		},
	)

	p := m.CollectPresetDevices()

	assert.Len(t, p.Devices, 2)
	assert.Equal(t, "Input", p.Devices[0].Name)
	assert.Equal(t, "Output", p.Devices[1].Name)
}

func TestCollectPresetDevices_SameNameDifferentType(t *testing.T) {
	// BlackHole 2ch exists as both input and output — selecting output should NOT select input
	m := newSetupModelForPresets(
		[]deviceRow{{ID: 10, Name: "BlackHole 2ch", IsInput: true, IsVirtual: true}},
		[]deviceRow{{ID: 11, Name: "BlackHole 2ch", IsVirtual: true}},
		map[string]DeviceRoleSet{
			selectKeyFor(11, "BlackHole 2ch", false): {Playback: true},
		},
	)

	p := m.CollectPresetDevices()

	assert.Len(t, p.Devices, 1)
	assert.Equal(t, "BlackHole 2ch", p.Devices[0].Name)
	assert.False(t, p.Devices[0].IsInput, "should be output device only")
}

func TestCollectPresetDevices_VirtualOutputHasVirtualSinkPreset(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{},
		[]deviceRow{{ID: 99, Name: "EchoWarp", IsVirtual: true}},
		map[string]DeviceRoleSet{
			selectKeyFor(99, "EchoWarp", false): {Playback: true},
		},
	)

	p := m.CollectPresetDevices()

	require.Len(t, p.Devices, 1)
	d := p.Devices[0]
	assert.True(t, d.Virtual)
	require.NotNil(t, d.VirtualSink, "virtual output should have VirtualSinkPreset")
	assert.Equal(t, "module-null-sink", d.VirtualSink.ModuleType)
	assert.Equal(t, "EchoWarp", d.VirtualSink.SinkName)
	assert.Equal(t, recent.SinkDelete, d.VirtualSink.OnStop)
	assert.Equal(t, recent.SinkRecreate, d.VirtualSink.OnStart)
}

func TestCollectPresetDevices_NonVirtualOutputHasNoVirtualSinkPreset(t *testing.T) {
	m := newSetupModelForPresets(
		[]deviceRow{},
		[]deviceRow{{ID: 1, Name: "Speaker", IsVirtual: false}},
		map[string]DeviceRoleSet{
			selectKeyFor(1, "Speaker", false): {Playback: true},
		},
	)

	p := m.CollectPresetDevices()

	require.Len(t, p.Devices, 1)
	assert.Nil(t, p.Devices[0].VirtualSink, "non-virtual output should not have VirtualSinkPreset")
}

func TestMatchPresetDevices_VirtualSinkMatchesByName(t *testing.T) {
	preset := recent.DevicePreset{
		Devices: []recent.PresetDevice{
			{
				ID: 42, Name: "EchoWarp", Virtual: true,
				VirtualSink: &recent.VirtualSinkPreset{
					ModuleType: "module-null-sink",
					SinkName:   "EchoWarp",
					OnStop:     recent.SinkDelete,
					OnStart:    recent.SinkRecreate,
				},
			},
		},
	}

	// Different ID but name contains "EchoWarp"
	outputs := []deviceRow{
		{ID: 999, Name: "EchoWarp", IsVirtual: true},
	}

	matched, unmatched := matchPresetDevices(preset, nil, outputs)
	assert.Len(t, matched, 1)
	assert.Equal(t, uint32(999), matched[0].ID)
	assert.Empty(t, unmatched)
}

func TestMatchPresetDevices_OldPresetWithoutVirtualSinkStillWorks(t *testing.T) {
	// Backward compat: old preset has Virtual=true but no VirtualSink field
	preset := recent.DevicePreset{
		Devices: []recent.PresetDevice{
			{ID: 5, Name: "Speaker", IsInput: false, Virtual: false},
		},
	}

	outputs := []deviceRow{
		{ID: 5, Name: "Speaker"},
	}

	matched, unmatched := matchPresetDevices(preset, nil, outputs)
	assert.Len(t, matched, 1)
	assert.Equal(t, uint32(5), matched[0].ID)
	assert.Empty(t, unmatched)
}
