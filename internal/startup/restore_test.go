package startup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestPrepareRestoresOnlyMissingRoles(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeDuplex)
	cfg.Devices = []config.DeviceEntry{{ID: 3, Role: config.RolePlayback, Volume: 0}}
	saved := testSaved(config.AudioModeDuplex)
	saved.Presets[cfg.AudioMode()].Devices[1].Name = "Missing speaker"
	got, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeDuplex))
	require.NoError(t, err)
	assert.Equal(t, uint32(3), *got.OutputDeviceID)
	assert.Zero(t, got.Devices[0].Volume)
	assert.Equal(t, uint32(0), *got.InputDeviceID)
	assert.True(t, got.Devices[1].AGC)
	assert.Equal(t, 0.5, got.Devices[1].Volume)
	assert.Equal(t, uint32(90), saved.Presets[cfg.AudioMode()].Devices[0].ID)
}

func TestPrepareSavedNamesMustBeUnique(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	saved := testSaved(config.AudioModeNormal)
	inventory := append(testInventory(), audio.AudioDevice{ID: 91, Name: "Speaker"})
	_, err := Prepare(cfg, inventory, saved, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "ambiguous")
	saved.Presets[cfg.AudioMode()].Devices[1].Name = ""
	_, err = Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "no name")
}

func TestPrepareSavedMixInputRequiresName(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	saved := testSaved(config.AudioModeNormal)
	saved.Presets[cfg.AudioMode()].Devices[1].MixInputID = testID(0)
	_, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "mix input has no name")
	saved.Presets[cfg.AudioMode()].Devices[1].MixInputName = "Second microphone"
	got, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
	require.NoError(t, err)
	assert.Equal(t, uint32(2), *got.Devices[0].MixInputID)
}

func TestPrepareSavedMixInputAmbiguity(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	saved := testSaved(config.AudioModeNormal)
	saved.Presets[cfg.AudioMode()].Devices[1].MixInputName = "Microphone"
	inventory := append(testInventory(), audio.AudioDevice{ID: 4, Name: "Microphone", IsInput: true})
	_, err := Prepare(cfg, inventory, saved, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "mix input")
	_, err = Prepare(cfg, testInventory()[1:], saved, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "mix input")
}

func TestPrepareRejectsDestructivePresetLifecycle(t *testing.T) {
	for _, action := range []recent.SinkLifecycle{recent.SinkDelete, recent.SinkRecreate} {
		t.Run(string(action), func(t *testing.T) {
			cfg := testConfig(config.ModeClient, config.AudioModeNormal)
			saved := testSaved(config.AudioModeNormal)
			preset := saved.Presets[cfg.AudioMode()]
			preset.VirtualSinks = []recent.VirtualSinkPreset{{OnStart: action}}
			saved.Presets[cfg.AudioMode()] = preset
			_, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
			require.ErrorContains(t, err, "unsafe startup action")
			preset.VirtualSinks = nil
			preset.Devices[1].VirtualSink = &recent.VirtualSinkPreset{OnStart: action}
			saved.Presets[cfg.AudioMode()] = preset
			_, err = Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
			require.ErrorContains(t, err, "unsafe startup action")
		})
	}
}

func TestPrepareIgnoresUnusedPresetLifecycleAndDevices(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.OutputDeviceID = testID(3)
	saved := testSaved(config.AudioModeNormal)
	saved.Presets[cfg.AudioMode()].Devices[1].Name = "Missing"
	saved.Presets[cfg.AudioMode()].Devices[1].VirtualSink = &recent.VirtualSinkPreset{OnStart: recent.SinkDelete}
	got, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
	require.NoError(t, err)
	assert.Equal(t, uint32(3), *got.DeviceID)
}

func TestPrepareMissingVirtualDeviceDoesNotRestoreOldID(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	saved := testSaved(config.AudioModeNormal)
	saved.Presets[cfg.AudioMode()].Devices[1] = recent.PresetDevice{
		ID: 1, Name: "Missing virtual speaker", Virtual: true,
		VirtualSink: &recent.VirtualSinkPreset{OnStart: recent.SinkKeep},
	}
	_, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "missing")
}

func TestPrepareDoesNotUseOtherModePreset(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeReverse)
	saved := testSaved(config.AudioModeNormal)
	saved.LastMode, saved.ServerID = "", ""
	_, err := Prepare(cfg, testInventory(), saved, testProbe(config.AudioModeReverse))
	require.ErrorContains(t, err, "missing required")
}
