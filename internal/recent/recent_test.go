package recent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	servers, err := Load()
	assert.NoError(t, err)
	assert.Empty(t, servers)
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	p := filepath.Join(dir, "recent_servers.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
	require.NoError(t, os.WriteFile(p, []byte("{{bad yaml"), 0600))

	servers, err := Load()
	assert.NoError(t, err)
	assert.Empty(t, servers)
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	now := time.Now().Truncate(time.Second)
	input := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now},
		{Address: "10.0.0.2", Port: 4416, Hostname: "B", LastConnected: now.Add(-time.Hour)},
	}
	require.NoError(t, Save(input))

	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "10.0.0.1", loaded[0].Address)
	assert.Equal(t, 4415, loaded[0].Port)
	assert.Equal(t, "A", loaded[0].Hostname)
	assert.True(t, loaded[0].LastConnected.Equal(now))
	assert.Equal(t, "B", loaded[1].Hostname)
}

func TestAddNew(t *testing.T) {
	now := time.Now()
	servers := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now},
	}
	result := Add(servers, Server{Address: "10.0.0.2", Port: 4416, Hostname: "B", LastConnected: now})
	require.Len(t, result, 2)
	assert.Equal(t, "10.0.0.2", result[0].Address, "new entry should be at front")
	assert.Equal(t, "10.0.0.1", result[1].Address)
}

func TestAddExistingUpdatesAndMovesToFront(t *testing.T) {
	t1 := time.Now().Add(-time.Hour)
	t2 := time.Now()
	servers := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: t1},
		{Address: "10.0.0.2", Port: 4416, Hostname: "B", LastConnected: t1},
	}
	result := Add(servers, Server{Address: "10.0.0.2", Port: 4416, Hostname: "B-updated", LastConnected: t2})
	require.Len(t, result, 2)
	assert.Equal(t, "10.0.0.2", result[0].Address, "updated entry should be at front")
	assert.Equal(t, "B-updated", result[0].Hostname)
	assert.True(t, result[0].LastConnected.Equal(t2))
}

func TestAddEvictsOldest(t *testing.T) {
	now := time.Now()
	var servers []Server
	for i := 0; i < MaxEntries; i++ {
		servers = append(servers, Server{
			Address:       "10.0.0.1",
			Port:          4400 + i,
			Hostname:      "",
			LastConnected: now.Add(-time.Duration(i+1) * time.Minute),
		})
	}
	require.Len(t, servers, MaxEntries)

	result := Add(servers, Server{Address: "10.0.0.99", Port: 9999, Hostname: "New", LastConnected: now})
	require.Len(t, result, MaxEntries)
	assert.Equal(t, "10.0.0.99", result[0].Address, "new entry at front")
	// Oldest original (port 4409, -10min) was evicted; last entry is port 4408
	assert.Equal(t, 4408, result[MaxEntries-1].Port)
}

func TestFilePath(t *testing.T) {
	p, err := FilePath()
	require.NoError(t, err)
	assert.Contains(t, p, "echowarp")
	assert.Contains(t, p, "recent_servers.yaml")
}

func TestUnmarshalJSON_BackwardCompatNickname(t *testing.T) {
	// Legacy JSON with "nickname" field should be read into Hostname.
	raw := `[{"address":"10.0.0.1","port":4415,"nickname":"OldName","last_connected":"2026-01-01T00:00:00Z"}]`
	var servers []Server
	require.NoError(t, json.Unmarshal([]byte(raw), &servers))
	require.Len(t, servers, 1)
	assert.Equal(t, "OldName", servers[0].Hostname)
}

func TestUnmarshalJSON_HostnamePreferred(t *testing.T) {
	// When both hostname and nickname are present, hostname wins.
	raw := `[{"address":"10.0.0.1","port":4415,"hostname":"NewName","nickname":"OldName","last_connected":"2026-01-01T00:00:00Z"}]`
	var servers []Server
	require.NoError(t, json.Unmarshal([]byte(raw), &servers))
	require.Len(t, servers, 1)
	assert.Equal(t, "NewName", servers[0].Hostname)
}

