//go:build darwin

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListLoopbackDevices_NoBlackHole(t *testing.T) {
	dm := &mockDeviceEnumerator{
		inputs:  []AudioDevice{{ID: 0, Name: "Mic", IsInput: true, Channels: 1}},
		outputs: []AudioDevice{{ID: 0, Name: "Speakers", Channels: 2}},
	}
	result, err := ListLoopbackDevices(dm)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestDetectBlackHole_Installed2ch(t *testing.T) {
	dm := &mockDeviceEnumerator{
		inputs: []AudioDevice{
			{ID: 0, Name: "Built-in Microphone", IsInput: true, Channels: 1},
			{ID: 1, Name: "BlackHole 2ch", IsInput: true, Channels: 2},
		},
	}
	result := DetectBlackHole(dm)
	assert.Len(t, result, 1)
	assert.Equal(t, "BlackHole 2ch", result[0].Name)
}

func TestDetectBlackHole_MultipleVariants(t *testing.T) {
	dm := &mockDeviceEnumerator{
		inputs: []AudioDevice{
			{ID: 0, Name: "BlackHole 2ch", IsInput: true, Channels: 2},
			{ID: 1, Name: "BlackHole 16ch", IsInput: true, Channels: 16},
			{ID: 2, Name: "BlackHole 64ch", IsInput: true, Channels: 64},
		},
	}
	result := DetectBlackHole(dm)
	assert.Len(t, result, 3)
}

func TestListLoopbackDevices_WithBlackHole(t *testing.T) {
	dm := &mockDeviceEnumerator{
		inputs: []AudioDevice{
			{ID: 0, Name: "Built-in Microphone", IsInput: true, Channels: 1},
			{ID: 1, Name: "BlackHole 2ch", IsInput: true, Channels: 2},
		},
		outputs: []AudioDevice{
			{ID: 0, Name: "MacBook Pro Speakers", Channels: 2},
			{ID: 1, Name: "BlackHole 2ch", Channels: 2}, // should be excluded
			{ID: 2, Name: "External DAC", Channels: 2},
		},
	}
	result, err := ListLoopbackDevices(dm)
	assert.NoError(t, err)
	assert.Len(t, result, 2) // Speakers + External DAC, no BlackHole

	assert.Equal(t, "MacBook Pro Speakers", result[0].OutputDevice.Name)
	assert.Equal(t, "BlackHole 2ch", result[0].BlackHole.Name)
	assert.Equal(t, uint32(2), result[0].Channels)

	assert.Equal(t, "External DAC", result[1].OutputDevice.Name)
}

func TestListLoopbackDevices_ChannelMatching(t *testing.T) {
	dm := &mockDeviceEnumerator{
		inputs: []AudioDevice{
			{ID: 0, Name: "BlackHole 2ch", IsInput: true, Channels: 2},
			{ID: 1, Name: "BlackHole 16ch", IsInput: true, Channels: 16},
		},
		outputs: []AudioDevice{
			{ID: 0, Name: "Stereo Speakers", Channels: 2},
			{ID: 1, Name: "Surround System", Channels: 8},
		},
	}
	result, err := ListLoopbackDevices(dm)
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	// Stereo speakers → BlackHole 2ch (max fitting)
	assert.Equal(t, "BlackHole 2ch", result[0].BlackHole.Name)
	assert.Equal(t, uint32(2), result[0].Channels)

	// Surround 8ch → BlackHole 2ch (16ch > 8ch, so 2ch is max fitting)
	assert.Equal(t, "BlackHole 2ch", result[1].BlackHole.Name)
	assert.Equal(t, uint32(2), result[1].Channels)
}

func TestBestBlackHole_ExactMatch(t *testing.T) {
	blackholes := []AudioDevice{
		{Name: "BlackHole 2ch", Channels: 2},
		{Name: "BlackHole 16ch", Channels: 16},
	}
	bh := bestBlackHole(blackholes, 2)
	assert.NotNil(t, bh)
	assert.Equal(t, "BlackHole 2ch", bh.Name)
}

func TestBestBlackHole_LargerAvailable(t *testing.T) {
	blackholes := []AudioDevice{
		{Name: "BlackHole 2ch", Channels: 2},
		{Name: "BlackHole 16ch", Channels: 16},
	}
	bh := bestBlackHole(blackholes, 16)
	assert.NotNil(t, bh)
	assert.Equal(t, "BlackHole 16ch", bh.Name)
}

func TestBestBlackHole_FallbackToSmallest(t *testing.T) {
	blackholes := []AudioDevice{
		{Name: "BlackHole 16ch", Channels: 16},
		{Name: "BlackHole 64ch", Channels: 64},
	}
	bh := bestBlackHole(blackholes, 2)
	assert.NotNil(t, bh)
	assert.Equal(t, "BlackHole 16ch", bh.Name)
}

func TestBestBlackHole_Empty(t *testing.T) {
	bh := bestBlackHole(nil, 2)
	assert.Nil(t, bh)
}
