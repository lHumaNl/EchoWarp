package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// withTempConfigHome redirects OS config home (used by recent/preset packages)
// to a temp dir for the duration of the test.
func withTempConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// recent and preset use os.UserConfigDir() which reads XDG_CONFIG_HOME on
	// Linux and HOME on macOS. Set both for portability.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	// On darwin, os.UserConfigDir returns "$HOME/Library/Application Support"
	_ = os.MkdirAll(filepath.Join(dir, "Library", "Application Support"), 0755)
	return dir
}

func TestPersistClientRuntime_UpdatesVolumeAndAGC(t *testing.T) {
	withTempConfigHome(t)

	// Seed with an existing recent server entry carrying an initial preset.
	initial := []recent.Server{{
		Address:       "10.0.0.1",
		Port:          4415,
		Hostname:      "testhost",
		LastConnected: time.Now(),
		Presets: map[string]recent.DevicePreset{
			"normal": {
				Devices: []recent.PresetDevice{
					{ID: 1, Name: "Mic", Volume: 1.0, AGC: false},
					{ID: 2, Name: "Speakers", Volume: 1.0, AGC: false},
				},
			},
		},
	}}
	require.NoError(t, recent.Save(initial))

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "10.0.0.1",
		Port:       4415,
		StreamMode: config.AudioModeNormal,
	}

	persistClientRuntime(cfg, map[uint32]runtimeDevState{
		1: {volume: 0.7, agc: true},
		2: {volume: 1.3, agc: false},
	})

	loaded, err := recent.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	mp := loaded[0].Presets["normal"]
	require.Len(t, mp.Devices, 2)
	assert.InDelta(t, 0.7, mp.Devices[0].Volume, 0.001)
	assert.True(t, mp.Devices[0].AGC)
	assert.InDelta(t, 1.3, mp.Devices[1].Volume, 0.001)
	assert.False(t, mp.Devices[1].AGC)
}

func TestPersistServerRuntime_UpdatesVolumeAndAGC(t *testing.T) {
	withTempConfigHome(t)

	// Seed with a server preset.
	sp := preset.ServerPresets{
		Presets: map[string]preset.ModePreset{
			"normal": {
				Devices: []recent.PresetDevice{
					{ID: 5, Name: "Mic", Volume: 1.0, AGC: false},
				},
				Port:       4415,
				MaxClients: 1,
			},
		},
	}
	require.NoError(t, preset.Save(sp))

	cfg := config.Config{
		Mode:       config.ModeServer,
		Port:       4415,
		StreamMode: config.AudioModeNormal,
	}

	persistServerRuntime(cfg, map[uint32]runtimeDevState{
		5: {volume: 0.5, agc: true},
	})

	loaded := preset.Load()
	mp := loaded.Get("normal")
	require.NotNil(t, mp)
	require.Len(t, mp.Devices, 1)
	assert.InDelta(t, 0.5, mp.Devices[0].Volume, 0.001)
	assert.True(t, mp.Devices[0].AGC)
	// Device Name preserved:
	assert.Equal(t, "Mic", mp.Devices[0].Name)
}

func TestPersistRuntime_EmptyStatesIsNoop(t *testing.T) {
	cmd := persistRuntimeDeviceStateCmd(config.Config{}, nil)
	assert.Nil(t, cmd)
}

func TestPersistRuntime_IgnoresUnknownDeviceIDs(t *testing.T) {
	withTempConfigHome(t)
	initial := []recent.Server{{
		Address: "1.1.1.1", Port: 4415, Hostname: "h",
		Presets: map[string]recent.DevicePreset{
			"normal": {Devices: []recent.PresetDevice{
				{ID: 1, Name: "Mic", Volume: 1.0},
			}},
		},
	}}
	require.NoError(t, recent.Save(initial))

	cfg := config.Config{Mode: config.ModeClient, Address: "1.1.1.1", Port: 4415, StreamMode: config.AudioModeNormal}
	// Send update for device ID 999 which doesn't exist in preset.
	persistClientRuntime(cfg, map[uint32]runtimeDevState{999: {volume: 0.5}})

	loaded, _ := recent.Load()
	// Original device 1 should be untouched.
	assert.InDelta(t, 1.0, loaded[0].Presets["normal"].Devices[0].Volume, 0.001)
}
