package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOverlayDeviceRolePriorityPreservesComplement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.yml")
	require.NoError(t, os.WriteFile(path, []byte("audio:\n  devices:\n    - id: 1\n      role: capture\n    - id: 7\n      role: playback\n"), 0600))
	t.Setenv("ECHOWARP_INPUT_DEVICE_ID", "2")
	cfg, _, err := LoadOver(DefaultConfig(), path)
	require.NoError(t, err)
	require.Len(t, cfg.Devices, 2)
	require.Equal(t, DeviceEntry{ID: 2, Role: RoleCapture, Volume: 1}, cfg.Devices[1])
	require.Equal(t, uint32(7), cfg.Devices[0].ID)
}

func TestOverlayExplicitDeviceArraySuppressesLowerIDs(t *testing.T) {
	id := uint32(3)
	base := DefaultConfig()
	base.InputDeviceID = &id
	base.DeviceID = &id
	t.Setenv("ECHOWARP_DEVICES", "[]")
	cfg, _, err := LoadOver(base, "")
	require.NoError(t, err)
	require.Empty(t, cfg.Devices)
	require.Nil(t, cfg.InputDeviceID)
	require.Nil(t, cfg.DeviceID)
}

func TestOverlayRefusesUnknownComplementRole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.yml")
	require.NoError(t, os.WriteFile(path, []byte("audio:\n  devices:\n    - id: 1\n      name: ambiguous\n"), 0600))
	t.Setenv("ECHOWARP_INPUT_DEVICE_ID", "2")
	_, _, err := LoadOver(DefaultConfig(), path)
	require.ErrorContains(t, err, "untyped device array")
}
