package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockDeviceEnumerator struct {
	inputs  []AudioDevice
	outputs []AudioDevice
}

func (m *mockDeviceEnumerator) ListInputDevices() ([]AudioDevice, error) {
	return m.inputs, nil
}

func (m *mockDeviceEnumerator) ListOutputDevices() ([]AudioDevice, error) {
	return m.outputs, nil
}

func TestDetectBlackHole_NotInstalled(t *testing.T) {
	dm := &mockDeviceEnumerator{
		inputs: []AudioDevice{
			{ID: 0, Name: "Built-in Microphone", IsInput: true, Channels: 1},
		},
	}
	result := DetectBlackHole(dm)
	assert.Empty(t, result)
}

func TestLoopbackDevice_Types(t *testing.T) {
	ld := LoopbackDevice{
		OutputDevice: AudioDevice{Name: "Speakers", Channels: 2},
		BlackHole:    AudioDevice{Name: "BlackHole 2ch", Channels: 2},
		Channels:     2,
	}
	assert.Equal(t, "Speakers", ld.OutputDevice.Name)
	assert.Equal(t, "BlackHole 2ch", ld.BlackHole.Name)
	assert.Equal(t, uint32(2), ld.Channels)
}
