package audio

import (
	"testing"

	"github.com/gen2brain/malgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceManager_CreateAndClose(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)
	require.NotNil(t, dm)

	err = dm.Close()
	assert.NoError(t, err)
}

func TestDeviceManager_ListInputDevices_ReturnsDevices(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)
	defer dm.Close()

	devices, err := dm.ListInputDevices()
	require.NoError(t, err)

	t.Logf("Found %d input devices", len(devices))
	for _, d := range devices {
		t.Logf("  [%d] %s (channels=%d, sampleRate=%d)", d.ID, d.Name, d.Channels, d.SampleRate)
		assert.True(t, d.IsInput)
	}
}

func TestDeviceManager_ListOutputDevices_ReturnsDevices(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)
	defer dm.Close()

	devices, err := dm.ListOutputDevices()
	require.NoError(t, err)

	t.Logf("Found %d output devices", len(devices))
	for _, d := range devices {
		t.Logf("  [%d] %s (channels=%d, sampleRate=%d)", d.ID, d.Name, d.Channels, d.SampleRate)
		assert.False(t, d.IsInput)
	}
}

func TestDeviceManager_Close_ReleasesResources(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)

	err = dm.Close()
	assert.NoError(t, err)

	err = dm.Close()
	assert.NoError(t, err)
}

func TestDeviceManager_SingleContextReuse(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)
	defer dm.Close()

	_, err = dm.ListInputDevices()
	require.NoError(t, err)
	_, err = dm.ListOutputDevices()
	require.NoError(t, err)
	_, err = dm.ListInputDevices()
	require.NoError(t, err)
}

func TestCapturer_SampleRate_MatchesConfigured(t *testing.T) {
	c, err := NewCapturer(48000, 1)
	require.NoError(t, err)
	assert.Equal(t, uint32(48000), c.SampleRate())
}

func TestCapturer_Channels_MatchesConfigured(t *testing.T) {
	c, err := NewCapturer(48000, 2)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), c.Channels())
}

func TestPlayer_Close_NoError(t *testing.T) {
	p, err := NewPlayer(48000, 1)
	require.NoError(t, err)
	assert.NoError(t, p.Close())
}

func TestDeviceManager_Context(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)
	defer dm.Close()

	ctx := dm.Context()
	assert.NotNil(t, ctx)
}

func TestDeviceManager_Context_AfterClose(t *testing.T) {
	dm, err := NewDeviceManager()
	require.NoError(t, err)

	err = dm.Close()
	require.NoError(t, err)

	ctx := dm.Context()
	assert.Nil(t, ctx)
}

func TestMalgoDeviceBackendID_UsesPrintableNativeID(t *testing.T) {
	var id malgo.DeviceID
	copy(id[:], "sink_echo.monitor")

	assert.Equal(t, "sink_echo.monitor", malgoDeviceBackendID(id))
}

func TestMalgoDeviceBackendID_FallsBackToHexForBinaryID(t *testing.T) {
	var id malgo.DeviceID
	id[0] = 0x01
	id[1] = 0xff

	assert.Equal(t, "01ff", malgoDeviceBackendID(id))
}
