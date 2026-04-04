//go:build linux

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLoopbackEnumerator is a test double for DeviceEnumerator.
type mockLoopbackEnumerator struct {
	inputs  []AudioDevice
	outputs []AudioDevice
}

func (m *mockLoopbackEnumerator) ListInputDevices() ([]AudioDevice, error)  { return m.inputs, nil }
func (m *mockLoopbackEnumerator) ListOutputDevices() ([]AudioDevice, error) { return m.outputs, nil }

func TestListLoopbackDevices_Linux_FindsMonitorSources(t *testing.T) {
	dm := &mockLoopbackEnumerator{
		outputs: []AudioDevice{
			{ID: 1, Name: "alsa_output.pci-0000_00_1f.3.analog-stereo", IsInput: false, Channels: 2},
		},
		inputs: []AudioDevice{
			{ID: 2, Name: "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor", IsInput: true, Channels: 2},
		},
	}

	devices, err := ListLoopbackDevices(dm)
	require.NoError(t, err)
	require.Len(t, devices, 1)

	dev := devices[0]
	assert.Equal(t, "alsa_output.pci-0000_00_1f.3.analog-stereo", dev.OutputDevice.Name)
	assert.Equal(t, "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor", dev.BlackHole.Name)
	assert.Equal(t, uint32(2), dev.Channels)
}

func TestListLoopbackDevices_Linux_NoMonitor(t *testing.T) {
	dm := &mockLoopbackEnumerator{
		outputs: []AudioDevice{
			{ID: 1, Name: "alsa_output.pci-0000_00_1f.3.analog-stereo", IsInput: false, Channels: 2},
		},
		inputs: []AudioDevice{
			// No matching .monitor source.
			{ID: 3, Name: "alsa_input.pci-0000_00_1f.3.analog-stereo", IsInput: true, Channels: 2},
		},
	}

	devices, err := ListLoopbackDevices(dm)
	require.NoError(t, err)
	assert.Empty(t, devices)
}

func TestListLoopbackDevices_Linux_MultipleOutputs(t *testing.T) {
	dm := &mockLoopbackEnumerator{
		outputs: []AudioDevice{
			{ID: 1, Name: "sink1", IsInput: false, Channels: 2},
			{ID: 2, Name: "sink2", IsInput: false, Channels: 6},
		},
		inputs: []AudioDevice{
			{ID: 3, Name: "sink1.monitor", IsInput: true, Channels: 2},
			// sink2 has no monitor.
		},
	}

	devices, err := ListLoopbackDevices(dm)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, "sink1", devices[0].OutputDevice.Name)
}
