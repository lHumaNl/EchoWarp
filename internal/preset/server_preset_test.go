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
		LastMode: "normal",
		Presets: map[string]ModePreset{
			"normal": {
				Devices: []recent.PresetDevice{
					{ID: 1, Name: "Microphone", Virtual: false},
				},
				Port:       4416,
				Password:   "secret",
				MaxClients: 3,
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
	assert.Equal(t, "normal", loaded.LastMode)

	require.Contains(t, loaded.Presets, "normal")
	require.Len(t, loaded.Presets["normal"].Devices, 1)
	assert.Equal(t, uint32(1), loaded.Presets["normal"].Devices[0].ID)
	assert.Equal(t, "Microphone", loaded.Presets["normal"].Devices[0].Name)
	assert.Equal(t, 4416, loaded.Presets["normal"].Port)
	assert.Equal(t, "secret", loaded.Presets["normal"].Password)
	assert.Equal(t, 3, loaded.Presets["normal"].MaxClients)

	require.Contains(t, loaded.Presets, "reverse")
	assert.Equal(t, uint32(42), loaded.Presets["reverse"].Devices[0].ID)
}

func TestSaveLoadTopLevelVirtualSinksRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	input := ServerPresets{Presets: map[string]ModePreset{"normal": {
		Devices:      []recent.PresetDevice{{ID: 1, Name: "Mic", IsInput: true}},
		VirtualSinks: []recent.VirtualSinkPreset{serverTestVirtualSinkPreset()},
	}}}
	require.NoError(t, Save(input))

	loaded := Load()
	normal := loaded.Presets["normal"]

	require.Len(t, normal.Devices, 1)
	require.Len(t, normal.VirtualSinks, 1)
	assert.Equal(t, "EchoWarp", normal.VirtualSinks[0].SinkName)
	assert.Equal(t, recent.SinkRecreate, normal.VirtualSinks[0].OnStart)
}

func TestLoadLifecycleOnlyModePreset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	yamlData := `presets:
    normal:
        virtual_sinks:
            - module_type: module-null-sink
              sink_name: EchoWarp
              on_stop: delete
              on_start: recreate
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.yaml"), []byte(yamlData), 0600))

	loaded := Load()
	normal := loaded.Presets["normal"]

	assert.Empty(t, normal.Devices)
	require.Len(t, normal.VirtualSinks, 1)
	assert.Equal(t, "EchoWarp", normal.VirtualSinks[0].SinkName)
}

func serverTestVirtualSinkPreset() recent.VirtualSinkPreset {
	return recent.VirtualSinkPreset{
		ModuleType: "module-null-sink",
		SinkName:   "EchoWarp",
		OnStop:     recent.SinkDelete,
		OnStart:    recent.SinkRecreate,
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	sp := ServerPresets{Presets: map[string]ModePreset{
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
		Presets: map[string]ModePreset{
			"normal": {Devices: []recent.PresetDevice{{ID: 2, Name: "FromYAML", Virtual: false}}},
		},
	}))

	sp := Load()
	require.Contains(t, sp.Presets, "normal")
	assert.Equal(t, "FromYAML", sp.Presets["normal"].Devices[0].Name, "YAML should be preferred over JSON")
}

// TestLoadLegacyFormatMigration verifies that a legacy YAML file with a global
// `settings:` block is migrated on Load into per-mode ModePreset fields.
func TestLoadLegacyFormatMigration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	legacyYAML := `presets:
    normal:
        devices:
            - id: 1
              name: Mic
              is_input: true
              virtual: false
    conference:
        devices:
            - id: 2
              name: Speaker
              is_input: false
              virtual: false
