package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConferenceLegacyDevicesKeepCaptureAndPlaybackSeparate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Conference = true
	input, output := uint32(7), uint32(2)
	cfg.InputDeviceID, cfg.OutputDeviceID, cfg.DeviceID = &input, &output, &output
	capture, playback := cfg.CaptureDevices(), cfg.PlaybackDevices()
	require.Len(t, capture, 1)
	require.Len(t, playback, 1)
	require.Equal(t, input, capture[0].ID)
	require.Equal(t, output, playback[0].ID)
}
