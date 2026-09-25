package startup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestPrepareNamesOverrideStaleIDs(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 2, Name: "Microphone", Volume: 0, AGC: true}}
	got, err := Prepare(cfg, testInventory(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), got.Devices[0].ID)
	assert.Zero(t, got.Devices[0].Volume)
	assert.True(t, got.Devices[0].AGC)
	assert.Equal(t, uint32(2), cfg.Devices[0].ID)
}

func TestPrepareNamedAmbiguityCannotUseID(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 0, Name: "Microphone"}}
	inventory := append(testInventory(), audio.AudioDevice{ID: 4, Name: "Microphone", IsInput: true})
	_, err := Prepare(cfg, inventory, nil, nil)
	require.ErrorContains(t, err, "ambiguous")
	cfg.Devices[0].Name = ""
	_, err = Prepare(cfg, inventory, nil, nil)
	require.NoError(t, err)
}

func TestPrepareDevicesResolveTypeAndMultipleSelections(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeDuplex)
	cfg.Devices = []config.DeviceEntry{{ID: 0, Type: config.DeviceInput}, {ID: 2, Role: config.RoleCapture}, {ID: 1}}
	got, err := Prepare(cfg, testInventory(), nil, nil)
	require.NoError(t, err)
	require.Len(t, got.Devices, 3)
	assert.Equal(t, config.RolePlayback, got.Devices[2].Role)
	assert.Equal(t, uint32(1), *got.DeviceID)
	cfg.Devices = append(cfg.Devices, cfg.Devices[0])
	_, err = Prepare(cfg, testInventory(), nil, nil)
	require.ErrorContains(t, err, "duplicate")
}

func TestPrepareRejectsConflictingDirection(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 0, Role: config.RolePlayback, Type: config.DeviceInput}}
	_, err := Prepare(cfg, testInventory(), nil, nil)
	require.ErrorContains(t, err, "conflicting")
	cfg.Devices = []config.DeviceEntry{{ID: 1}}
	_, err = Prepare(cfg, testInventory(), nil, nil)
	require.ErrorContains(t, err, "capture device")
}

func TestPrepareOverlappingIDsRequireDirection(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeDuplex)
	cfg.DeviceID = testID(0)
	inventory := []audio.AudioDevice{{ID: 0, Name: "Mic", IsInput: true}, {ID: 0, Name: "Speaker"}}
	_, err := Prepare(cfg, inventory, nil, testProbe(config.AudioModeDuplex))
	require.ErrorContains(t, err, "ambiguous direction")
	cfg.DeviceID, cfg.InputDeviceID, cfg.OutputDeviceID = nil, testID(0), testID(0)
	got, err := Prepare(cfg, inventory, nil, testProbe(config.AudioModeDuplex))
	require.NoError(t, err)
	assert.Len(t, got.Devices, 2)
}

func TestPrepareLegacyDeviceDoesNotChangeDirectionToFillMissingRole(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeDuplex)
	cfg.DeviceID, cfg.OutputDeviceID = testID(0), testID(0)
	inventory := []audio.AudioDevice{{ID: 0, Name: "Mic", IsInput: true}, {ID: 0, Name: "Speaker"}}
	_, err := Prepare(cfg, inventory, nil, testProbe(config.AudioModeDuplex))
	require.ErrorContains(t, err, "ambiguous direction")
}

func TestPrepareOwnsDevicePointersAndSlices(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 1, MixInputID: testID(0)}}
	got, err := Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeNormal))
	require.NoError(t, err)
	*got.Devices[0].MixInputID = 2
	got.Devices[0].Name = "Changed"
	got.STUNServers[0] = "Changed"
	assert.Equal(t, uint32(0), *cfg.Devices[0].MixInputID)
	assert.Empty(t, cfg.Devices[0].Name)
	assert.NotEqual(t, "Changed", cfg.STUNServers[0])
}

func TestPrepareErrorDoesNotMutateInput(t *testing.T) {
	cfg := testConfig(config.ModeServer, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{Name: "Microphone", ID: 2}, {ID: 999}}
	_, err := Prepare(cfg, testInventory(), nil, nil)
	require.Error(t, err)
	assert.Equal(t, uint32(2), cfg.Devices[0].ID)
	assert.Empty(t, cfg.Devices[0].Role)
}

func TestPrepareConfiguredMixNameOverridesID(t *testing.T) {
	cfg := testConfig(config.ModeClient, config.AudioModeNormal)
	cfg.Devices = []config.DeviceEntry{{ID: 1, MixInputID: testID(99), MixInputName: "Microphone"}}
	got, err := Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeNormal))
	require.NoError(t, err)
	assert.Equal(t, uint32(0), *got.Devices[0].MixInputID)
	cfg.Devices[0].MixInputName = "Missing"
	_, err = Prepare(cfg, testInventory(), nil, testProbe(config.AudioModeNormal))
	require.ErrorContains(t, err, "mix input")
}