settings:
    last_mode: normal
    port: 4420
    password: hunter2
    max_clients: 4
    tls: true
    tls_cert: /etc/ssl/cert.pem
    tls_key: /etc/ssl/key.pem
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.yaml"), []byte(legacyYAML), 0600))

	sp := Load()
	assert.Equal(t, "normal", sp.LastMode, "top-level last_mode should be migrated from legacy settings.last_mode")

	require.Contains(t, sp.Presets, "normal")
	normal := sp.Presets["normal"]
	assert.Equal(t, 4420, normal.Port, "legacy port should be migrated into per-mode preset")
	assert.Equal(t, "hunter2", normal.Password)
	assert.Equal(t, 4, normal.MaxClients)
	assert.True(t, normal.TLS)
	assert.Equal(t, "/etc/ssl/cert.pem", normal.TLSCert)
	assert.Equal(t, "/etc/ssl/key.pem", normal.TLSKey)
	require.Len(t, normal.Devices, 1)
	assert.Equal(t, "Mic", normal.Devices[0].Name)

	require.Contains(t, sp.Presets, "conference")
	conf := sp.Presets["conference"]
	assert.Equal(t, 4420, conf.Port, "legacy port should be migrated into every mode")
	assert.Equal(t, 4, conf.MaxClients)
	assert.True(t, conf.TLS)
	require.Len(t, conf.Devices, 1)
	assert.Equal(t, "Speaker", conf.Devices[0].Name)

	// After re-save, the file should be in new format: no `settings:` block.
	require.NoError(t, Save(sp))
	data, err := os.ReadFile(filepath.Join(dir, "server_presets.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "settings:", "new format must not contain settings block")
}

// TestSaveOmitsDefaultFields verifies that fields matching DefaultsFor(mode) are
// NOT written to the YAML file (omitempty + pre-marshal zeroing).
func TestSaveOmitsDefaultFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	sp := ServerPresets{
		LastMode: "normal",
		Presets: map[string]ModePreset{
			"normal": {
				Devices:    []recent.PresetDevice{{ID: 1, Name: "Mic"}},
				Port:       4415, // default → should be omitted
				MaxClients: 1,    // default → should be omitted
			},
			"conference": {
				Devices:    []recent.PresetDevice{{ID: 2, Name: "Speaker"}},
				Port:       4415, // default → should be omitted
				MaxClients: 2,    // conference default → should be omitted
			},
		},
	}
	require.NoError(t, Save(sp))

	data, err := os.ReadFile(filepath.Join(dir, "server_presets.yaml"))
	require.NoError(t, err)
	yaml := string(data)

	assert.NotContains(t, yaml, "port:", "default port 4415 must be omitted from YAML")
	assert.NotContains(t, yaml, "max_clients:", "default max_clients must be omitted from YAML")
	assert.NotContains(t, yaml, "password:", "empty password must be omitted")
	assert.NotContains(t, yaml, "tls:", "default tls=false must be omitted")
	assert.NotContains(t, yaml, "tls_cert:", "empty tls_cert must be omitted")
	assert.NotContains(t, yaml, "tls_key:", "empty tls_key must be omitted")
	assert.Contains(t, yaml, "last_mode: normal")
	assert.Contains(t, yaml, "Mic")
	assert.Contains(t, yaml, "Speaker")
}

// TestSaveWritesNonDefaultFields verifies that non-default values ARE written.
func TestSaveWritesNonDefaultFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	sp := ServerPresets{
		Presets: map[string]ModePreset{
			"normal": {
				Devices:    []recent.PresetDevice{{ID: 1, Name: "Mic"}},
				Port:       4420, // non-default
				Password:   "secret",
				MaxClients: 5, // non-default
				TLS:        true,
				TLSCert:    "/etc/ssl/a.crt",
				TLSKey:     "/etc/ssl/a.key",
			},
		},
	}
	require.NoError(t, Save(sp))

	data, err := os.ReadFile(filepath.Join(dir, "server_presets.yaml"))
	require.NoError(t, err)
	yaml := string(data)

	assert.Contains(t, yaml, "port: 4420")
	assert.Contains(t, yaml, "password: secret")
	assert.Contains(t, yaml, "max_clients: 5")
	assert.Contains(t, yaml, "tls: true")
	assert.Contains(t, yaml, "tls_cert: /etc/ssl/a.crt")
	assert.Contains(t, yaml, "tls_key: /etc/ssl/a.key")
}

// TestDefaultsFor verifies the defaults per mode.
func TestDefaultsFor(t *testing.T) {
	normal := DefaultsFor("normal")
	assert.Equal(t, 4415, normal.Port)
	assert.Equal(t, 1, normal.MaxClients)

	conf := DefaultsFor("conference")
	assert.Equal(t, 4415, conf.Port)
	assert.Equal(t, 2, conf.MaxClients)

	dup := DefaultsFor("duplex")
	assert.Equal(t, 4415, dup.Port)
	assert.Equal(t, 1, dup.MaxClients)
}

// TestLoadNewFormatOverridesLegacy verifies that per-mode fields in the new format
// take precedence over legacy `settings:` fields when both are present.
func TestLoadNewFormatOverridesLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Hand-edited file with both a legacy settings block AND per-mode fields.
	mixedYAML := `last_mode: conference
presets:
    normal:
        devices:
            - id: 1
              name: Mic
              is_input: true
        port: 5000
        max_clients: 7
    conference:
        devices:
            - id: 2
              name: Speaker
              is_input: false
settings:
    port: 4420
    max_clients: 4
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_presets.yaml"), []byte(mixedYAML), 0600))

	sp := Load()
	assert.Equal(t, "conference", sp.LastMode, "top-level last_mode takes precedence")

	require.Contains(t, sp.Presets, "normal")
	n := sp.Presets["normal"]
	assert.Equal(t, 5000, n.Port, "new-format per-mode port should override legacy settings.port")
	assert.Equal(t, 7, n.MaxClients, "new-format per-mode max_clients should override legacy")

	require.Contains(t, sp.Presets, "conference")
	c := sp.Presets["conference"]
	assert.Equal(t, 4420, c.Port, "conference has no per-mode port → legacy value migrated")
	assert.Equal(t, 4, c.MaxClients, "conference has no per-mode max_clients → legacy value migrated")
}
