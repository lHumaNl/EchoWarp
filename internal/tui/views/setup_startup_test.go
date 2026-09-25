package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestAutomaticSetupConstructionDoesNotRestoreVirtualDevices(t *testing.T) {
	stub := stubVirtualAudioFuncs(t)
	vs := customVirtualSinkPreset("saved_sink", "Saved")
	require.NoError(t, preset.Save(preset.ServerPresets{LastMode: "duplex", Presets: map[string]preset.ModePreset{
		"duplex": {Devices: []recent.PresetDevice{{Name: vs.MonitorName, IsInput: true, Virtual: true, VirtualSink: &vs}}, VirtualSinks: []recent.VirtualSinkPreset{vs}},
	}}))
	cfg := newTestConfig(config.ModeServer)
	cfg.Devices = []config.DeviceEntry{{ID: 2, Name: "Physical mic", Role: config.RoleCapture, Volume: 1}}
	dl := list.New([]list.Item{mockDeviceItem{id: 2, name: "Physical mic", isInput: true}}, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, dl, true, 120, 40, true).WithUnifiedDeviceList(false)
	require.Empty(t, stub.createdNames)
	require.Empty(t, stub.removedIDs)
	require.Nil(t, m.pendingRestoreCmd)
	require.False(t, m.isDuplexMode, "saved mode must not replace explicit automatic configuration")
	require.NotNil(t, m.serverPresets, "manual restoration remains available")
	// Manual setup preserves the established restoration path.
	NewSetupModel(cfg, dl, true, 120, 40).WithUnifiedDeviceList(false)
	require.NotEmpty(t, stub.createdNames)
}
