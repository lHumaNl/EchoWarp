package startup

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestLatestRecentSelectsTimestampNotPosition(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	now := time.Now()
	servers := []recent.Server{
		{Address: "older", Port: 4415, LastConnected: now.Add(-time.Hour)},
		{Address: "latest", Port: 4415, LastConnected: now},
		{Address: "invalid-port", Port: maxPort + 1, LastConnected: now.Add(time.Hour)},
		{Address: "zero-time", Port: 4415},
	}
	require.NoError(t, recent.Save(servers))
	got, err := LatestRecent()
	require.NoError(t, err)
	assert.Equal(t, "latest", got.Address)
}

func TestLatestRecentMissingCorruptAndZeroTime(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	_, err := LatestRecent()
	require.ErrorContains(t, err, "no valid recent")
	path, err := recent.FilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("{{bad yaml"), 0600))
	_, err = LatestRecent()
	require.ErrorContains(t, err, "no valid recent")
	require.NoError(t, recent.Save([]recent.Server{{Address: "localhost", Port: 4415}}))
	_, err = LatestRecent()
	require.ErrorContains(t, err, "no valid recent")
}

func TestPresetFromConfigIsOwnedAndPreservesFields(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 1, Name: "Speaker", Role: config.RolePlayback,
		Volume: 0, AGC: true, MixInputID: testID(0), MixInputName: "Microphone"}}
	preset := PresetFromConfig(cfg)
	require.Len(t, preset.Devices, 1)
	assert.Equal(t, recent.PresetDevice{ID: 1, Name: "Speaker", IsInput: false,
		Volume: 0, VolumeSet: true, AGC: true, MixInputID: testID(0), MixInputName: "Microphone"}, preset.Devices[0])
	*preset.Devices[0].MixInputID = 2
	preset.Devices[0].Name = "Changed"
	assert.Equal(t, uint32(0), *cfg.Devices[0].MixInputID)
	assert.Equal(t, "Speaker", cfg.Devices[0].Name)
}

func TestSuccessfulHistoryPreservesExplicitZeroVolume(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.Address = "host"
	cfg.Devices = []config.DeviceEntry{{ID: 1, Name: "Speaker", Role: config.RolePlayback, Volume: 0}}
	require.NoError(t, SaveSuccessful(cfg, nil))
	saved, err := LatestRecent()
	require.NoError(t, err)
	require.Zero(t, saved.Presets["normal"].Devices[0].Volume)
	require.True(t, saved.Presets["normal"].Devices[0].VolumeSet)
}

func TestPresetFromConfigLegacyRoles(t *testing.T) {
	for _, tc := range roleCases {
		t.Run(string(tc.role)+"/"+string(tc.mode), func(t *testing.T) {
			cfg := testConfig(tc.role, tc.mode)
			cfg.InputDeviceID, cfg.OutputDeviceID = testID(0), testID(1)
			preset := PresetFromConfig(cfg)
			require.Len(t, preset.Devices, tc.count)
			assert.Equal(t, preset.Devices[0].ID == 0, preset.Devices[0].IsInput)
			assert.Empty(t, preset.Devices[0].Name)
		})
	}
}

func persistedServer(t *testing.T) recent.Server {
	t.Helper()
	servers, err := recent.Load()
	require.NoError(t, err)
	require.Len(t, servers, 1)
	return servers[0]
}

func historyWithVirtualPreset() recent.Server {
	saved := testSaved(config.AudioModeNormal)
	saved.Address, saved.Port = "localhost", config.DefaultConfig().Port
	preset := saved.Presets[string(config.AudioModeNormal)]
	preset.VirtualSinks = []recent.VirtualSinkPreset{{SinkName: "Keep metadata", OnStart: recent.SinkKeep}}
	preset.Devices[1].Virtual = true
	preset.Devices[1].VirtualSink = &recent.VirtualSinkPreset{SinkName: "Per-device metadata", OnStart: recent.SinkKeep}
	saved.Presets[string(config.AudioModeNormal)] = preset
	saved.Presets[string(config.AudioModeReverse)] = recent.DevicePreset{Devices: []recent.PresetDevice{{Name: "Other microphone", IsInput: true, Volume: 1}}}
	return *saved
}

func TestSaveSuccessfulMergesModesAndVirtualMetadata(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	saved := historyWithVirtualPreset()
	require.NoError(t, recent.Save([]recent.Server{saved}))
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 1, Name: "Speaker", Role: config.RolePlayback, Volume: 0.5}}
	require.NoError(t, SaveSuccessful(cfg, testProbe(config.AudioModeNormal)))
	got := persistedServer(t)
	assert.Equal(t, string(config.AudioModeNormal), got.LastMode)
	assert.False(t, got.LastConnected.IsZero())
	assert.Equal(t, saved.Presets[string(config.AudioModeReverse)], got.Presets[string(config.AudioModeReverse)])
	preset := got.Presets[cfg.AudioMode()]
	assert.Equal(t, saved.Presets[cfg.AudioMode()].VirtualSinks, preset.VirtualSinks)
	require.Len(t, preset.Devices, 1)
	assert.True(t, preset.Devices[0].Virtual)
	assert.Equal(t, saved.Presets[cfg.AudioMode()].Devices[1].VirtualSink, preset.Devices[0].VirtualSink)
}

func TestSaveSuccessfulNoSecretsAndInvalidEndpointSkipped(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.Password, cfg.PasswordHash = t.Name(), t.Name()+"Hash"
	cfg.TURNServers = []config.TURNServer{{Credential: t.Name() + "TURN"}}
	cfg.Address = ""
	require.NoError(t, SaveSuccessful(cfg, nil))
	servers, err := recent.Load()
	require.NoError(t, err)
	assert.Empty(t, servers)
	cfg.Address = "localhost"
	require.NoError(t, SaveSuccessful(cfg, nil))
	path, err := recent.FilePath()
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), t.Name())
}

func TestSaveSuccessfulSerializesWriters(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	var writers sync.WaitGroup
	for _, mode := range []config.AudioMode{config.AudioModeNormal, config.AudioModeReverse, config.AudioModeDuplex, config.AudioModeConference} {
		writers.Go(func() {
			cfg := testConfig(config.ModeClient, mode)
			if err := SaveSuccessful(cfg, testProbe(mode)); err != nil {
				t.Error(err)
			}
		})
	}
	writers.Wait()
	assert.Len(t, persistedServer(t).Presets, 4)
}

func TestSaveSuccessfulPropagatesWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/not-a-directory"
	require.NoError(t, os.WriteFile(path, nil, 0600))
	t.Setenv("ECHOWARP_CONFIG_DIR", path)
	err := SaveSuccessful(testConfig(config.ModeClient, config.AudioModeNormal), nil)
	require.ErrorContains(t, err, "save successful connection")
}