func TestAddPreservesPresets(t *testing.T) {
	now := time.Now()
	existing := []Server{
		{
			Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now.Add(-time.Hour),
			Presets: map[string]DevicePreset{
				"normal": {Devices: []PresetDevice{{ID: 1, Name: "Mic", Virtual: false}}},
			},
		},
	}
	// Add same server without presets — presets should be preserved
	result := Add(existing, Server{Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now})
	require.Len(t, result, 1)
	require.Contains(t, result[0].Presets, "normal")
	assert.Equal(t, uint32(1), result[0].Presets["normal"].Devices[0].ID)
}

func TestAddOverwritesPresets(t *testing.T) {
	now := time.Now()
	existing := []Server{
		{
			Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now.Add(-time.Hour),
			Presets: map[string]DevicePreset{
				"normal": {Devices: []PresetDevice{{ID: 1, Name: "Mic", Virtual: false}}},
			},
		},
	}
	// Add same server WITH new presets — new presets should win
	result := Add(existing, Server{
		Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now,
		Presets: map[string]DevicePreset{
			"duplex": {Devices: []PresetDevice{{ID: 42, Name: "Speakers", Virtual: false}}},
		},
	})
	require.Len(t, result, 1)
	require.Contains(t, result[0].Presets, "duplex")
	assert.NotContains(t, result[0].Presets, "normal")
}

func TestAddMatchByServerID(t *testing.T) {
	now := time.Now()
	uid := "550e8400-e29b-41d4-a716-446655440000"

	existing := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", ServerID: uid, LastConnected: now.Add(-time.Hour)},
		{Address: "10.0.0.2", Port: 4416, Hostname: "B", LastConnected: now.Add(-time.Hour)},
	}
	// Same ServerID, but different addr:port (server moved to a new network)
	updated := Server{Address: "192.168.1.5", Port: 4415, Hostname: "A", ServerID: uid, LastConnected: now}
	result := Add(existing, updated)

	require.Len(t, result, 2, "should not add a duplicate")
	assert.Equal(t, "192.168.1.5", result[0].Address, "address should be updated to new network")
	assert.Equal(t, uid, result[0].ServerID)
}

func TestAddServerIDMatchPreservesPresets(t *testing.T) {
	now := time.Now()
	uid := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	presets := map[string]DevicePreset{
		"normal": {Devices: []PresetDevice{{ID: 7, Name: "Mic", Virtual: false}}},
	}

	existing := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", ServerID: uid, LastConnected: now.Add(-time.Hour), Presets: presets},
	}
	// Same ServerID, different IP, no presets in new entry
	updated := Server{Address: "172.16.0.1", Port: 4415, Hostname: "A", ServerID: uid, LastConnected: now}
	result := Add(existing, updated)

	require.Len(t, result, 1)
	require.Contains(t, result[0].Presets, "normal", "presets should be preserved after ServerID match")
	assert.Equal(t, uint32(7), result[0].Presets["normal"].Devices[0].ID)
}

func TestAddFallsBackToAddrPortWhenNoServerID(t *testing.T) {
	now := time.Now()
	existing := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", LastConnected: now.Add(-time.Hour)},
	}
	// No ServerID on either — should still deduplicate by addr:port
	result := Add(existing, Server{Address: "10.0.0.1", Port: 4415, Hostname: "A-updated", LastConnected: now})
	require.Len(t, result, 1)
	assert.Equal(t, "A-updated", result[0].Hostname)
}

func TestAddServerIDRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	uid := "550e8400-e29b-41d4-a716-446655440000"
	servers := []Server{
		{Address: "10.0.0.1", Port: 4415, Hostname: "A", ServerID: uid, LastConnected: time.Now()},
	}
	require.NoError(t, Save(servers))

	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, uid, loaded[0].ServerID)
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	require.NoError(t, Save([]Server{{Address: "1.2.3.4", Port: 1234}}))

	p := filepath.Join(dir, "recent_servers.yaml")
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(data), "1.2.3.4")
}

func TestLoadFallbackFromJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Write legacy JSON file
	legacyJSON := `[{"address":"10.0.0.1","port":4415,"hostname":"Legacy","last_connected":"2026-01-01T00:00:00Z"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "recent_servers.json"), []byte(legacyJSON), 0600))

	// No YAML file exists — should fall back to JSON
	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "Legacy", loaded[0].Hostname)
	assert.Equal(t, 4415, loaded[0].Port)
}

func TestLoadPrefersYAMLOverJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Write both files
	legacyJSON := `[{"address":"10.0.0.1","port":4415,"hostname":"FromJSON","last_connected":"2026-01-01T00:00:00Z"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "recent_servers.json"), []byte(legacyJSON), 0600))

	// Save via YAML
	require.NoError(t, Save([]Server{
		{Address: "10.0.0.2", Port: 4416, Hostname: "FromYAML", LastConnected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}))

	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "FromYAML", loaded[0].Hostname, "YAML should be preferred over JSON")
}

func TestVirtualSinkPresetYAMLRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	id := uint32(42)
	servers := []Server{
		{
			Address: "10.0.0.1", Port: 4415, Hostname: "A",
			LastConnected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Presets: map[string]DevicePreset{
				"normal": {Devices: []PresetDevice{
					{
						ID: 99, Name: "EchoWarp", Virtual: true,
						VirtualSink: &VirtualSinkPreset{
							ModuleType: "module-null-sink",
							SinkName:   "EchoWarp",
							OnStop:     SinkDelete,
							OnStart:    SinkRecreate,
						},
					},
					{ID: 1, Name: "Mic", IsInput: true, MixInputID: &id, MixInputName: "Mic"},
				}},
			},
		},
	}
	require.NoError(t, Save(servers))

	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	preset := loaded[0].Presets["normal"]
	require.Len(t, preset.Devices, 2)

	// Virtual device with VirtualSinkPreset
	vd := preset.Devices[0]
	require.NotNil(t, vd.VirtualSink)
	assert.Equal(t, "module-null-sink", vd.VirtualSink.ModuleType)
	assert.Equal(t, "EchoWarp", vd.VirtualSink.SinkName)
	assert.Equal(t, SinkDelete, vd.VirtualSink.OnStop)
	assert.Equal(t, SinkRecreate, vd.VirtualSink.OnStart)

	// Non-virtual device: VirtualSink should be nil
	assert.Nil(t, preset.Devices[1].VirtualSink)
}

func TestTopLevelVirtualSinkPresetYAMLRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	servers := []Server{{
		Address: "10.0.0.1", Port: 4415, Hostname: "A",
		LastConnected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Presets: map[string]DevicePreset{"normal": {
			Devices:      []PresetDevice{{ID: 1, Name: "Mic", IsInput: true}},
			VirtualSinks: []VirtualSinkPreset{echoWarpVirtualSinkPreset()},
		}},
	}}
	require.NoError(t, Save(servers))

	loaded, err := Load()
	require.NoError(t, err)
	preset := loaded[0].Presets["normal"]

	require.Len(t, preset.Devices, 1)
	require.Len(t, preset.VirtualSinks, 1)
	assert.Equal(t, "EchoWarp", preset.VirtualSinks[0].SinkName)
	assert.Equal(t, SinkRecreate, preset.VirtualSinks[0].OnStart)
}

func echoWarpVirtualSinkPreset() VirtualSinkPreset {
	return VirtualSinkPreset{
		ModuleType: "module-null-sink",
		SinkName:   "EchoWarp",
		OnStop:     SinkDelete,
		OnStart:    SinkRecreate,
	}
}

func TestVirtualSinkPresetYAML_BackwardCompat_NoVirtualSink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Old-format YAML without virtual_sink field
	yamlData := `- address: "10.0.0.1"
  port: 4415
  hostname: "A"
  last_connected: 2026-01-01T00:00:00Z
  presets:
    normal:
      devices:
        - id: 5
          name: "Speaker"
          is_input: false
          virtual: true
`
	p := filepath.Join(dir, "recent_servers.yaml")
	require.NoError(t, os.WriteFile(p, []byte(yamlData), 0600))

	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	d := loaded[0].Presets["normal"].Devices[0]
	assert.True(t, d.Virtual)
	assert.Nil(t, d.VirtualSink, "old presets without virtual_sink should deserialize with nil")
}

func TestLoadJSONFallbackWithNickname(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Legacy JSON with "nickname" field
	legacyJSON := `[{"address":"10.0.0.1","port":4415,"nickname":"OldNick","last_connected":"2026-01-01T00:00:00Z"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "recent_servers.json"), []byte(legacyJSON), 0600))

	loaded, err := Load()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "OldNick", loaded[0].Hostname, "nickname should migrate to hostname via JSON fallback")
}
