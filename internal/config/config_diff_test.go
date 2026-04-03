package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSaveNonDefault_AllDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeServer

	err := SaveNonDefault(cfg, ModeServer, path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &m))

	// Only role should be present.
	assert.Equal(t, "server", m["role"])
	// No other top-level keys besides role.
	assert.Len(t, m, 1, "only role should be in output for all-default config")
}

func TestSaveNonDefault_ServerWithChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Port = 4416
	cfg.Password = "secret"
	cfg.Devices = []DeviceEntry{
		{ID: 1, Role: RoleCapture, Volume: 1.0},
		{ID: 2, Role: RolePlayback, Volume: 0.8},
	}

	err := SaveNonDefault(cfg, ModeServer, path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &m))

	assert.Equal(t, "server", m["role"])

	net, ok := m["network"].(map[string]interface{})
	require.True(t, ok, "network section should exist")
	assert.Equal(t, 4416, net["port"])

	sec, ok := m["security"].(map[string]interface{})
	require.True(t, ok, "security section should exist")
	assert.Equal(t, "secret", sec["password"])

	audio, ok := m["audio"].(map[string]interface{})
	require.True(t, ok, "audio section should exist")
	devices, ok := audio["devices"].([]interface{})
	require.True(t, ok, "devices should be a list")
	assert.Len(t, devices, 2)
}

func TestSaveNonDefault_ClientExcludesProbed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeClient
	cfg.Address = "10.0.0.1"
	// Set server-probed values that should be excluded.
	cfg.SampleRate = 44100
	cfg.Channels = 2
	cfg.OpusBitrate = 128000
	cfg.OpusComplexity = 10
	cfg.OpusApplication = "audio"
	cfg.OpusDTX = false
	cfg.OpusFEC = false
	cfg.MaxClients = 8
	cfg.Conference = true
	cfg.ServerMuted = true
	cfg.Reverse = true
	cfg.Duplex = true

	err := SaveNonDefault(cfg, ModeClient, path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &m))

	assert.Equal(t, "client", m["role"])

	// None of the probed fields should appear.
	assert.Nil(t, m["mode"], "mode (stream mode) should be excluded for client")
	assert.Nil(t, m["server_muted"], "server_muted should be excluded for client")
	assert.Nil(t, m["opus"], "opus section should be excluded for client")

	// Audio should not contain sample_rate or channels.
	if audio, ok := m["audio"].(map[string]interface{}); ok {
		assert.Nil(t, audio["sample_rate"])
		assert.Nil(t, audio["channels"])
	}

	// Connection should not contain max_clients.
	if conn, ok := m["connection"].(map[string]interface{}); ok {
		assert.Nil(t, conn["max_clients"])
	}
}

func TestSaveNonDefault_ClientIncludesAddress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeClient
	cfg.Address = "192.168.1.10"

	err := SaveNonDefault(cfg, ModeClient, path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &m))

	// Client mode: address is a top-level base field
	assert.Equal(t, "192.168.1.10", m["address"])
}

func TestSaveNonDefault_DevicesAlwaysIncluded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Devices = []DeviceEntry{
		{ID: 5, Role: RoleCapture, Volume: 1.0},
	}

	err := SaveNonDefault(cfg, ModeServer, path)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &m))

	audio, ok := m["audio"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, audio["devices"], "devices should always be included when present")
}

func TestSaveNonDefault_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test not applicable on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeServer

	err := SaveNonDefault(cfg, ModeServer, path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file should have 0600 permissions")

	dirInfo, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(), "directory should have 0700 permissions")
}

