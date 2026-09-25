package startup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func testConfig(role config.Mode, mode config.AudioMode) config.Config {
	cfg := config.DefaultConfig()
	cfg.Mode, cfg.StreamMode, cfg.Address = role, mode, "localhost"
	cfg.SyncFromStreamMode()
	return cfg
}

func testInventory() []audio.AudioDevice {
	return []audio.AudioDevice{
		{ID: 0, Name: "Microphone", IsInput: true},
		{ID: 1, Name: "Speaker"},
		{ID: 2, Name: "Second microphone", IsInput: true},
		{ID: 3, Name: "Second speaker"},
	}
}

func testProbe(mode config.AudioMode) *probe.ProbeServerResult {
	return &probe.ProbeServerResult{Mode: string(mode), ServerID: "server-test"}
}

func testID(id uint32) *uint32 { return &id }

func testSaved(mode config.AudioMode) *recent.Server {
	return &recent.Server{ServerID: "server-test", LastMode: string(mode), Presets: map[string]recent.DevicePreset{
		string(mode): {Devices: []recent.PresetDevice{
			{ID: 90, Name: "Microphone", IsInput: true, Volume: 0.5, AGC: true},
			{ID: 91, Name: "Speaker", Volume: 0.75},
		}},
	}}
}

var roleCases = []struct {
	role    config.Mode
	mode    config.AudioMode
	primary uint32
	count   int
}{
	{config.ModeServer, config.AudioModeNormal, 0, 1},
	{config.ModeServer, config.AudioModeReverse, 1, 1},
	{config.ModeServer, config.AudioModeDuplex, 1, 2},
	{config.ModeServer, config.AudioModeConference, 0, 1},
	{config.ModeClient, config.AudioModeNormal, 1, 1},
	{config.ModeClient, config.AudioModeReverse, 0, 1},
	{config.ModeClient, config.AudioModeDuplex, 1, 2},
	{config.ModeClient, config.AudioModeConference, 1, 2},
}

func TestPrepareRequiredRoles(t *testing.T) {
	for _, tc := range roleCases {
		t.Run(string(tc.role)+"/"+string(tc.mode), func(t *testing.T) {
			cfg := testConfig(tc.role, tc.mode)
			cfg.InputDeviceID, cfg.OutputDeviceID = testID(0), testID(1)
			got, err := Prepare(cfg, testInventory(), nil, testProbe(tc.mode))
			require.NoError(t, err)
			require.Len(t, got.Devices, tc.count)
			require.NotNil(t, got.DeviceID)
			assert.Equal(t, tc.primary, *got.DeviceID)
			assert.NotEmpty(t, got.Devices[0].Name)
			assert.NotEmpty(t, got.Devices[0].Type)
			assert.NotEmpty(t, got.Devices[0].Role)
		})
	}
}

func TestPrepareMutedConferenceHub(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeConference)
	cfg.ServerMuted = true
	cfg.DeviceID = testID(999)
	got, err := Prepare(cfg, nil, testSaved(config.AudioModeConference), nil)
	require.NoError(t, err)
	assert.Empty(t, got.Devices)
	assert.Nil(t, got.DeviceID)
	assert.Greater(t, got.MaxClients, 1)
}

func TestPrepareLegacyZeroID(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeNormal)
	cfg.DeviceID = testID(0)
	got, err := Prepare(cfg, testInventory(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Microphone", got.Devices[0].Name)
	assert.Equal(t, uint32(0), *got.InputDeviceID)
	assert.Equal(t, defaultVolume, got.Devices[0].Volume)
}

func TestPrepareRequiresClientProbeAndAddress(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.OutputDeviceID = testID(1)
	_, err := Prepare(cfg, testInventory(), nil, nil)
	require.ErrorContains(t, err, "probe")
	cfg.Address = " "
	_, err = Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "address")
}

func TestPrepareSavedIdentityAndMode(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	saved := testSaved(config.AudioModeNormal)
	info := testProbe(config.AudioModeNormal)
	info.ServerID = "another-server"
	_, err := Prepare(cfg, testInventory(), saved, info)
	require.ErrorContains(t, err, "identity changed")
	info.ServerID, info.Mode = saved.ServerID, string(config.AudioModeReverse)
	_, err = Prepare(cfg, testInventory(), saved, info)
	require.ErrorContains(t, err, "mode changed")
	info.Mode = ""
	_, err = Prepare(cfg, testInventory(), saved, info)
	require.NoError(t, err)
}

func TestPrepareProbeParametersAndPassword(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	info := testProbe(config.AudioModeReverse)
	info.SampleRate, info.Channels, info.OpusBitrate = 24000, 1, 32000
	info.TLSRequired, info.TLSSelfSigned, info.PasswordRequired = true, true, true
	_, err := Prepare(cfg, testInventory(), nil, info)
	require.ErrorContains(t, err, "password")
	cfg.Password, cfg.InputDeviceID = t.Name(), testID(0)
	got, err := Prepare(cfg, testInventory(), nil, info)
	require.NoError(t, err)
	assert.Equal(t, info.SampleRate, got.SampleRate)
	assert.Equal(t, info.Channels, got.Channels)
	assert.Equal(t, info.OpusBitrate, got.OpusBitrate)
	assert.True(t, got.Reverse && got.TLS && got.TLSInsecure)
	assert.Empty(t, cfg.Devices)
}

func TestPrepareValidatesAfterProbe(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.OutputDeviceID, cfg.SampleRate = testID(1), 123
	info := testProbe(config.AudioModeNormal)
	info.SampleRate = config.DefaultConfig().SampleRate
	_, err := Prepare(cfg, testInventory(), nil, info)
	require.NoError(t, err)
	info.Channels = 3
	_, err = Prepare(cfg, testInventory(), nil, info)
	require.ErrorContains(t, err, "channels")
	info.Mode = "unknown"
	_, err = Prepare(cfg, testInventory(), nil, info)
	require.ErrorContains(t, err, "unsupported audio mode")
}

func TestPrepareMissingRoles(t *testing.T) {
	for _, tc := range roleCases {
		t.Run(string(tc.role)+"/"+string(tc.mode), func(t *testing.T) {
			_, err := Prepare(testConfig(tc.role, tc.mode), testInventory(), nil, testProbe(tc.mode))
			require.ErrorContains(t, err, "missing required")
		})
	}
}
