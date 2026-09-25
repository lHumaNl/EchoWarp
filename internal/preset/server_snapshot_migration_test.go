package preset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

const mixedSnapshotYAML = `settings:
  last_mode: conference
  password: test-only-legacy
  tls: true
  tls_cert: legacy.crt
  tls_key: legacy.key
  port: 5020
presets:
  conference:
    version: 1
    password: ""
    tls: false
    tls_cert: ""
    tls_key: ""
    server_muted: false
    opus_complexity: 0
    stun_servers: []
    devices: []
    virtual_sinks: []
  normal: {}
`

const mixedSnapshotJSON = `{"settings":{"last_mode":"conference","password":"test-only-legacy","tls":true,"tls_cert":"legacy.crt","tls_key":"legacy.key","port":5020},"presets":{"conference":{"version":1,"password":"","tls":false,"tls_cert":"","tls_key":"","server_muted":false,"opus_complexity":0,"stun_servers":[],"devices":[],"virtual_sinks":[]},"normal":{}}}`

func TestSnapshotMixedLegacyPresenceOverrides(t *testing.T) {
	for extension, fixture := range map[string]string{"yaml": mixedSnapshotYAML, "yml": mixedSnapshotYAML, "json": mixedSnapshotJSON} {
		t.Run(extension, func(t *testing.T) {
			t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
			path := filepath.Join(filepath.Dir(FilePath()), "server_presets."+extension)
			require.NoError(t, os.WriteFile(path, []byte(fixture), 0600))
			sp := Load()
			assertMixedSnapshot(t, sp)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, fixture, string(data), "loading must not migrate on disk")
		})
	}
}

func assertMixedSnapshot(t *testing.T, sp ServerPresets) {
	t.Helper()
	assert.Equal(t, "conference", sp.LastMode)
	mp := sp.Presets["conference"]
	assert.Equal(t, 1, mp.Version)
	assert.Empty(t, mp.Password)
	assert.False(t, mp.TLS)
	assert.Empty(t, mp.TLSCert)
	assert.Empty(t, mp.TLSKey)
	assert.Equal(t, 5020, mp.Port)
	got := Apply(snapshotConfig(), "conference", mp)
	assert.False(t, got.ServerMuted)
	assert.Zero(t, got.OpusComplexity)
	assert.Empty(t, got.STUNServers)
	assert.Equal(t, "test-only-legacy", sp.Presets["normal"].Password)
	assert.True(t, sp.Presets["normal"].TLS)
}

func TestSnapshotLegacyMissingNewFieldsKeepsBase(t *testing.T) {
	for _, fixture := range []string{"presets: {conference: {}}", `{"presets":{"conference":{}}}`} {
		t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
		require.NoError(t, os.WriteFile(FilePath(), []byte(fixture), 0600))
		base := snapshotConfig()
		want := base
		want.Port, want.MaxClients = DefaultsFor("conference").Port, DefaultsFor("conference").MaxClients
		want.Password, want.TLSCert, want.TLSKey, want.TLS = "", "", "", false
		got := Apply(base, "conference", Load().Presets["conference"])
		assert.Equal(t, want, got)
		assert.Equal(t, config.ModeClient, got.Mode)
	}
}

func TestSnapshotExplicitLegacyTLSFalseDoesNotRevivePaths(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, os.WriteFile(FilePath(), []byte("settings:\n  tls: true\n  tls_cert: inherited.crt\n  tls_key: inherited.key\npresets:\n  normal:\n    tls: false\n"), 0600))
	cfg := Apply(config.DefaultConfig(), "normal", Load().Presets["normal"])
	require.False(t, cfg.TLS)
	require.Empty(t, cfg.TLSCert)
	require.Empty(t, cfg.TLSKey)
}

func TestSnapshotLegacyVolumePresence(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	fixture := "presets:\n  normal:\n    devices:\n      - name: omitted\n      - name: zero\n        volume: 0\n      - name: marked\n        volume_set: true\n"
	require.NoError(t, os.WriteFile(FilePath(), []byte(fixture), 0600))
	devices := Load().Presets["normal"].Devices
	require.Equal(t, 1.0, devices[0].Volume)
	require.Zero(t, devices[1].Volume)
	require.True(t, devices[1].VolumeSet)
	require.Zero(t, devices[2].Volume)
	require.True(t, devices[2].VolumeSet)
}