func TestConfigInfoLine_Server(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected string
	}{
		{
			name: "default port with devices and conference",
			cfg: Config{
				Port:       4415,
				Devices:    []DeviceEntry{{ID: 1}, {ID: 2}, {ID: 3}},
				Conference: true,
			},
			expected: "devices: 3 | conference",
		},
		{
			name: "custom port with duplex and aec",
			cfg: Config{
				Port:    4416,
				Devices: []DeviceEntry{{ID: 1}},
				Duplex:  true,
				AEC:     true,
			},
			expected: "port: 4416 | devices: 1 | duplex | aec",
		},
		{
			name: "no devices, tls",
			cfg: Config{
				Port: 4415,
				TLS:  true,
			},
			expected: "devices: 0 | tls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConfigInfoLine(tt.cfg, ModeServer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigInfoLine_Client(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected string
	}{
		{
			name: "basic client",
			cfg: Config{
				Address:  "192.168.1.10",
				Port:     4415,
				Password: "secret",
			},
			expected: "📡 192.168.1.10:4415 | 🔑",
		},
		{
			name: "client with auto_reconnect",
			cfg: Config{
				Address:       "10.0.0.5",
				Port:          4415,
				AutoReconnect: true,
			},
			expected: "📡 10.0.0.5:4415 | 🔄",
		},
		{
			name: "client config no extras",
			cfg: Config{
				Address: "host.example.com",
				Port:    4416,
			},
			expected: "📡 host.example.com:4416",
		},
		{
			name: "profile basic",
			cfg: Config{
				Mode:    ModeClient,
				Address: "",
			},
			expected: "📋 profile",
		},
		{
			name: "profile with nickname",
			cfg: Config{
				Mode:     ModeClient,
				Address:  "",
				Nickname: "Gamer",
			},
			expected: "📋 profile | nick: Gamer",
		},
		{
			name: "client single mode with devices",
			cfg: Config{
				Address: "10.0.0.1",
				Port:    4415,
				ClientModes: map[string]interface{}{
					"conference": map[string]interface{}{
						"audio": map[string]interface{}{
							"devices": []interface{}{
								map[string]interface{}{"role": "capture"},
								map[string]interface{}{"role": "playback"},
							},
						},
					},
				},
			},
			expected: "📡 10.0.0.1:4415 | conference | 🎤1 | 🔊1",
		},
		{
			name: "client multiple modes",
			cfg: Config{
				Address: "10.0.0.1",
				Port:    4415,
				ClientModes: map[string]interface{}{
					"conference": map[string]interface{}{},
					"duplex":     map[string]interface{}{},
				},
			},
			expected: "📡 10.0.0.1:4415 | modes: conference, duplex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConfigInfoLine(tt.cfg, ModeClient)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestListConfigs_Empty(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())

	entries, err := ListConfigs(ModeServer)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestListConfigs_SortedByModTime(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	dir := ConfigsDir(ModeServer)
	require.NoError(t, os.MkdirAll(dir, 0700))

	// Create configs with different mod times.
	cfgOld := DefaultConfig()
	cfgOld.Mode = ModeServer
	cfgOld.Port = 4416

	cfgNew := DefaultConfig()
	cfgNew.Mode = ModeServer
	cfgNew.Port = 4417

	pathOld := filepath.Join(dir, "old.yaml")
	pathNew := filepath.Join(dir, "new.yaml")

	require.NoError(t, SaveNonDefault(cfgOld, ModeServer, pathOld))
	// Set old file to an older time.
	oldTime := time.Now().Add(-1 * time.Hour)
	require.NoError(t, os.Chtimes(pathOld, oldTime, oldTime))

	require.NoError(t, SaveNonDefault(cfgNew, ModeServer, pathNew))

	entries, err := ListConfigs(ModeServer)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "new", entries[0].Name, "most recent should be first")
	assert.Equal(t, "old", entries[1].Name, "oldest should be last")
}

func TestDeleteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	dir := ConfigsDir(ModeServer)
	require.NoError(t, os.MkdirAll(dir, 0700))

	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	path := filepath.Join(dir, "todelete.yaml")
	require.NoError(t, SaveNonDefault(cfg, ModeServer, path))

	// Verify file exists.
	_, err := os.Stat(path)
	require.NoError(t, err)

	err = DeleteConfig(ModeServer, "todelete")
	require.NoError(t, err)

	// Verify file is gone.
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	err := DeleteConfig(ModeServer, "nonexistent")
	assert.Error(t, err)
}

func TestConfigsDir(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", "/tmp/ew-test")

	assert.Equal(t, "/tmp/ew-test/configs/server", ConfigsDir(ModeServer))
	assert.Equal(t, "/tmp/ew-test/configs/client", ConfigsDir(ModeClient))
}
