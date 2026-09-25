package startup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

func testClientModes() map[string]interface{} {
	return map[string]interface{}{
		string(config.AudioModeDuplex): map[string]interface{}{
			"audio": map[string]interface{}{"devices": []config.DeviceEntry{
				{ID: 99, Name: "Second microphone", Role: config.RoleCapture, Volume: 0.25},
				{ID: 99, Name: "Missing speaker", Role: config.RolePlayback},
			}},
			"aec": true,
		},
		string(config.AudioModeNormal): map[string]interface{}{
			"audio": map[string]interface{}{"devices": []config.DeviceEntry{{ID: 99, Name: "Unrelated stale device"}}},
		},
	}
}

func TestPrepareClientModeFillsOnlyMissingConfiguredRoles(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.OutputDeviceID, cfg.ClientModes = testID(1), testClientModes()
	got, err := Prepare(cfg, testInventory(), testSaved(config.AudioModeDuplex), testProbe(config.AudioModeDuplex))
	require.NoError(t, err)
	assert.Equal(t, uint32(1), *got.OutputDeviceID)
	assert.Equal(t, uint32(2), *got.InputDeviceID)
	assert.Equal(t, 0.25, got.Devices[1].Volume)
	assert.True(t, got.AEC)
	assert.False(t, cfg.AEC)
	assert.Nil(t, cfg.InputDeviceID)
	assert.Empty(t, cfg.Devices)
	assert.Equal(t, testClientModes(), cfg.ClientModes)
}

func TestPrepareClientModePreservesExplicitZeroVolume(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeDuplex)
	cfg.Devices = []config.DeviceEntry{{ID: 1, Role: config.RolePlayback, Volume: 0}}
	cfg.ClientModes = testClientModes()
	got, err := Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeDuplex))
	require.NoError(t, err)
	assert.Zero(t, got.Devices[0].Volume)
	assert.Equal(t, uint32(1), got.Devices[0].ID)
}

func TestPrepareClientModeLegacyFields(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeDuplex)
	cfg.OutputDeviceID = testID(1)
	cfg.ClientModes = map[string]interface{}{cfg.AudioMode(): map[string]interface{}{
		"audio": map[string]interface{}{"input_device_id": 0, "output_device_id": 99},
	}}
	got, err := Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeDuplex))
	require.NoError(t, err)
	assert.Equal(t, uint32(0), *got.InputDeviceID)
	assert.Equal(t, uint32(1), *got.OutputDeviceID)
}

func TestPrepareInvalidModeDataRefusesStartup(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.ClientModes = map[string]interface{}{cfg.AudioMode(): map[string]interface{}{"audio": "invalid"}}
	_, err := Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "client mode settings")
}
