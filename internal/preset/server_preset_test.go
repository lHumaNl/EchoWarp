package preset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	sp := Load()
	assert.NotNil(t, sp.Presets)
	assert.Empty(t, sp.Presets)
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.yaml"), []byte("{{bad"), 0600))

	sp := Load()
	assert.NotNil(t, sp.Presets)
	assert.Empty(t, sp.Presets)
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	input := ServerPresets{
		Presets: map[string]recent.DevicePreset{
			"normal": {
				Devices: []recent.PresetDevice{
					{ID: 1, Name: "Microphone", Virtual: false},
				},
			},
			"reverse": {
				Devices: []recent.PresetDevice{
					{ID: 42, Name: "Speakers", Virtual: false},
				},
			},
		},
	}
	require.NoError(t, Save(input))

	loaded := Load()
	require.Len(t, loaded.Presets, 2)
	require.Contains(t, loaded.Presets, "normal")
	require.Len(t, loaded.Presets["normal"].Devices, 1)
	assert.Equal(t, uint32(1), loaded.Presets["normal"].Devices[0].ID)
	assert.Equal(t, "Microphone", loaded.Presets["normal"].Devices[0].Name)
	assert.False(t, loaded.Presets["normal"].Devices[0].Virtual)

	require.Contains(t, loaded.Presets, "reverse")
	assert.Equal(t, uint32(42), loaded.Presets["reverse"].Devices[0].ID)
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	sp := ServerPresets{Presets: map[string]recent.DevicePreset{
		"duplex": {Devices: []recent.PresetDevice{{ID: 5, Name: "VirtMic", Virtual: true}}},
	}}
	require.NoError(t, Save(sp))

	data, err := os.ReadFile(filepath.Join(dir, "server_presets.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "VirtMic")
}

func TestLoadNilPresetsMap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	// Write valid YAML with null presets
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.yaml"), []byte("presets:\n"), 0600))

	sp := Load()
	assert.NotNil(t, sp.Presets, "nil presets in YAML should be initialized to empty map")
}

func TestLoadFallbackFromJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Write legacy JSON file
	legacyJSON := `{"presets":{"normal":{"devices":[{"id":1,"name":"Mic","virtual":false}]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.json"), []byte(legacyJSON), 0600))

	// No YAML file exists — should fall back to JSON
	sp := Load()
	require.Contains(t, sp.Presets, "normal")
	assert.Equal(t, "Mic", sp.Presets["normal"].Devices[0].Name)
}

func TestLoadPrefersYAMLOverJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Write legacy JSON with "FromJSON"
	legacyJSON := `{"presets":{"normal":{"devices":[{"id":1,"name":"FromJSON","virtual":false}]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.json"), []byte(legacyJSON), 0600))

	// Write YAML with "FromYAML"
	require.NoError(t, Save(ServerPresets{
		Presets: map[string]recent.DevicePreset{
			"normal": {Devices: []recent.PresetDevice{{ID: 2, Name: "FromYAML", Virtual: false}}},
		},
	}))

	sp := Load()
	require.Contains(t, sp.Presets, "normal")
	assert.Equal(t, "FromYAML", sp.Presets["normal"].Devices[0].Name, "YAML should be preferred over JSON")
}
